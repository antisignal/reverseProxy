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

func main() {

	var originIP net.IP
	var port int

	var originIPString = flag.String("origin-ip", "127.0.0.1", "origin ip")
	var portPtr = flag.Int("port", 8080, "http port")
	if portPtr == nil {
		log.Fatal("invalid port")
	}
	port = *portPtr
	originIP = net.ParseIP(*originIPString)
	if originIP == nil || originIP.To4() == nil {
		log.Fatal("invalid origin ip")
	}

	originStr, err := url.Parse("http://" + originIP.String() + ":" + strconv.Itoa(port))
	if err != nil {
		log.Fatalln("failed to parse origin URL:", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(originStr)

	http.Handle("/", proxy)
	log.Println("Listening on :" + strconv.Itoa(port))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
