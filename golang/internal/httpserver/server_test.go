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
	srv := New("8080", logger)

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
