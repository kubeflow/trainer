package progress

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Middleware func(http.Handler) http.Handler

// chain applies middleware in order: first middleware wraps second, etc.
func chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// recoveryMiddleware recovers from panics in HTTP handlers to prevent Server crashes.
func recoveryMiddleware(log logr.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error(fmt.Errorf("panic: %v", err), "Panic in HTTP handler",
						"path", r.URL.Path, "method", r.Method)
					badRequest(w, log, "Internal Server Error", v1.StatusReasonInternalError, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// loggingMiddleware logs incoming HTTP requests.
func loggingMiddleware(log logr.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.V(5).Info("HTTP request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}

// bodySizeLimitMiddleware enforces a maximum request body size.
func bodySizeLimitMiddleware(log logr.Logger, maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Reject based on Content-Length header if present
			if r.ContentLength > maxBytes {
				badRequest(w, log, "Payload too large",
					v1.StatusReasonRequestEntityTooLarge,
					http.StatusRequestEntityTooLarge)
				return
			}

			// Wrap body to enforce limit for chunked/streaming requests
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// authenticationMiddleware validates the request has a valid projected service account token.
func authenticationMiddleware(log logr.Logger, verifier TokenVerifier) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				badRequest(w, log, "Missing Authorization header",
					v1.StatusReasonUnauthorized,
					http.StatusUnauthorized)
				return
			}

			// Check for "Bearer " prefix
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				badRequest(w, log, "Invalid Authorization header format",
					v1.StatusReasonUnauthorized,
					http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, bearerPrefix)
			if token == "" {
				badRequest(w, log, "Empty bearer token",
					v1.StatusReasonUnauthorized,
					http.StatusUnauthorized)
				return
			}

			// Verify and decode the token
			saToken, err := verifier.Verify(r.Context(), token)
			if err != nil || saToken == nil {
				log.V(5).Error(err, "Token validation failed")
				badRequest(w, log, "Invalid or expired token",
					v1.StatusReasonUnauthorized,
					http.StatusUnauthorized)
				return
			}

			// Add token to context for authorization
			ctx := withServiceAccountToken(r.Context(), *saToken)

			// Token is valid, proceed to next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
