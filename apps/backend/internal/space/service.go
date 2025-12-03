package space

import (
	"context"
)

type Storer interface {
	GetAll(ctx context.Context) ([]*Space, error)
}

type Service struct {
	store Storer
}

func NewService(store Storer) *Service {
	return &Service{
		store: store,
	}
}
