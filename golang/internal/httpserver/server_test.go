package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHelloRoutes(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := New("8080", logger, nil, "http://127.0.0.1:8081")

	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "get", method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: helloBody},
		{name: "post", method: http.MethodPost, path: "/", wantStatus: http.StatusOK, wantBody: helloBody},
		{name: "missing", method: http.MethodGet, path: "/nope", wantStatus: http.StatusNotFound},
		{name: "put", method: http.MethodPut, path: "/", wantStatus: http.StatusMethodNotAllowed},
		{name: "call nodejs down", method: http.MethodGet, path: "/call-nodejs", wantStatus: http.StatusBadGateway},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			srv.Handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody == "" {
				return
			}
			got := rec.Body.String()
			if got != tc.wantBody {
				t.Fatalf("body = %q, want %q", got, tc.wantBody)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
				t.Fatalf("content-type = %q, want text/plain", ct)
			}
		})
	}
}

func TestCallNodejs(t *testing.T) {
	t.Parallel()

	nodejs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "hello world-nodes\n")
	}))
	t.Cleanup(nodejs.Close)

	srv := New("8080", discardLogger(), nil, nodejs.URL)
	req := httptest.NewRequest(http.MethodGet, "/call-nodejs", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "hello world-nodes\n" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHTTPBaseURL(t *testing.T) {
	t.Parallel()

	got, err := HTTPBaseURL("", "http://127.0.0.1:8081")
	if err != nil || got != "http://127.0.0.1:8081" {
		t.Fatalf("empty env: got %q err %v", got, err)
	}

	got, err = HTTPBaseURL("https://nodejs-api.example.run.app/", "")
	if err != nil || got != "https://nodejs-api.example.run.app" {
		t.Fatalf("trim slash: got %q err %v", got, err)
	}

	if _, err := HTTPBaseURL("ftp://evil", "http://127.0.0.1:8081"); err == nil {
		t.Fatal("expected scheme error")
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
