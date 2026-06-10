package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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

func main() {

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
	var originURLs = []*url.URL{}
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
		originURLs = append(originURLs, originURL)
		numBackendsAssigned++
		currentPort++
		go webServer(listener)

	}
	var logLock = sync.Mutex{}
	go reverseProxy(originURLs, listenPort, &logLock)
	<-make(chan int)
}

func reverseProxy(originURLs []*url.URL, listenPort int, logLock *sync.Mutex) {
	var roundRobinChoice uint64 = 0
	log.Print("starting reverse proxy\n")
	proxy := httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			idx := atomic.AddUint64(&roundRobinChoice, 1)
			fmt.Println("roundRobinChoice:", roundRobinChoice)
			idx = idx % uint64(len(originURLs))
			fmt.Println("idx:", idx)
			pr.SetURL(originURLs[idx])
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		loggingWriter := makeWriterLogging(w)
		proxy.ServeHTTP(*loggingWriter, r)
		latency := time.Since(start)
		var logStrings = []string{}
		logStrings = append(logStrings, "timestamp: "+time.Now().Format("2006-01-02 15:04:05"))
		logStrings = append(logStrings, "sending addr: "+r.RemoteAddr)
		logStrings = append(logStrings, "destination host: "+originURLs[roundRobinChoice%uint64(len(originURLs))].String())
		logStrings = append(logStrings, "path: "+r.URL.Path)
		logStrings = append(logStrings, "latency: "+strconv.Itoa(int(latency)))
		logStrings = append(logStrings, "status: "+strconv.Itoa((*loggingWriter).code))
		for _, s := range logStrings {
			log.Println(s)
		}
	})
	log.Println("Listening on :" + strconv.Itoa(listenPort))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(listenPort), nil))
}

type LoggingWriter struct {
	http.ResponseWriter
	code int
}

func makeWriterLogging(w http.ResponseWriter) *LoggingWriter {
	return &LoggingWriter{w, http.StatusOK}
}

func (lw LoggingWriter) WriteHeader(code int) {
	lw.code = code
	lw.ResponseWriter.WriteHeader(code)
}
