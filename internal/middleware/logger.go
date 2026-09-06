package appmiddleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func Slogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := chimiddleware.NewWrapResponseWriter(
				w,
				r.ProtoMajor,
			)

			requestID := chimiddleware.GetReqID(r.Context())

			requestLogger := logger.With(
				slog.String("request_id", requestID),
			)

			ctx := withLogger(
				r.Context(),
				requestLogger,
			)

			r = r.WithContext(ctx)

			next.ServeHTTP(ww, r)

			status := ww.Status()

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Duration("duration", time.Since(start)),
			}

			if route := chi.RouteContext(r.Context()).RoutePattern(); route != "" {
				attrs = append(
					attrs,
					slog.String("route", route),
				)
			}

			switch {
			case status >= 500:
				requestLogger.LogAttrs(
					r.Context(),
					slog.LevelError,
					"http request",
					attrs...,
				)

			case status >= 400:
				requestLogger.LogAttrs(
					r.Context(),
					slog.LevelWarn,
					"http request",
					attrs...,
				)

			default:
				requestLogger.LogAttrs(
					r.Context(),
					slog.LevelInfo,
					"http request",
					attrs...,
				)
			}
		})
	}
}
