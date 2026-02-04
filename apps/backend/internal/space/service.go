package space

import (
	"context"
	"fmt"
)

type Storer interface {
	GetAll(ctx context.Context) ([]*Space, error)
	GetByName(ctx context.Context, name string) (*Space, error)
	Create(ctx context.Context, req *CreateSpaceRequest) (*Space, error)
	Delete(ctx context.Context, id int64) error
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

// CreateSpace는 새로운 Space를 생성합니다
func (s *Service) CreateSpace(ctx context.Context, req *CreateSpaceRequest) (*Space, error) {
	// 요청 데이터 검증
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 이름 중복 체크
	existingSpace, err := s.store.GetByName(ctx, req.SpaceName)
	if err == nil && existingSpace != nil {
		return nil, fmt.Errorf("space with name '%s' already exists", req.SpaceName)
	}

	// Store를 통해 생성
	createdSpace, err := s.store.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create space: %w", err)
	}

	return createdSpace, nil
}

// DeleteSpace는 Space를 삭제합니다
func (s *Service) DeleteSpace(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid space id: %d", id)
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete space: %w", err)
	}

	return nil
}
