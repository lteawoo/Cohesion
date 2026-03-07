package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"taeu.kr/cohesion/internal/audit"
)

var publicAPIPaths = map[string]struct{}{
	"/api/health":         {},
	"/api/system/version": {},
	"/api/auth/login":     {},
	"/api/auth/refresh":   {},
	"/api/auth/logout":    {},
	"/api/setup/status":   {},
	"/api/setup/admin":    {},
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deniedRule, shouldAuditDenied := deniedAuditRuleForRequest(r)

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
			if shouldAuditDenied && deniedRule.AllowUnauthorized {
				r = s.recordDeniedAudit(r, deniedRule, "", "invalid_token", "auth.invalid_token", http.StatusUnauthorized)
			}
			writeUnauthorized(w)
			return
		}
		currentUser, err := s.resolveCurrentUserFromClaims(r.Context(), claims)
		if err != nil {
			if shouldAuditDenied && deniedRule.AllowUnauthorized {
				r = s.recordDeniedAudit(r, deniedRule, "", "invalid_token_subject", "auth.invalid_subject", http.StatusUnauthorized)
			}
			writeUnauthorized(w)
			return
		}
		claims.UserID = currentUser.ID
		claims.Username = currentUser.Username
		claims.Nickname = currentUser.Nickname
		claims.Role = currentUser.Role

		if requiredPermission, ok := requiredPermissionForRequest(r); ok {
			allowed, err := s.HasPermission(r.Context(), claims.Role, requiredPermission)
			if err != nil {
				writeInternalServerError(w)
				return
			}
			if !allowed {
				if shouldAuditDenied {
					r = s.recordDeniedAudit(r, deniedRule, claims.Username, "permission_denied", "auth.permission_denied", http.StatusForbidden)
				}
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
				if shouldAuditDenied {
					r = s.recordDeniedAudit(r, deniedRule, claims.Username, "space_permission_denied", "auth.space_permission_denied", http.StatusForbidden)
				}
				writeForbidden(w)
				return
			}
		}

		r.Header.Set("X-Cohesion-Actor", claims.Username)
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

func (s *Service) recordDeniedAudit(
	r *http.Request,
	rule deniedAuditRule,
	actor string,
	reason string,
	code string,
	status int,
) *http.Request {
	if s.auditRecorder == nil || DeniedAuditRecorded(r.Context()) || strings.TrimSpace(rule.Action) == "" {
		return r
	}

	event := audit.Event{
		Action:    rule.Action,
		Result:    audit.ResultDenied,
		Actor:     strings.TrimSpace(actor),
		Target:    deniedAuditTargetForRequest(r),
		RequestID: strings.TrimSpace(r.Header.Get("X-Request-Id")),
		Metadata: map[string]any{
			"reason": strings.TrimSpace(reason),
			"code":   strings.TrimSpace(code),
			"status": status,
		},
	}
	if spaceID, ok := extractSpaceID(r.URL.Path); ok {
		event.SpaceID = &spaceID
	}

	s.auditRecorder.RecordBestEffort(event)
	return r.WithContext(WithDeniedAuditRecorded(r.Context()))
}

func deniedAuditTargetForRequest(r *http.Request) string {
	queryPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if queryPath != "" {
		return queryPath
	}

	trimmedPath := strings.TrimPrefix(strings.TrimSpace(r.URL.Path), "/api/")
	if trimmedPath == "" {
		return "api"
	}
	return trimmedPath
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
