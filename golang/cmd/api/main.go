package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang-api/internal/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	lim := newLimiterFromEnv()
	defer lim.Stop()

	nodejsURL, err := httpserver.HTTPBaseURL(os.Getenv("NODEJS_URL"), "http://127.0.0.1:8081")
	if err != nil {
		logger.Error("invalid NODEJS_URL", slog.String("error", err.Error()))
		os.Exit(1)
	}

	srv := httpserver.New(port, logger, lim, nodejsURL)
	logger.Info("nodejs_url", slog.String("url", nodejsURL))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("golang-api listening", slog.String("addr", srv.Addr))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server exited", slog.String("error", err.Error()))
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("golang-api stopped")
	}
}

func newLimiterFromEnv() *httpserver.IPLimiter {
	rps := envFloat("RATE_LIMIT_RPS", 5)
	if rps <= 0 {
		return nil
	}
	return httpserver.NewIPLimiter(httpserver.LimitConfig{
		RatePerSec: rps,
		Burst:      envInt("RATE_LIMIT_BURST", 20),
		MaxKeys:    envInt("RATE_LIMIT_MAX_KEYS", 10000),
	})
}

func envFloat(key string, fallback float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}
