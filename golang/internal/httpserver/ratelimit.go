package httpserver

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultRatePerSec = 5.0
	defaultBurst      = 20
	defaultMaxKeys    = 10_000
	idleTTL           = 2 * time.Minute
	cleanupEvery      = 30 * time.Second
)

// IPLimiter is a per-key token bucket.
// Allow is O(1) average; space is O(min(distinct keys, MaxKeys)).
type IPLimiter struct {
	rate     float64
	burst    float64
	maxKeys  int
	mu       sync.Mutex
	buckets  map[string]*bucket
	stop     chan struct{}
	stopOnce sync.Once
}

type bucket struct {
	tokens float64
	last   time.Time
}

// LimitConfig controls refill rate, burst, and map cap.
type LimitConfig struct {
	RatePerSec float64
	Burst      int
	MaxKeys    int
}

// NewIPLimiter starts a background idle-eviction loop. Call Stop in tests.
func NewIPLimiter(cfg LimitConfig) *IPLimiter {
	rate := cfg.RatePerSec
	if rate <= 0 {
		rate = defaultRatePerSec
	}
	burst := float64(cfg.Burst)
	if burst < 1 {
		burst = defaultBurst
	}
	maxKeys := cfg.MaxKeys
	if maxKeys < 1 {
		maxKeys = defaultMaxKeys
	}

	l := &IPLimiter{
		rate:    rate,
		burst:   burst,
		maxKeys: maxKeys,
		buckets: make(map[string]*bucket, 64),
		stop:    make(chan struct{}),
	}
	go l.cleanupLoop()
	return l
}

// Stop ends the cleanup goroutine. Safe to call more than once.
func (l *IPLimiter) Stop() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() { close(l.stop) })
}

// Allow reports whether key may proceed and consumes one token when true.
func (l *IPLimiter) Allow(key string) bool {
	if l == nil || key == "" {
		return true
	}

	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxKeys {
			return false
		}
		l.buckets[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *IPLimiter) cleanupLoop() {
	ticker := time.NewTicker(cleanupEvery)
	defer ticker.Stop()
	for {
		select {
		case <-l.stop:
			return
		case now := <-ticker.C:
			l.evictIdle(now)
		}
	}
}

func (l *IPLimiter) evictIdle(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, b := range l.buckets {
		if now.Sub(b.last) > idleTTL {
			delete(l.buckets, key)
		}
	}
}

func withRateLimit(lim *IPLimiter, next http.Handler) http.Handler {
	if lim == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !lim.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}
