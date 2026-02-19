package auth

import (
	"errors"
	"time"
)

const (
	AccessCookieName  = "cohesion_access_token"
	RefreshCookieName = "cohesion_refresh_token"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidToken       = errors.New("invalid token")
	ErrSetupRequired      = errors.New("initial setup required")
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type Config struct {
	Secret         string
	Issuer         string
	AccessTokenTTL time.Duration
	RefreshTTL     time.Duration
}
