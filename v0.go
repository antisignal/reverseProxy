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

	"github.com/google/uuid"
)

type DebugInfo struct {
	test502BadGateway       bool
	testDeadBackends        bool
	terminateOnChaosExiting bool
	verbose                 bool
}

var debugInfo = DebugInfo{
	test502BadGateway:       false,
	testDeadBackends:        true,
	terminateOnChaosExiting: true,
	verbose:                 true,
}

func getDebugInfo() DebugInfo {
	return debugInfo
}

func webServer(listener net.Listener) {
	slog.Info("[webServer] server starting",
		"event", EVENT_SERVER_STARTED,
		"listener", listener.Addr(),
		"timestamp", time.Now().String(),
		"service", "webServer",
	)

	err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("hello! you're connected on addr " + listener.Addr().String() + " :D"))
		if err != nil {
			slog.Info("[webServer] failed to write response",
				"event", EVENT_SERVER_ERROR,
				"listener", listener.Addr(),
				"timestamp", time.Now().String(),
				"error", err.Error(),
				"service", "webServer",
			)
		}
	}))
	if err != nil {
		slog.Error("[webServer] backend failed to serve",
			"event", EVENT_SERVER_ERROR,
			"listener", listener.Addr(),
			"error", err.Error(),
			"timestamp", time.Now().String(),
			"service", "webServer",
		)
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
	slog.Info("[loadBalancer] selecting a backend",
		"event", EVENT_BACKEND_SELECTING,
		"timestamp", time.Now().String(),
		"service", "getNextBackend",
	)
	var before = time.Now()
	l.mutex.Lock()
	var delta = 1
	for delta <= len(l.backends) {
		var iWrapping = (l.nextBackend + delta) % len(l.backends)
		if !l.backends[iWrapping].alive {
			slog.Debug("[loadBalancer] backend is dead; incrementing delta",
				"backend", iWrapping,
				"timestamp", time.Now().String(),
				"service", "getNextBackend",
				"event", EVENT_BACKEND_SKIPPING,
				"reason", REASON_BACKEND_DEAD)
			delta++
		} else {
			break
		}
	}
	l.mutex.Unlock()
	if delta == len(l.backends) {
		return nil, errors.New("no available backend")
	}
	l.nextBackend = (l.nextBackend + delta) % len(l.backends)
	var since = time.Since(before)
	slog.Info("[loadBalancer] backend selected",
		"backend", l.nextBackend,
		"event", EVENT_BACKEND_SELECTED,
		"timestamp", time.Now().String(),
		"service", "getNextBackend",
		"latency-microseconds", since.Microseconds())
	return &l.backends[l.nextBackend], nil
}

