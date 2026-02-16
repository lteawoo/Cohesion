package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"taeu.kr/cohesion/internal/account"
)

var publicAPIPaths = map[string]struct{}{
	"/api/health":       {},
	"/api/auth/login":   {},
	"/api/auth/refresh": {},
	"/api/auth/logout":  {},
}

var adminOnlyPrefixes = []string{
	"/api/accounts",
}

var adminOnlyPaths = map[string]struct{}{
	"/api/config":         {},
	"/api/system/restart": {},
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		if _, ok := publicAPIPaths[r.URL.Path]; ok {
			next.ServeHTTP(w, r)
			return
		}

		accessCookie, err := r.Cookie(AccessCookieName)
		if err != nil || accessCookie.Value == "" {
			writeUnauthorized(w)
			return
		}

		claims, err := s.ParseToken(accessCookie.Value, "access")
		if err != nil {
			writeUnauthorized(w)
			return
		}

		if isAdminOnlyPath(r.URL.Path) && claims.Role != account.RoleAdmin {
			writeForbidden(w)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Unauthorized",
	})
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Forbidden",
	})
}

func isAdminOnlyPath(path string) bool {
	if _, ok := adminOnlyPaths[path]; ok {
		return true
	}
	for _, prefix := range adminOnlyPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
