package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DebugInfo struct {
	test502BadGateway bool
	testDeadBackends  bool
	verbose           bool
}

var debugInfo = DebugInfo{
	test502BadGateway: false,
	testDeadBackends:  true,
	verbose:           true,
}

func getDebugInfo() DebugInfo {
	return debugInfo
}

func webServer(listener net.Listener) {
	if debugInfo.verbose {
		log.Println("[webServer] serving with listener", listener.Addr())
	}

	err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("hello! you're connected on addr " + listener.Addr().String() + " :D"))
		if err != nil {
			log.Fatal(err)
		}
	}))
	if err != nil {
		log.Printf("[webServer] backend stopped: %v", err)
	}
}

type Backend struct {
	alive    bool
	url      *url.URL
	listener *net.Listener
}

type LoadBalancer struct {
	backends    []Backend
	nextBackend int
	mutex       sync.Mutex
}

func (l *LoadBalancer) getNextBackend() (*Backend, error) {
	if debugInfo.verbose {
		log.Println("[loadBalancer] selecting a backend")

	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	var delta = 1
	for delta < len(l.backends) {
		var iWrapping = (l.nextBackend + delta) % len(l.backends)
		if !l.backends[iWrapping].alive {
			if debugInfo.verbose {
				log.Printf("[loadBalancer] backend %d is dead; incrementing delta\n", iWrapping)
			}
			delta++
		} else {
			break
		}
	}
	if delta == len(l.backends) {
		return nil, errors.New("no available backend")
	}
	l.nextBackend = (l.nextBackend + delta) % len(l.backends)
	if debugInfo.verbose {
		log.Println("[loadBalancer] backend selected: " + strconv.Itoa(l.nextBackend))
	}
	return &l.backends[l.nextBackend], nil
}

func slogFatal(msg string, args ...interface{}) {
	slog.Error(msg, args)
	os.Exit(1)
}

func main() {

	var originIP net.IP
	var listenPort int
	var originPortsStart int
	var numBackends int

	var originIPString = flag.String("origin-ip", "127.0.0.1", "origin ip")
	var listenPortPtr = flag.Int("listen-port", 8080, "http listen port")
	var originPortsStartPtr = flag.Int("origin-ports-start", 9090, "http origin port")
	var numBackendsPtr = flag.Int("num-backends", 10, "number of backends")

	flag.Parse()
	listenPort = *listenPortPtr
	originPortsStart = *originPortsStartPtr
	numBackends = *numBackendsPtr
	originIP = net.ParseIP(*originIPString)
	if originIP == nil || originIP.To4() == nil {
		slogFatal("[main] invalid origin ip",
			"event", EVENT_PROXY_ERROR_STARTUP,
			"origin-ip-string", originIPString,
			"timestamp", time.Now().String(),
		)
	}

	var loadBalancer = LoadBalancer{
		backends:    []Backend{},
		nextBackend: 0,
		mutex:       sync.Mutex{},
	}
	var currentPort = originPortsStart
	var numBackendsAssigned = 0
	for numBackendsAssigned < numBackends {
		var originHostPortPair = originIP.String() + ":" + strconv.Itoa(currentPort)
		originURL, err := url.Parse("http://" + originHostPortPair)
		if err != nil {
			slogFatal("[main] failed to parse origin url",
				"event", EVENT_PROXY_ERROR_STARTUP,
				"originHostPortPair", originHostPortPair,
				"timestamp", time.Now().String(),
				"err", err.Error(),
			)
		}
		listener, err := net.Listen("tcp", originHostPortPair)
		if err != nil {
			slogFatal("[main] failed to listen on address",
				"timestamp", time.Now().String(),
				"event", EVENT_PROXY_ERROR_STARTUP,
				"originHostPortPair", originHostPortPair,
				"err", err.Error(),
			)
			currentPort++
			if currentPort > 65535 {
				log.Fatal("[main] ports above chosen start port exhausted!")
			}
			continue
		}
		loadBalancer.backends = append(loadBalancer.backends, Backend{
			alive:    true,
			url:      originURL,
			listener: &listener,
		})
		numBackendsAssigned++
		currentPort++
		if getDebugInfo().test502BadGateway {
			var dummyURL *url.URL
			dummyURL, err = url.Parse("http://127.0.0.1:8081")
			if err != nil {
				log.Fatal(err)
			}
			loadBalancer.backends[numBackendsAssigned-1].url = dummyURL
		}
		go webServer(listener)

	}
	logLock := sync.Mutex{}
	go reverseProxy(&loadBalancer, listenPort, &logLock)
	if debugInfo.testDeadBackends {
		go func() {
			for {
				time.Sleep(time.Duration(rand.Intn(5)) * time.Second)

				initialIdx := rand.Intn(len(loadBalancer.backends))
				var delta = 0
				var idx = 0
				for delta < len(loadBalancer.backends) {
					idx = (initialIdx + delta) % len(loadBalancer.backends)
					if !loadBalancer.backends[idx].alive {
						delta++
					} else {
						break
					}
				}
				if delta == len(loadBalancer.backends) {
					log.Printf("[chaos] no more backends to kill! exiting")
					return
				}

				log.Printf("[chaos] killing backend %s", loadBalancer.backends[idx].url)
				err := (*(loadBalancer.backends[idx].listener)).Close()
				if err != nil {
					log.Println("[chaos] error closing listener (expected):", err)
				} else {
					log.Printf("[chaos] killed %s", loadBalancer.backends[idx].url)
				}
			}
		}()
	}
	healthChecker(loadBalancer.backends, &logLock)
}

type contextKey string

const chosenBackendKey contextKey = "chosenBackend"

type StatusInfoBackend struct {
	URL   string `json:"url"`
	Alive bool   `json:"alive"`
}

func reverseProxy(loadBalancer *LoadBalancer, listenPort int, logLock *sync.Mutex) {
	log.Print("[reverseProxy] starting reverse proxy\n")
	proxy := httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			var chosen = pr.In.Context().Value(chosenBackendKey).(*Backend)
			pr.SetURL(chosen.url)
		},
	}
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		var statusInfo = make(map[string]any)
		statusInfo["backends"] = []StatusInfoBackend{}
		for _, b := range loadBalancer.backends {
			val, ok := statusInfo["backends"]
			if !ok {
				log.Fatal("[reverseProxy] unreachable")
			}
			statusInfo["backends"] = append(val.([]StatusInfoBackend), StatusInfoBackend{
				URL:   b.url.String(),
				Alive: b.alive,
			})
		}
		statusInfoJSON, err := json.MarshalIndent(statusInfo, "\n", "\t")
		if err != nil {
			log.Fatal("[reverseProxy] failed to marshal to json: ", err)
		}
		_, err = w.Write(statusInfoJSON) // XXX: does this send all data in the case when we don't get an error?
		if err != nil {
			log.Fatal("[reverseProxy] failed to write response: ", err)
		}
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var chosenBackend, err = loadBalancer.getNextBackend()
		if err != nil {
			log.Println("[reverseProxy] all backends exhausted, returning 503 Service Unavailable")
			http.Error(w, "503 Service Unavailable", 503)
			return
		}
		*r = *r.WithContext(
			context.WithValue(r.Context(), chosenBackendKey, chosenBackend))
		start := time.Now()
		loggingWriter := makeWriterLogging(w)
		proxy.ServeHTTP(loggingWriter, r)
		latency := time.Since(start)
		var logStrings = []string{}
		logStrings = append(logStrings, "[reverseProxy] handled request")
		logStrings = append(logStrings, "==========")
		logStrings = append(logStrings, "timestamp: "+time.Now().Format("2006-01-02 15:04:05"))
		logStrings = append(logStrings, "sending addr: "+r.RemoteAddr)
		logStrings = append(logStrings, "destination host: "+chosenBackend.url.String())
		logStrings = append(logStrings, "method: "+r.Method)
		logStrings = append(logStrings, "path: "+r.URL.Path)
		logStrings = append(logStrings, "latency: "+strconv.Itoa(int(latency)))
		logStrings = append(logStrings, "status: "+strconv.Itoa((*loggingWriter).code))
		logStrings = append(logStrings, "==========")
		logLock.Lock()
		for _, s := range logStrings {
			log.Println(s)
		}
		logLock.Unlock()
	})
	log.Println("[reverseProxy] Listening on :" + strconv.Itoa(listenPort))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(listenPort), nil))
}

