package httpserver

import (
	"log/slog"
	"net"
	"net/http"
	"time"
)

const helloBody = "hello world-golangs\n"

// Server is the golang-api HTTP server.
type Server struct {
	*http.Server
}

// New returns an HTTP server that listens on 0.0.0.0:port.
// Pass a nil limiter to disable rate limiting (tests).
func New(port string, logger *slog.Logger, lim *IPLimiter) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleHello)
	mux.HandleFunc("POST /{$}", handleHello)

	handler := withRequestLog(
		logger,
		withRateLimit(lim, http.MaxBytesHandler(mux, 32<<10)),
	)

	return &Server{
		Server: &http.Server{
			Addr:              net.JoinHostPort("0.0.0.0", port),
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 12,
		},
	}
}

func handleHello(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(helloBody))
}

func withRequestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		logger.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", sw.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
