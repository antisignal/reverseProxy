package chaos

import (
	"log"
	"log/slog"
	"math/rand"
	"os"
	"reverseProxy/internal/config"
	"reverseProxy/internal/loadBalancer"
	"reverseProxy/internal/logging"
	"time"
)

func Chaos(lb *loadBalancer.LoadBalancer) { // chaos
	if len(lb.Backends) == 0 {
		panic("violated invariant: no backends provided")
	}
	for {
		time.Sleep(time.Duration(rand.Intn(5)) * time.Second)

		initialIdx := rand.Intn(len(lb.Backends))
		var delta = 0
		var idx = 0
		lb.Mutex.Lock()
		for delta < len(lb.Backends) {
			idx = (initialIdx + delta) % len(lb.Backends)
			if !lb.Backends[idx].Alive {
				delta++
			} else {
				break
			}
		}
		lb.Mutex.Unlock()
		if delta == len(lb.Backends) {
			slog.Info("[chaos] no more backends to kill! exiting",
				"event", logging.EVENT_CHAOS_EXITING,
				"reason", logging.REASON_ALL_BACKENDS_DEAD,
				"service", "chaos",
				"timestamp", time.Now().String())
			if config.GetDebugInfo().TerminateOnChaosExiting {
				slog.Info("[chaos] stopping entire program (terminateOnChaosExiting)",
					"event", logging.EVENT_PROGRAM_EXITING,
					"service", "chaos",
					"reason", logging.REASON_TERMINATE_ON_CHAOS_EXITING,
					"timestamp", time.Now().String())
				os.Exit(0)
			}
			return
		}
		slog.Info("\"[chaos] killing backend",
			"backend-idx", idx,
			"backend-url", lb.Backends[idx].Url,
			"timestamp", time.Now().String(),
			"service", "chaos",
			"event", logging.EVENT_CHAOS_KILLING_BACKEND)
		var before = time.Now()
		err := (*(lb.Backends[idx].Listener)).Close()
		if err != nil {
			log.Println("[chaos] error closing listener (expected):", err)
			slog.Info("[chaos] attempted to close already closed listener",
				"event", logging.EVENT_CHAOS_FAILED_TO_KILL,
				"reason", logging.REASON_LISTENER_ALREADY_CLOSED,
				"backend-idx", idx,
				"backend-url", lb.Backends[idx].Url,
				"timestamp", time.Now().String())
		} else {
			slog.Info("[chaos] killed backend",
				"backend-idx", idx,
				"backend-url", lb.Backends[idx].Url,
				"event", logging.EVENT_CHAOS_KILLED_BACKEND,
				"service", "chaos",
				"timestamp", time.Now().String(),
				"latency", time.Since(before),
			)
		}
	}
}
