package appmiddleware

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())

		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r)
	})
}
