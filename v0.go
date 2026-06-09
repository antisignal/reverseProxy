package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
)

func webServer(originURL *url.URL) {
	err := http.ListenAndServe(":"+originURL.Port(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("hello! you're connected on port " + originURL.Port() + " :D"))
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
	var originPort int

	var originIPString = flag.String("origin-ip", "127.0.0.1", "origin ip")
	var listenPortPtr = flag.Int("listen-port", 8080, "http listen port")
	var originPortPtr = flag.Int("origin-port", 9090, "http origin port")
	flag.Parse()
	if listenPortPtr == nil {
		log.Fatal("invalid port")
	}
	listenPort = *listenPortPtr
	if originPortPtr == nil {
		log.Fatal("invalid port")
	}
	originPort = *originPortPtr
	originIP = net.ParseIP(*originIPString)
	if originIP == nil || originIP.To4() == nil {
		log.Fatal("invalid origin ip")
	}

	originURL, err := url.Parse("http://" + originIP.String() + ":" + strconv.Itoa(originPort))
	if err != nil {
		log.Fatalln("failed to parse origin URL:", err)
	}

	go webServer(originURL)
	go reverseProxy(originURL, listenPort)
	<-make(chan int)
}

func reverseProxy(originURL *url.URL, listenPort int) {

	proxy := httputil.NewSingleHostReverseProxy(originURL)

	http.Handle("/", proxy)
	log.Println("Listening on :" + strconv.Itoa(listenPort))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(listenPort), nil))
}
