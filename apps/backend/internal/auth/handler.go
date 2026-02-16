package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"taeu.kr/cohesion/internal/platform/web"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/auth/login", web.Handler(h.handleLogin))
	mux.Handle("POST /api/auth/refresh", web.Handler(h.handleRefresh))
	mux.Handle("POST /api/auth/logout", web.Handler(h.handleLogout))
	mux.Handle("GET /api/auth/me", web.Handler(h.handleMe))
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authUserResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) *web.Error {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}

	tokenPair, user, err := h.service.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if err == ErrInvalidCredentials {
			return &web.Error{Code: http.StatusUnauthorized, Message: "Invalid credentials", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to login", Err: err}
	}

	setAuthCookies(w, r, tokenPair)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": authUserResponse{
			ID:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
			Role:     string(user.Role),
		},
	})
	return nil
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) *web.Error {
	refreshCookie, err := r.Cookie(RefreshCookieName)
	if err != nil || refreshCookie.Value == "" {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Refresh token not found", Err: err}
	}

	tokenPair, user, err := h.service.Refresh(r.Context(), refreshCookie.Value)
	if err != nil {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Invalid refresh token", Err: err}
	}

	setAuthCookies(w, r, tokenPair)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": authUserResponse{
			ID:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
			Role:     string(user.Role),
		},
	})
	return nil
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) *web.Error {
	clearAuthCookies(w, r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) *web.Error {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authUserResponse{
		ID:       claims.UserID,
		Username: claims.Username,
		Nickname: claims.Nickname,
		Role:     string(claims.Role),
	})
	return nil
}

func setAuthCookies(w http.ResponseWriter, r *http.Request, tokenPair *TokenPair) {
	secure := r.TLS != nil
	now := time.Now()

	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    tokenPair.AccessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  now.Add(15 * time.Minute),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    tokenPair.RefreshToken,
		Path:     "/api/auth/refresh",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  now.Add(7 * 24 * time.Hour),
	})
}

func clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil
	expired := time.Unix(0, 0)

	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expired,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/api/auth/refresh",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expired,
		MaxAge:   -1,
	})
}