func slogFatal(msg string, args ...interface{}) {
	slog.Error(msg, args...)
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
			"origin-ip-string", *originIPString,
			"timestamp", time.Now().String(),
			"service", "main",
			"error", "",
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
				"error", err.Error(),
				"service", "main",
			)
		}
		listener, err := net.Listen("tcp", originHostPortPair)
		if err != nil {
			slog.Info("[main] failed to listen on address; trying next available port",
				"timestamp", time.Now().String(),
				"event", EVENT_PROXY_ERROR_STARTUP,
				"reason", REASON_LISTEN_FAILED,
				"originHostPortPair", originHostPortPair,
				"error", err.Error(),
				"service", "main",
			)
			currentPort++
			if currentPort > 65535 {
				slogFatal("[main] ports above chosen start port exhausted!",
					"event", EVENT_PROXY_ERROR_STARTUP,
					"reason", REASON_PORTS_EXHAUSTED,
					"originHostPortPair", originHostPortPair,
					"error", "nil",
					"service", "main",
				)
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
				slogFatal("failed to parse dummy url",
					"event", EVENT_SERVER_ERROR_STARTUP,
					"reason", REASON_DUMMY_URL_INVALID,
					"timestamp", time.Now().String(),
					"error", err.Error(),
				)
			}
			loadBalancer.backends[numBackendsAssigned-1].url = dummyURL
		}
		go webServer(listener)

	}
	go reverseProxy(&loadBalancer, listenPort)
	if debugInfo.testDeadBackends {
		go func() { // chaos
			if len(loadBalancer.backends) == 0 {
				panic("violated invariant: no backends provided")
			}
			for {
				time.Sleep(time.Duration(rand.Intn(5)) * time.Second)

				initialIdx := rand.Intn(len(loadBalancer.backends))
				var delta = 0
				var idx = 0
				loadBalancer.mutex.Lock()
				for delta < len(loadBalancer.backends) {
					idx = (initialIdx + delta) % len(loadBalancer.backends)
					if !loadBalancer.backends[idx].alive {
						delta++
					} else {
						break
					}
				}
				loadBalancer.mutex.Unlock()
				if delta == len(loadBalancer.backends) {
					slog.Info("[chaos] no more backends to kill! exiting",
						"event", EVENT_CHAOS_EXITING,
						"reason", REASON_ALL_BACKENDS_DEAD,
						"service", "chaos",
						"timestamp", time.Now().String())
					if debugInfo.terminateOnChaosExiting {
						os.Exit(0)
					}
					return
				}
				slog.Info("\"[chaos] killing backend",
					"backend-idx", idx,
					"backend-url", loadBalancer.backends[idx].url,
					"timestamp", time.Now().String(),
					"service", "chaos",
					"event", EVENT_CHAOS_KILLING_BACKEND)
				var before = time.Now()
				err := (*(loadBalancer.backends[idx].listener)).Close()
				if err != nil {
					log.Println("[chaos] error closing listener (expected):", err)
					slog.Info("[chaos] attempted to close already closed listener",
						"event", EVENT_CHAOS_FAILED_TO_KILL,
						"reason", REASON_LISTENER_ALREADY_CLOSED,
						"backend-idx", idx,
						"backend-url", loadBalancer.backends[idx].url,
						"timestamp", time.Now().String())
				} else {
					slog.Info("[chaos] killed backend",
						"backend-idx", idx,
						"backend-url", loadBalancer.backends[idx].url,
						"event", EVENT_CHAOS_KILLED_BACKEND,
						"service", "chaos",
						"timestamp", time.Now().String(),
						"latency", time.Since(before),
					)
				}
			}
		}()
	}
	healthChecker(&loadBalancer)
}

type contextKey string

const chosenBackendKey contextKey = "chosenBackend"

type StatusInfoBackend struct {
	URL   string `json:"url"`
	Alive bool   `json:"alive"`
}

