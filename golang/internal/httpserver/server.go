package httpserver

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const helloBody = "hello world-golangs\n"

// Server is the golang-api HTTP server.
type Server struct {
	*http.Server
	nodejsURL string
	client    *http.Client
}

// New listens on 0.0.0.0:port. nodejsURL is the other service (local default
// is set by main from NODEJS_URL). A nil limiter disables rate limiting.
func New(port string, logger *slog.Logger, lim *IPLimiter, nodejsURL string) *Server {
	mux := http.NewServeMux()
	srv := &Server{
		nodejsURL: strings.TrimRight(nodejsURL, "/"),
		client:    &http.Client{Timeout: 3 * time.Second},
	}
	mux.HandleFunc("GET /{$}", handleHello)
	mux.HandleFunc("POST /{$}", handleHello)
	mux.HandleFunc("GET /call-nodejs", srv.handleCallNodejs)

	handler := withRequestLog(
		logger,
		withRateLimit(lim, http.MaxBytesHandler(mux, 32<<10)),
	)

	srv.Server = &http.Server{
		Addr:              net.JoinHostPort("0.0.0.0", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 12,
	}
	return srv
}

func handleHello(w http.ResponseWriter, _ *http.Request) {
	writePlain(w, http.StatusOK, helloBody)
}

func (s *Server) handleCallNodejs(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.nodejsURL+"/", nil)
	if err != nil {
		http.Error(w, "nodejs unavailable", http.StatusInternalServerError)
		return
	}
	res, err := s.client.Do(req)
	if err != nil {
		http.Error(w, "nodejs unavailable", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4096))
	if err != nil || res.StatusCode != http.StatusOK {
		http.Error(w, "nodejs unavailable", http.StatusBadGateway)
		return
	}
	writePlain(w, http.StatusOK, string(body))
}

// HTTPBaseURL returns raw if it is http(s), otherwise fallback.
func HTTPBaseURL(raw, fallback string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("url must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("url host required")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func writePlain(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
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
