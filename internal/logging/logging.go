package logging

import (
	"log/slog"
	"net/http"
	"os"
)

type LoggingWriter struct {
	http.ResponseWriter
	Code int
}

func SlogFatal(msg string, args ...interface{}) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func MakeWriterLogging(w http.ResponseWriter) *LoggingWriter {
	return &LoggingWriter{w, http.StatusOK}
}

func (lw *LoggingWriter) WriteHeader(code int) {
	lw.Code = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *LoggingWriter) Write(b []byte) (int, error) {
	if lw.Code == 0 {
		lw.Code = http.StatusOK
	}
	return lw.ResponseWriter.Write(b)
}
