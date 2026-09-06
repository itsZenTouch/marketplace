package appmiddleware

import (
	"net"
	"net/http"
	"strings"
)

func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		if ip != "" {
			r.RemoteAddr = ip
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	// Sesuaikan dengan infrastructure kamu.
	//
	// Kalau aplikasi berada di belakang trusted reverse proxy,
	// proxy tersebut bisa mengisi X-Forwarded-For.
	//
	// Jangan gunakan header ini secara blindly jika server
	// bisa diakses langsung dari internet.

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")

		// Untuk proxy yang trusted, biasanya IP pertama adalah
		// original client IP.
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])

			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		if net.ParseIP(xrip) != nil {
			return xrip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}
