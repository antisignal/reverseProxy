package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync/atomic"
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
	go reverseProxy(originURLs, listenPort)
	<-make(chan int)
}

func reverseProxy(originURLs []*url.URL, listenPort int) {
	var roundRobinChoice uint64 = 0
	proxy := httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			roundRobinChoice++
			idx := atomic.AddUint64(&roundRobinChoice, 1) - 1
			pr.SetURL(originURLs[idx%uint64(len(originURLs))])
		},
	}

	http.Handle("/", &proxy)
	log.Println("Listening on :" + strconv.Itoa(listenPort))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(listenPort), nil))
}
