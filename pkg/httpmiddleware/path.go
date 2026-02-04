package httpmiddleware

import (
	"net/http"
	"strings"
)

// StripPrefix middleware removes a prefix from request URLs
func StripPrefix(prefix string) func(http.Handler) http.Handler {
	if prefix == "" {
		// Return a no-op middleware if no prefix
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only strip if the prefix matches exactly as a path segment
			if strings.HasPrefix(r.URL.Path, prefix) {
				// Check if it's an exact match or followed by a slash
				if len(r.URL.Path) == len(prefix) || r.URL.Path[len(prefix)] == '/' {
					r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
