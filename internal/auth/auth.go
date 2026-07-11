package auth

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

func TokenAuth(localToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if localToken == "" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				slog.Warn("authentication failed: missing or malformed authorization header")
				writeUnauthorized(w)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				slog.Warn("authentication failed: empty token")
				writeUnauthorized(w)
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(localToken)) != 1 {
				slog.Warn("authentication failed: token mismatch")
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}
