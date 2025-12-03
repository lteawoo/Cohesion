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

// 모든 Space 조회
func (s *Service) GetAllSpaces(ctx context.Context) ([]*Space, error) {
	return s.store.GetAll(ctx)
}
