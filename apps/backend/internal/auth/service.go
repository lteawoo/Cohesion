package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"taeu.kr/cohesion/internal/account"
)

type Claims struct {
	UserID   int64        `json:"userId"`
	Username string       `json:"username"`
	Nickname string       `json:"nickname"`
	Role     account.Role `json:"role"`
	Type     string       `json:"type"`
	jwt.RegisteredClaims
}

type Service struct {
	accountService *account.Service
	config         Config
}

func NewService(accountService *account.Service, config Config) *Service {
	return &Service{
		accountService: accountService,
		config:         config,
	}
}

func (s *Service) Login(ctx context.Context, username, password string) (*TokenPair, *account.User, error) {
	needsSetup, err := s.accountService.NeedsBootstrap(ctx)
	if err != nil {
		return nil, nil, err
	}
	if needsSetup {
		return nil, nil, ErrSetupRequired
	}

	authed, err := s.accountService.Authenticate(ctx, username, password)
	if err != nil {
		return nil, nil, err
	}
	if !authed {
		return nil, nil, ErrInvalidCredentials
	}

	user, err := s.accountService.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokenPair, err := s.IssueTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	return tokenPair, user, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, *account.User, error) {
	claims, err := s.ParseToken(refreshToken, "refresh")
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	user, err := s.accountService.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	tokenPair, err := s.IssueTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	return tokenPair, user, nil
}

func (s *Service) IssueTokenPair(user *account.User) (*TokenPair, error) {
	access, err := s.signToken(user, "access", s.config.AccessTokenTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := s.signToken(user, "refresh", s.config.RefreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (s *Service) ParseToken(tokenString string, expectedType string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.config.Secret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Type != expectedType {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *Service) signToken(user *account.User, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Nickname: user.Nickname,
		Role:     user.Role,
		Type:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.config.Issuer,
			Subject:   user.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.config.Secret))
	if err != nil {
		return "", errors.New("failed to sign token")
	}
	return signedToken, nil
}
