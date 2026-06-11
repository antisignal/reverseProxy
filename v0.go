package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func webServer(listener net.Listener) {
	err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("hello! you're connected on addr " + listener.Addr().String() + " :D"))
		if err != nil {
			log.Fatal(err)
		}
	}))
	if err != nil {
		log.Fatal(err)
	}
}

type Backend struct {
	alive bool
	url   *url.URL
}

func main() {

	var debug = struct {
		test502BadGateway bool
		testDeadBackends  bool
	}{
		test502BadGateway: false,
		testDeadBackends:  true,
	}

	var originIP net.IP
	var listenPort int
	var originPortsStart int
	var numBackends int

	var originIPString = flag.String("origin-ip", "127.0.0.1", "origin ip")
	var listenPortPtr = flag.Int("listen-port", 8080, "http listen port")
	var originPortsStartPtr = flag.Int("origin-ports-start", 9090, "http origin port")
	var numBackendsPtr = flag.Int("num-backends", 3, "number of backends")

	flag.Parse()
	listenPort = *listenPortPtr
	originPortsStart = *originPortsStartPtr
	numBackends = *numBackendsPtr
	originIP = net.ParseIP(*originIPString)
	if originIP == nil || originIP.To4() == nil {
		log.Fatal("invalid origin ip")
	}
	var originBackends = []Backend{}
	var currentPort = originPortsStart
	var numBackendsAssigned = 0
	for numBackendsAssigned < numBackends {
		var originHostPortPair = originIP.String() + ":" + strconv.Itoa(currentPort)
		originURL, err := url.Parse("http://" + originHostPortPair)
		if err != nil {
			log.Fatal(err)
		}
		listener, err := net.Listen("tcp", originHostPortPair)
		if err != nil {
			log.Println(err)
			currentPort++
			if currentPort > 65535 {
				log.Fatal("ports above chosen start port exhausted!")
			}
			continue
		}
		originBackends = append(originBackends, Backend{
			alive: true,
			url:   originURL,
		})
		numBackendsAssigned++
		currentPort++
		if debug.test502BadGateway {
			var dummyURL *url.URL
			dummyURL, err = url.Parse("http://127.0.0.1:8081")
			if err != nil {
				log.Fatal(err)
			}
			originBackends[numBackendsAssigned-1].url = dummyURL
		}
		go webServer(listener)

	}
	logLock := sync.Mutex{}
	go reverseProxy(originBackends, listenPort, &logLock)
	healthChecker(originBackends, &logLock)
}

type contextKey string

const chosenBackendKey contextKey = "chosenBackend"

func reverseProxy(originBackends []Backend, listenPort int, logLock *sync.Mutex) {
	var roundRobinChoice uint64 = 0
	log.Print("starting reverse proxy\n")
	proxy := httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			idx := atomic.AddUint64(&roundRobinChoice, 1)
			fmt.Println("roundRobinChoice:", roundRobinChoice)
			idx = idx % uint64(len(originBackends))
			fmt.Println("idx:", idx)
			var chosen = originBackends[idx]
			pr.SetURL(chosen.url)
			*pr.In = *pr.In.WithContext(
				context.WithValue(pr.In.Context(), chosenBackendKey, chosen))
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		loggingWriter := makeWriterLogging(w)
		proxy.ServeHTTP(loggingWriter, r)
		var chosenBackend = r.Context().Value(chosenBackendKey).(Backend)
		latency := time.Since(start)
		var logStrings = []string{}
		logStrings = append(logStrings, "timestamp: "+time.Now().Format("2006-01-02 15:04:05"))
		logStrings = append(logStrings, "sending addr: "+r.RemoteAddr)
		logStrings = append(logStrings, "destination host: "+chosenBackend.url.String())
		logStrings = append(logStrings, "method: "+r.Method)
		logStrings = append(logStrings, "path: "+r.URL.Path)
		logStrings = append(logStrings, "latency: "+strconv.Itoa(int(latency)))
		logStrings = append(logStrings, "status: "+strconv.Itoa((*loggingWriter).code))
		logLock.Lock()
		for _, s := range logStrings {
			log.Println(s)
		}
		logLock.Unlock()
	})
	log.Println("Listening on :" + strconv.Itoa(listenPort))
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
				if err != nil {
					log.Fatal(err)
				}
				logLock.Lock()
				if os.IsTimeout(err) || resp.StatusCode != http.StatusOK {
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
				err = resp.Body.Close()
				if err != nil {
					log.Fatal(err)
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
