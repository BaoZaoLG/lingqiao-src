package main

import (
	"net/http"
	"os"
	"strings"
)

func corsMiddleware(next http.Handler) http.Handler {
	adminOrigin := os.Getenv("ADMIN_ORIGIN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		isAdmin := strings.HasPrefix(r.URL.Path, "/admin")

		if isAdmin {
			setCredentialedCORSOrigin(w, r, adminOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-HMAC-Signature, X-Client-ID, X-Timestamp, X-Nonce, X-Session-Token")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		applyBodyLimit(w, r)
		setSecurityHeaders(w)

		if r.URL.Path != "/api/v1/dll" &&
			!strings.HasPrefix(r.URL.Path, "/api/v1/update/package/") &&
			!strings.HasPrefix(r.URL.Path, "/admin/api/update/download") {
			setCSP(w)
		}

		next.ServeHTTP(w, r)
	})
}

func corsMiddlewareAgent(next http.Handler) http.Handler {
	agentOrigin := os.Getenv("AGENT_ORIGIN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCredentialedCORSOrigin(w, r, agentOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		setSecurityHeaders(w)
		setCSP(w)

		next.ServeHTTP(w, r)
	})
}

// setCredentialedCORSOrigin only allows an explicit configured origin for
// cookie-authenticated admin and agent routes.
func setCredentialedCORSOrigin(w http.ResponseWriter, r *http.Request, allowedOrigin string) {
	origin := r.Header.Get("Origin")
	if allowedOrigin != "" && origin == allowedOrigin {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

// applyBodyLimit sets the request body size limit based on the endpoint.
func applyBodyLimit(w http.ResponseWriter, r *http.Request) {
	limit := int64(1 << 20) // 1MB default
	if strings.HasPrefix(r.URL.Path, "/admin/api/payload/upload") ||
		strings.HasPrefix(r.URL.Path, "/admin/api/update/upload") ||
		strings.Contains(r.URL.Path, "/packages") {
		limit = 200 << 20 // 200MB for uploads
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
}
