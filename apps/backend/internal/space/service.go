package space

import (
	"context"
)

type Storer interface {
	GetAll(ctx context.Context) ([]*Space, error)
	GetByName(ctx context.Context, name string) (*Space, error)
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

// 특정 이름의 Space 조회
func (s *Service) GetSpaceByName(ctx context.Context, name string) (*Space, error) {
	return s.store.GetByName(ctx, name)
}
