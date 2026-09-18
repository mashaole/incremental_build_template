package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPLimiterAllowsBurstThenRejects(t *testing.T) {
	t.Parallel()
	lim := NewIPLimiter(LimitConfig{RatePerSec: 0.001, Burst: 2, MaxKeys: 8})
	t.Cleanup(lim.Stop)

	if !lim.Allow("1.1.1.1") || !lim.Allow("1.1.1.1") {
		t.Fatal("burst should allow two requests")
	}
	if lim.Allow("1.1.1.1") {
		t.Fatal("third request should be rejected")
	}
	if !lim.Allow("8.8.8.8") {
		t.Fatal("a different IP should have its own bucket")
	}
}

func TestIPLimiterFailClosedAtMaxKeys(t *testing.T) {
	t.Parallel()
	lim := NewIPLimiter(LimitConfig{RatePerSec: 10, Burst: 1, MaxKeys: 1})
	t.Cleanup(lim.Stop)

	if !lim.Allow("1.1.1.1") {
		t.Fatal("first key should fit")
	}
	if lim.Allow("2.2.2.2") {
		t.Fatal("new keys must be rejected when the map is full")
	}
}

func TestRateLimitHTTPStatus(t *testing.T) {
	t.Parallel()
	lim := NewIPLimiter(LimitConfig{RatePerSec: 0.001, Burst: 1, MaxKeys: 8})
	t.Cleanup(lim.Stop)

	logger := discardLogger()
	srv := New("8080", logger, lim, "http://127.0.0.1:8081")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:4000"

	first := httptest.NewRecorder()
	srv.Handler.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	srv.Handler.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestClientIPUsesFirstForwardedHop(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 192.0.2.1")
	req.RemoteAddr = "192.0.2.1:443"
	if got := clientIP(req); got != "203.0.113.10" {
		t.Fatalf("clientIP = %q, want 203.0.113.10", got)
	}
}
