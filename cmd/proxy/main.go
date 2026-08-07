package main

import (
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

	var conf = config.GetConfig()
	if conf == nil {
		logging.SlogFatal("[main] config creation failed; terminating",
			"event", logging.EVENT_PROGRAM_EXITING,
			"reason", logging.REASON_INVALID_CONFIG,
			"timestamp", time.Now().String(),
			"service", "main")
	}

	if conf.OriginIP == nil || conf.OriginIP.To4() == nil {
		logging.SlogFatal("[main] invalid origin ip",
			"event", logging.EVENT_PROXY_ERROR_STARTUP,
			"origin-ip-string", conf.OriginIP.String(),
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
	var currentPort = conf.OriginPortsStart
	var numBackendsAssigned = 0
	for numBackendsAssigned < conf.NumBackends {
		var backendHostPortPair = conf.OriginIP.String() + ":" + strconv.Itoa(currentPort)
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
		if conf.Test502BadGateway {
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
	go proxy.ReverseProxy(&lb, conf.ListenPort)
	if conf.TestDeadBackends {
		go chaos.Chaos(&lb, conf)
	}
	health.HealthChecker(&lb)
}
