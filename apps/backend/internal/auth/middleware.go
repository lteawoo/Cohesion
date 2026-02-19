package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

var publicAPIPaths = map[string]struct{}{
	"/api/health":       {},
	"/api/auth/login":   {},
	"/api/auth/refresh": {},
	"/api/auth/logout":  {},
	"/api/setup/status": {},
	"/api/setup/admin":  {},
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

		if requiredPermission, ok := requiredPermissionForRequest(r); ok {
			allowed, err := s.HasPermission(r.Context(), claims.Role, requiredPermission)
			if err != nil {
				writeInternalServerError(w)
				return
			}
			if !allowed {
				writeForbidden(w)
				return
			}
		}
		if spacePermission, ok := requiredSpacePermissionForRequest(r); ok {
			allowed, err := s.accountService.CanAccessSpaceByID(r.Context(), claims.Username, spacePermission.spaceID, spacePermission.required)
			if err != nil {
				writeInternalServerError(w)
				return
			}
			if !allowed {
				writeForbidden(w)
				return
			}
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

func writeInternalServerError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Internal server error",
	})
}
