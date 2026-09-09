package appmiddleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5"
)

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				requestLogger := LoggerFromContext(r.Context())

				requestLogger.ErrorContext(
					r.Context(),
					"panic recovered",
					slog.Any("panic", recovered),
					slog.String(
						"route",
						chi.RouteContext(r.Context()).RoutePattern(),
					),
					slog.String(
						"stack",
						string(debug.Stack()),
					),
				)

				writeError(
					w,
					http.StatusInternalServerError,
					"internal server error",
				)

			}
		}()

		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
