package main

import (
	"flag"
	"log/slog"
	"net"
	"net/url"
	"reverseProxy/internal/backend"
	"reverseProxy/internal/chaos"
	"reverseProxy/internal/config"
	"reverseProxy/internal/health"
	"reverseProxy/internal/loadBalancer"
	"reverseProxy/internal/logging"
	"reverseProxy/internal/proxy"
	"strconv"
	"sync"
	"time"
)

func main() {

	var originIP net.IP
	var listenPort int
	var backendPortsStart int
	var numBackends int

	var originIPString = flag.String("origin-ip", "127.0.0.1", "origin ip")
	var listenPortPtr = flag.Int("listen-port", 8080, "http listen port")
	var backendPortsStartPtr = flag.Int("backend-ports-start", 9090, "http backend origin port")
	var numBackendsPtr = flag.Int("num-backends", 10, "number of backends")

	flag.Parse()
	listenPort = *listenPortPtr
	backendPortsStart = *backendPortsStartPtr
	numBackends = *numBackendsPtr
	originIP = net.ParseIP(*originIPString)
	if originIP == nil || originIP.To4() == nil {
		logging.SlogFatal("[main] invalid origin ip",
			"event", logging.EVENT_PROXY_ERROR_STARTUP,
			"origin-ip-string", *originIPString,
			"timestamp", time.Now().String(),
			"service", "main",
			"error", "",
		)
	}

	var lb = loadBalancer.LoadBalancer{
		Backends:    []loadBalancer.Backend{},
		NextBackend: 0,
		Mutex:       sync.Mutex{},
	}
	var currentPort = backendPortsStart
	var numBackendsAssigned = 0
	for numBackendsAssigned < numBackends {
		var backendHostPortPair = originIP.String() + ":" + strconv.Itoa(currentPort)
		backendURL, err := url.Parse("http://" + backendHostPortPair)
		if err != nil {
			logging.SlogFatal("[main] failed to parse backend url",
				"event", logging.EVENT_PROXY_ERROR_STARTUP,
				"backendHostPortPair", backendHostPortPair,
				"timestamp", time.Now().String(),
				"error", err.Error(),
				"service", "main",
			)
		}
		listener, err := net.Listen("tcp", backendHostPortPair)
		if err != nil {
			slog.Info("[main] failed to listen on address; trying next available port",
				"timestamp", time.Now().String(),
				"event", logging.EVENT_PROXY_ERROR_STARTUP,
				"reason", logging.REASON_LISTEN_FAILED,
				"originHostPortPair", backendHostPortPair,
				"error", err.Error(),
				"service", "main",
			)
			currentPort++
			if currentPort > 65535 {
				logging.SlogFatal("[main] ports above chosen start port exhausted!",
					"event", logging.EVENT_PROXY_ERROR_STARTUP,
					"reason", logging.REASON_PORTS_EXHAUSTED,
					"originHostPortPair", backendHostPortPair,
					"error", "nil",
					"service", "main",
				)
			}
			continue
		}
		lb.Backends = append(lb.Backends, loadBalancer.Backend{
			Alive:    true,
			Url:      backendURL,
			Listener: &listener,
		})
		numBackendsAssigned++
		currentPort++
		if config.GetDebugInfo().Test502BadGateway {
			var dummyURL *url.URL
			dummyURL, err = url.Parse("http://127.0.0.1:8081")
			if err != nil {
				logging.SlogFatal("failed to parse dummy url",
					"event", logging.EVENT_SERVER_ERROR_STARTUP,
					"reason", logging.REASON_DUMMY_URL_INVALID,
					"timestamp", time.Now().String(),
					"error", err.Error(),
				)
			}
			lb.Backends[numBackendsAssigned-1].Url = dummyURL
		}
		go backend.WebServer(listener)

	}
	go proxy.ReverseProxy(&lb, listenPort)
	if config.GetDebugInfo().TestDeadBackends {
		go chaos.Chaos(&lb)
	}
	health.HealthChecker(&lb)
}