func healthChecker(backends []Backend, logLock *sync.Mutex) {
	for {
		var wg sync.WaitGroup
		for i, backend := range backends {
			wg.Add(1)
			go func() {
				defer wg.Done()
				client := http.Client{
					Timeout: time.Second * 2,
				}
				resp, err := client.Get(backend.url.String())
				var connectionRefused = false
				if err != nil {
					if !os.IsTimeout(err) {
						if strings.Contains(err.Error(), "connect: connection refused") {
							connectionRefused = true
						} else {
							log.Fatal(err)
						}
					}
				}
				logLock.Lock()
				if os.IsTimeout(err) || connectionRefused || resp.StatusCode != http.StatusOK {
					if backends[i].alive == true {
						log.Println("[healthChecker] backend " + backends[i].url.String() + " is dead")
						backends[i].alive = false
					}
				} else {
					if backends[i].alive == false {
						backends[i].alive = true
						log.Println("[healthChecker] backend " + backends[i].url.String() + " is alive")
					}
				}
				logLock.Unlock()
				if resp != nil {
					err = resp.Body.Close()
					if err != nil {
						log.Fatal(err)
					}
				}
			}()
		}
		wg.Wait()
		time.Sleep(5 * time.Second)
	}
}

type LoggingWriter struct {
	http.ResponseWriter
	code int
}

func makeWriterLogging(w http.ResponseWriter) *LoggingWriter {
	return &LoggingWriter{w, http.StatusOK}
}

func (lw *LoggingWriter) WriteHeader(code int) {
	lw.code = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *LoggingWriter) Write(b []byte) (int, error) {
	if lw.code == 0 {
		lw.code = http.StatusOK
	}
	return lw.ResponseWriter.Write(b)
}
