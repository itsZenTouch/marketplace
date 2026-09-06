package appmiddleware

import (
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

				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
