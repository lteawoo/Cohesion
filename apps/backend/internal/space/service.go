package space

import (
	"context"
	"fmt"
)

type Storer interface {
	GetAll(ctx context.Context) ([]*Space, error)
	GetByName(ctx context.Context, name string) (*Space, error)
	GetByID(ctx context.Context, id int64) (*Space, error)
	Create(ctx context.Context, req *CreateSpaceRequest) (*Space, error)
	Delete(ctx context.Context, id int64) error
}

type quotaUpdatable interface {
	UpdateQuota(ctx context.Context, id int64, quotaBytes *int64) (*Space, error)
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

// 특정 ID의 Space 조회
func (s *Service) GetSpaceByID(ctx context.Context, id int64) (*Space, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid space id: %d", id)
	}
	return s.store.GetByID(ctx, id)
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

// UpdateSpaceQuota는 Space 쿼터를 갱신합니다. nil이면 무제한으로 설정됩니다.
func (s *Service) UpdateSpaceQuota(ctx context.Context, id int64, quotaBytes *int64) (*Space, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid space id: %d", id)
	}
	if quotaBytes != nil && *quotaBytes < 0 {
		return nil, fmt.Errorf("invalid quota bytes: %d", *quotaBytes)
	}

	updatable, ok := s.store.(quotaUpdatable)
	if !ok {
		return nil, fmt.Errorf("space store does not support quota updates")
	}

	updated, err := updatable.UpdateQuota(ctx, id, quotaBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to update space quota: %w", err)
	}
	return updated, nil
}
