package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// ReadJSON reads the request body and decodes it as JSON into dst.
func ReadJSON(r *http.Request, dst any) error {
	body, err := ReadBody(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

// ReadBody reads the entire request body and returns it as a byte slice.
func ReadBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	return body, nil
}

// RequireMethod returns an error response if the request method doesn't match and returns false.
func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

// GetClientIP extracts the real client IP from the request.
func GetClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Only trust X-Forwarded-For from localhost (where nginx runs)
	if host == "127.0.0.1" || host == "::1" {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	return host
}
