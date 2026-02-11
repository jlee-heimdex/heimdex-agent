package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/heimdex/heimdex-agent/internal/catalog"
	"github.com/heimdex/heimdex-agent/internal/logging"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

var heimdexSubdomainRe = regexp.MustCompile(`^https?://[a-z0-9]([a-z0-9-]*[a-z0-9])?\.app\.heimdex\.(co|local)(:[0-9]+)?$`)

var localhostWithPortRe = regexp.MustCompile(`^http://(localhost|127\.0\.0\.1)(:[0-9]+)?$`)

func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	lower := strings.ToLower(origin)

	if localhostWithPortRe.MatchString(lower) {
		return true
	}

	if heimdexSubdomainRe.MatchString(lower) {
		return true
	}

	return false
}

func isLoopbackRemoteAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	return ip.IsLoopback()
}

func CORSAllowlist() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" {
				if !isAllowedOrigin(origin) {
					if r.Method == http.MethodOptions {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					next.ServeHTTP(w, r)
					return
				}

				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type, Authorization, X-Heimdex-Request-Id, X-Heimdex-Device-Id")
				w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Accept-Ranges, Content-Length, Content-Type")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func LoopbackGuard() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				WriteError(w, http.StatusForbidden, "playback is only available from localhost", "FORBIDDEN")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func AuthMiddleware(repo catalog.Repository, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				WriteError(w, http.StatusUnauthorized, "missing authorization header", "UNAUTHORIZED")
				return
			}

			if !strings.HasPrefix(auth, "Bearer ") {
				WriteError(w, http.StatusUnauthorized, "invalid authorization format", "UNAUTHORIZED")
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")

			storedToken, err := repo.GetConfig(r.Context(), "auth_token")
			if err != nil || storedToken == "" {
				logger.Error("failed to get auth token from config", "error", err)
				WriteError(w, http.StatusInternalServerError, "auth configuration error", "INTERNAL_ERROR")
				return
			}

			if token != storedToken {
				logger.Warn("invalid auth token", "provided", logging.SanitizeToken(token))
				WriteError(w, http.StatusUnauthorized, "invalid token", "UNAUTHORIZED")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			requestID, _ := r.Context().Value(RequestIDKey).(string)
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", requestID,
			)
		})
	}
}

func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID, _ := r.Context().Value(RequestIDKey).(string)
					logger.Error("panic recovered", "error", err, "request_id", requestID)
					WriteError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := catalog.NewID()[:8]
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
			w.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func WriteError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message, Code: code})
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
