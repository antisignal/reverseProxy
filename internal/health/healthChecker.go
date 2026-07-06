package health

import (
	"log/slog"
	"net/http"
	"os"
	"reverseProxy/internal/loadBalancer"
	"reverseProxy/internal/logging"
	"strings"
	"sync"
	"time"
)

func HealthChecker(lb *loadBalancer.LoadBalancer) {
	for {
		var wg sync.WaitGroup
		for i, backend := range lb.Backends {
			wg.Add(1)
			go func() {
				defer wg.Done()
				client := http.Client{
					Timeout: time.Second * 2,
				}
				resp, err := client.Get(backend.Url.String())
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
								"event", logging.EVENT_HEALTHCHECKER_ERROR,
								"reason", logging.REASON_REQUEST_FAILED,
							)
							return
						}
					}
				}
				lb.Mutex.Lock()
				if os.IsTimeout(err) || connectionRefused || resp.StatusCode != http.StatusOK {
					if lb.Backends[i].Alive == true {
						slog.Info("[healthChecker] backend is dead",
							"backend-url", lb.Backends[i].Url.String(),
							"backend-idx", i,
							"service", "healthChecker",
							"timestamp", time.Now().String(),
							"event", logging.EVENT_BACKEND_HEALTH_CHANGED,
							"health-status", "dead")
						lb.Backends[i].Alive = false
					}
				} else {
					if lb.Backends[i].Alive == false {
						lb.Backends[i].Alive = true
						slog.Info("[healthChecker] backend is dead",
							"backend-url", lb.Backends[i].Url.String(),
							"backend-idx", i,
							"service", "healthChecker",
							"timestamp", time.Now().String(),
							"event", logging.EVENT_BACKEND_HEALTH_CHANGED,
							"health-status", "alive")
					}
				}
				lb.Mutex.Unlock()
				if resp != nil {
					err = resp.Body.Close()
					if err != nil {
						slog.Error("[healthChecker] failed to close response body",
							"event", logging.EVENT_HEALTHCHECKER_ERROR,
							"service", "healthChecker",
							"timestamp", time.Now().String(),
							"reason", logging.REASON_FAILED_TO_CLOSE_RESPONSE_BODY)
						return
					}
				}
			}()
		}
		wg.Wait()
		time.Sleep(5 * time.Second)
	}
}