func reverseProxy(loadBalancer *LoadBalancer, listenPort int) {

	slog.Info("[reverseProxy] starting reverse proxy",
		"event", EVENT_PROXY_STARTING,
		"timestamp", time.Now().String(),
		"service", "reverseProxy")
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
				panic("[reverseProxy] statusInfo invariant violated: missing backends")
			}
			loadBalancer.mutex.Lock()
			statusInfo["backends"] = append(val.([]StatusInfoBackend), StatusInfoBackend{
				URL:   b.url.String(),
				Alive: b.alive,
			})
			loadBalancer.mutex.Unlock()
		}
		statusInfoJSON, err := json.MarshalIndent(statusInfo, "\n", "\t")
		if err != nil {
			slogFatal("[reverseProxy] failed to marshal to json: ",
				"error", err,
				"service", "reverseProxy",
				"timestamp", time.Now().String(),
				"event", EVENT_PROXY_ERROR,
				"reason", REASON_FAILED_TO_MARSHAL_TO_JSON,
				"error", err.Error())
		}
		_, err = w.Write(statusInfoJSON) // XXX: does this send all data in the case when we don't get an error?
		if err != nil {
			slogFatal("[reverseProxy] failed to write response: ",
				"error", err,

				"service", "reverseProxy",
				"timestamp", time.Now().String(),
				"event", EVENT_PROXY_ERROR,
				"reason", REASON_FAILED_TO_WRITE,
				"error", err.Error())
		}
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var requestID = uuid.New()
		slog.Info("[reverseProxy] request received",
			"event", EVENT_REQUEST_RECEIVED,
			"timestamp", time.Now().String(),
			"sending-addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"id", requestID.String(),
		)
		var chosenBackend, err = loadBalancer.getNextBackend()
		if err != nil {
			slog.Info("[reverseProxy] all backends exhausted, returning 503 Service Unavailable",
				"service", "reverseProxy",
				"timestamp", time.Now().String(),
				"event", EVENT_PROXY_RETURNING_503,
				"reason", REASON_ALL_BACKENDS_DEAD,
				"error", err.Error())
			http.Error(w, "503 Service Unavailable", 503)
			return
		}
		*r = *r.WithContext(
			context.WithValue(r.Context(), chosenBackendKey, chosenBackend))
		start := time.Now()
		loggingWriter := makeWriterLogging(w)
		proxy.ServeHTTP(loggingWriter, r)
		slog.Info("[reverseProxy] handled request",
			"timestamp", time.Now().String(),
			"event", EVENT_REQUEST_COMPLETED,
			"timestamp", time.Now().String(),
			"sending-addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"id", requestID.String(),
			"latency", time.Since(start),
			"status", strconv.Itoa((*loggingWriter).code))
	})
	log.Println("[reverseProxy] Listening on :" + strconv.Itoa(listenPort))
	err := http.ListenAndServe(":"+strconv.Itoa(listenPort), nil)
	if err != nil {
		slogFatal("[reverseProxy] stopping reverse proxy due to error",
			"event", EVENT_PROXY_STOPPING,
			"reason", REASON_STOPPING_DUE_TO_ERROR,
			"timestamp", time.Now().String(),
			"service", "reverseProxy",
			"error", err.Error(),
		)
	}
	slog.Info("[reverseProxy] stopping reverse proxy (without error)",
		"event", EVENT_PROXY_STOPPING,
		"reason", REASON_STOPPING_NORMALLY,
		"timestamp", time.Now().String(),
		"service", "reverseProxy",
	)
}

func healthChecker(loadBalancer *LoadBalancer) {
	for {
		var wg sync.WaitGroup
		for i, backend := range loadBalancer.backends {
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
							slog.Error("[healthChecker] unhandled error after GET request",
								"service", "healthChecker",
								"error", err.Error(),
								"timestamp", time.Now().String(),
								"event", EVENT_HEALTHCHECKER_ERROR,
								"reason", REASON_REQUEST_FAILED,
							)
							return
						}
					}
				}
				loadBalancer.mutex.Lock()
				if os.IsTimeout(err) || connectionRefused || resp.StatusCode != http.StatusOK {
					if loadBalancer.backends[i].alive == true {
						slog.Info("[healthChecker] backend is dead",
							"backend-url", loadBalancer.backends[i].url.String(),
							"backend-idx", i,
							"service", "healthChecker",
							"timestamp", time.Now().String(),
							"event", EVENT_BACKEND_HEALTH_CHANGED,
							"health-status", "dead")
						loadBalancer.backends[i].alive = false
					}
				} else {
					if loadBalancer.backends[i].alive == false {
						loadBalancer.backends[i].alive = true
						slog.Info("[healthChecker] backend is dead",
							"backend-url", loadBalancer.backends[i].url.String(),
							"backend-idx", i,
							"service", "healthChecker",
							"timestamp", time.Now().String(),
							"event", EVENT_BACKEND_HEALTH_CHANGED,
							"health-status", "alive")
					}
				}
				loadBalancer.mutex.Unlock()
				if resp != nil {
					err = resp.Body.Close()
					if err != nil {
						slog.Error("[healthChecker] failed to close response body",
							"event", EVENT_HEALTHCHECKER_ERROR,
							"service", "healthChecker",
							"timestamp", time.Now().String(),
							"reason", REASON_FAILED_TO_CLOSE_RESPONSE_BODY)
						return
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
