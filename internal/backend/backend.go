package backend

import (
	"log/slog"
	"net"
	"net/http"
	"reverseProxy/internal/logging"
	"time"
)

func WebServer(listener net.Listener) {
	slog.Info("[webServer] server starting",
		"event", logging.EVENT_SERVER_STARTED,
		"listener", listener.Addr(),
		"timestamp", time.Now().String(),
		"service", "webServer",
	)

	err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("hello! you're connected on addr " + listener.Addr().String() + " :D"))
		if err != nil {
			slog.Info("[webServer] failed to write response",
				"event", logging.EVENT_SERVER_ERROR,
				"listener", listener.Addr(),
				"timestamp", time.Now().String(),
				"error", err.Error(),
				"service", "webServer",
			)
		}
	}))
	if err != nil {
		slog.Error("[webServer] backend failed to serve",
			"event", logging.EVENT_SERVER_ERROR,
			"listener", listener.Addr(),
			"error", err.Error(),
			"timestamp", time.Now().String(),
			"service", "webServer",
		)
	}
}
