package proxy

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"reverseProxy/internal/loadBalancer"
	"strconv"
	"time"

	"reverseProxy/internal/logging"

	"github.com/google/uuid"
)

type contextKey string

const chosenBackendKey contextKey = "chosenBackend"

type StatusInfoBackend struct {
	URL   string `json:"url"`
	Alive bool   `json:"alive"`
}

func ReverseProxy(lb *loadBalancer.LoadBalancer, listenPort int) {

	slog.Info("[reverseProxy] starting reverse proxy",
		"event", logging.EVENT_PROXY_STARTING,
		"timestamp", time.Now().String(),
		"service", "reverseProxy")
	proxy := httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			var chosen = pr.In.Context().Value(chosenBackendKey).(*loadBalancer.Backend)
			pr.SetURL(chosen.Url)
		},
	}
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		var statusInfo = make(map[string]any)
		statusInfo["backends"] = []StatusInfoBackend{}
		for _, b := range lb.Backends {
			val, ok := statusInfo["backends"]
			if !ok {
				panic("[reverseProxy] statusInfo invariant violated: missing backends")
			}
			lb.Mutex.Lock()
			statusInfo["backends"] = append(val.([]StatusInfoBackend), StatusInfoBackend{
				URL:   b.Url.String(),
				Alive: b.Alive,
			})
			lb.Mutex.Unlock()
		}
		statusInfoJSON, err := json.MarshalIndent(statusInfo, "\n", "\t")
		if err != nil {
			logging.SlogFatal("[reverseProxy] failed to marshal to json: ",
				"error", err,
				"service", "reverseProxy",
				"timestamp", time.Now().String(),
				"event", logging.EVENT_PROXY_ERROR,
				"reason", logging.REASON_FAILED_TO_MARSHAL_TO_JSON,
				"error", err.Error())
		}
		_, err = w.Write(statusInfoJSON) // XXX: does this send all data in the case when we don't get an error?
		if err != nil {
			logging.SlogFatal("[reverseProxy] failed to write response: ",
				"error", err,

				"service", "reverseProxy",
				"timestamp", time.Now().String(),
				"event", logging.EVENT_PROXY_ERROR,
				"reason", logging.REASON_FAILED_TO_WRITE,
				"error", err.Error())
		}
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var requestID = uuid.New()
		slog.Info("[reverseProxy] request received",
			"event", logging.EVENT_REQUEST_RECEIVED,
			"timestamp", time.Now().String(),
			"sending-addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"id", requestID.String(),
		)
		var chosenBackend, err = lb.GetNextBackend()
		if err != nil {
			slog.Info("[reverseProxy] all backends exhausted, returning 503 Service Unavailable",
				"service", "reverseProxy",
				"timestamp", time.Now().String(),
				"event", logging.EVENT_PROXY_RETURNING_503,
				"reason", logging.REASON_ALL_BACKENDS_DEAD,
				"error", err.Error())
			http.Error(w, "503 Service Unavailable", 503)
			return
		}
		*r = *r.WithContext(
			context.WithValue(r.Context(), chosenBackendKey, chosenBackend))
		start := time.Now()
		loggingWriter := logging.MakeWriterLogging(w)
		proxy.ServeHTTP(loggingWriter, r)
		slog.Info("[reverseProxy] handled request",
			"timestamp", time.Now().String(),
			"event", logging.EVENT_REQUEST_COMPLETED,
			"timestamp", time.Now().String(),
			"sending-addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"id", requestID.String(),
			"latency", time.Since(start),
			"status", strconv.Itoa((*loggingWriter).Code))
	})
	log.Println("[reverseProxy] Listening on :" + strconv.Itoa(listenPort))
	err := http.ListenAndServe(":"+strconv.Itoa(listenPort), nil)
	if err != nil {
		logging.SlogFatal("[reverseProxy] stopping reverse proxy due to error",
			"event", logging.EVENT_PROXY_STOPPING,
			"reason", logging.REASON_STOPPING_DUE_TO_ERROR,
			"timestamp", time.Now().String(),
			"service", "reverseProxy",
			"error", err.Error(),
		)
	}
	slog.Info("[reverseProxy] stopping reverse proxy (without error)",
		"event", logging.EVENT_PROXY_STOPPING,
		"reason", logging.REASON_STOPPING_NORMALLY,
		"timestamp", time.Now().String(),
		"service", "reverseProxy",
	)
}
