package httpapi

import (
	"fmt"
	"net/http"
	"strings"
)

// SetSecurityHeaders sets standard security headers.
func SetSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
}

// SetCSP sets the Content-Security-Policy header.
func SetCSP(w http.ResponseWriter, extraConnectSrc ...string) {
	connectSrc := "'self'"
	if len(extraConnectSrc) > 0 {
		connectSrc = strings.Join(append([]string{"'self'"}, extraConnectSrc...), " ")
	}
	w.Header().Set("Content-Security-Policy",
		fmt.Sprintf("default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src %s", connectSrc))
}
