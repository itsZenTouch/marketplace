package appmiddleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestRequestID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := chimiddleware.RequestID(
		RequestID(next),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	requestID := rec.Header().Get("X-Request-ID")

	if requestID == "" {
		t.Fatal("X-Request-ID header is empty")
	}
}

func TestRealIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "uses X-Forwarded-For",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.10, 10.0.0.2",
			},
			want: "203.0.113.10",
		},
		{
			name:       "uses X-Real-IP when X-Forwarded-For is absent",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.20",
			},
			want: "203.0.113.20",
		},
		{
			name:       "falls back to RemoteAddr",
			remoteAddr: "192.0.2.10:4321",
			want:       "192.0.2.10",
		},
		{
			name:       "falls back to raw RemoteAddr when not host port",
			remoteAddr: "192.0.2.10",
			want:       "192.0.2.10",
		},
		{
			name:       "ignores invalid X-Forwarded-For",
			remoteAddr: "192.0.2.10:4321",
			headers: map[string]string{
				"X-Forwarded-For": "not-an-ip",
			},
			want: "192.0.2.10",
		},
		{
			name:       "ignores invalid X-Real-IP",
			remoteAddr: "192.0.2.10:4321",
			headers: map[string]string{
				"X-Real-IP": "not-an-ip",
			},
			want: "192.0.2.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.RemoteAddr; got != tt.want {
					t.Fatalf(
						"RemoteAddr = %q, want %q",
						got,
						tt.want,
					)
				}

				w.WriteHeader(http.StatusNoContent)
			})

			handler := RealIP(next)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)
		})
	}
}

func TestRecoverer(t *testing.T) {
	t.Run("recovers panic and returns internal server error", func(t *testing.T) {
		logger := slog.New(
			slog.NewTextHandler(
				bytes.NewBuffer(nil),
				nil,
			),
		)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		handler := Slogger(logger)(Recoverer(next))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if got := rec.Code; got != http.StatusInternalServerError {
			t.Fatalf(
				"status = %d, want %d",
				got,
				http.StatusInternalServerError,
			)
		}

		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf(
				"Content-Type = %q, want %q",
				got,
				"application/json",
			)
		}

		var response map[string]string

		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if got := response["error"]; got != "internal server error" {
			t.Fatalf(
				"error = %q, want %q",
				got,
				"internal server error",
			)
		}
	})
}
