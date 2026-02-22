package space

import (
	"context"
	"fmt"
)

type TrashStorer interface {
	CreateTrashItem(ctx context.Context, req *CreateTrashItemRequest) (*TrashItem, error)
	ListTrashItemsBySpace(ctx context.Context, spaceID int64) ([]*TrashItem, error)
	GetTrashItemByID(ctx context.Context, id int64) (*TrashItem, error)
	DeleteTrashItemByID(ctx context.Context, id int64) error
	DeleteTrashItemsBySpace(ctx context.Context, spaceID int64) error
}

type TrashService struct {
	store TrashStorer
}

func NewTrashService(store TrashStorer) *TrashService {
	return &TrashService{
		store: store,
	}
}

func (s *TrashService) CreateTrashItem(ctx context.Context, req *CreateTrashItemRequest) (*TrashItem, error) {
	if req == nil {
		return nil, fmt.Errorf("trash create request is required")
	}
	if req.SpaceID <= 0 {
		return nil, fmt.Errorf("invalid space id: %d", req.SpaceID)
	}
	if req.OriginalPath == "" {
		return nil, fmt.Errorf("original path is required")
	}
	if req.StoragePath == "" {
		return nil, fmt.Errorf("storage path is required")
	}
	if req.ItemName == "" {
		return nil, fmt.Errorf("item name is required")
	}
	if req.DeletedBy == "" {
		return nil, fmt.Errorf("deleted by is required")
	}
	return s.store.CreateTrashItem(ctx, req)
}

func (s *TrashService) ListTrashItems(ctx context.Context, spaceID int64) ([]*TrashItem, error) {
	if spaceID <= 0 {
		return nil, fmt.Errorf("invalid space id: %d", spaceID)
	}
	return s.store.ListTrashItemsBySpace(ctx, spaceID)
}

func (s *TrashService) GetTrashItem(ctx context.Context, id int64) (*TrashItem, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid trash item id: %d", id)
	}
	return s.store.GetTrashItemByID(ctx, id)
}

func (s *TrashService) DeleteTrashItem(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid trash item id: %d", id)
	}
	return s.store.DeleteTrashItemByID(ctx, id)
}

func (s *TrashService) DeleteTrashItemsBySpace(ctx context.Context, spaceID int64) error {
	if spaceID <= 0 {
		return fmt.Errorf("invalid space id: %d", spaceID)
	}
	return s.store.DeleteTrashItemsBySpace(ctx, spaceID)
}
