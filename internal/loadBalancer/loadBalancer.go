package loadBalancer

import (
	"errors"
	"log/slog"
	"net"
	"net/url"
	"reverseProxy/internal/logging"
	"sync"
	"time"
)

type Backend struct {
	Alive    bool
	Url      *url.URL
	Listener *net.Listener
}
type LoadBalancer struct {
	Backends    []Backend
	NextBackend int
	Mutex       sync.Mutex
}

func (l *LoadBalancer) GetNextBackend() (*Backend, error) {
	slog.Info("[loadBalancer] selecting a backend",
		"event", logging.EVENT_BACKEND_SELECTING,
		"timestamp", time.Now().String(),
		"service", "getNextBackend",
	)
	var before = time.Now()
	l.Mutex.Lock()
	var delta = 1
	for delta <= len(l.Backends) {
		var iWrapping = (l.NextBackend + delta) % len(l.Backends)
		if !l.Backends[iWrapping].Alive {
			slog.Debug("[loadBalancer] backend is dead; incrementing delta",
				"backend", iWrapping,
				"timestamp", time.Now().String(),
				"service", "getNextBackend",
				"event", logging.EVENT_BACKEND_SKIPPING,
				"reason", logging.REASON_BACKEND_DEAD)
			delta++
		} else {
			break
		}
	}
	if delta == len(l.Backends) {
		l.Mutex.Unlock()
		return nil, errors.New("no available backend")
	}
	l.NextBackend = (l.NextBackend + delta) % len(l.Backends)
	var since = time.Since(before)
	slog.Info("[loadBalancer] backend selected",
		"backend", l.NextBackend,
		"event", logging.EVENT_BACKEND_SELECTED,
		"timestamp", time.Now().String(),
		"service", "getNextBackend",
		"latency-microseconds", since.Microseconds())
	l.Mutex.Unlock()
	return &l.Backends[l.NextBackend], nil
}
