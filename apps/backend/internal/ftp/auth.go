package ftp

import (
	"context"

	"taeu.kr/cohesion/internal/account"
)

type accountAuth struct {
	accountService *account.Service
}

func (a *accountAuth) CheckPasswd(username, password string) (bool, error) {
	return a.accountService.Authenticate(context.Background(), username, password)
}
