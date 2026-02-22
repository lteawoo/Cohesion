package space

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	spaceDomain "taeu.kr/cohesion/internal/space"
)

type TrashStore struct {
	db *sql.DB
	qb sq.StatementBuilderType
}

func NewTrashStore(db *sql.DB) *TrashStore {
	return &TrashStore{
		db: db,
		qb: sq.StatementBuilder.PlaceholderFormat(sq.Question),
	}
}

func (s *TrashStore) CreateTrashItem(ctx context.Context, req *spaceDomain.CreateTrashItemRequest) (*spaceDomain.TrashItem, error) {
	now := time.Now()
	isDir := 0
	if req.IsDir {
		isDir = 1
	}

	sqlQuery, args, err := s.qb.
		Insert("trash_items").
		Columns(
			"space_id",
			"original_path",
			"storage_path",
			"item_name",
			"is_dir",
			"item_size",
			"deleted_by",
			"deleted_at",
		).
		Values(
			req.SpaceID,
			req.OriginalPath,
			req.StoragePath,
			req.ItemName,
			isDir,
			req.ItemSize,
			req.DeletedBy,
			now,
		).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for CreateTrashItem: %w", err)
	}

	result, err := s.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("trash item already exists for storage path: %w", err)
		}
		return nil, fmt.Errorf("failed to insert trash item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get trash item id: %w", err)
	}

	return &spaceDomain.TrashItem{
		ID:           id,
		SpaceID:      req.SpaceID,
		OriginalPath: req.OriginalPath,
		StoragePath:  req.StoragePath,
		ItemName:     req.ItemName,
		IsDir:        req.IsDir,
		ItemSize:     req.ItemSize,
		DeletedBy:    req.DeletedBy,
		DeletedAt:    now,
	}, nil
}

func (s *TrashStore) ListTrashItemsBySpace(ctx context.Context, spaceID int64) ([]*spaceDomain.TrashItem, error) {
	sqlQuery, args, err := s.qb.
		Select(
			"id",
			"space_id",
			"original_path",
			"storage_path",
			"item_name",
			"is_dir",
			"item_size",
			"deleted_by",
			"deleted_at",
		).
		From("trash_items").
		Where(sq.Eq{"space_id": spaceID}).
		OrderBy("deleted_at DESC", "id DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for ListTrashItemsBySpace: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query trash items: %w", err)
	}
	defer rows.Close()

	items := make([]*spaceDomain.TrashItem, 0)
	for rows.Next() {
		var item spaceDomain.TrashItem
		var isDirInt int
		if err := rows.Scan(
			&item.ID,
			&item.SpaceID,
			&item.OriginalPath,
			&item.StoragePath,
			&item.ItemName,
			&isDirInt,
			&item.ItemSize,
			&item.DeletedBy,
			&item.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan trash item row: %w", err)
		}
		item.IsDir = isDirInt == 1
		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error in ListTrashItemsBySpace: %w", err)
	}

	return items, nil
}

func (s *TrashStore) GetTrashItemByID(ctx context.Context, id int64) (*spaceDomain.TrashItem, error) {
	sqlQuery, args, err := s.qb.
		Select(
			"id",
			"space_id",
			"original_path",
			"storage_path",
			"item_name",
			"is_dir",
			"item_size",
			"deleted_by",
			"deleted_at",
		).
		From("trash_items").
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for GetTrashItemByID: %w", err)
	}

	row := s.db.QueryRowContext(ctx, sqlQuery, args...)

	var item spaceDomain.TrashItem
	var isDirInt int
	if err := row.Scan(
		&item.ID,
		&item.SpaceID,
		&item.OriginalPath,
		&item.StoragePath,
		&item.ItemName,
		&isDirInt,
		&item.ItemSize,
		&item.DeletedBy,
		&item.DeletedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("trash item with id %d not found", id)
		}
		return nil, fmt.Errorf("failed to scan trash item row: %w", err)
	}
	item.IsDir = isDirInt == 1

	return &item, nil
}

func (s *TrashStore) DeleteTrashItemByID(ctx context.Context, id int64) error {
	sqlQuery, args, err := s.qb.
		Delete("trash_items").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build SQL query for DeleteTrashItemByID: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("failed to delete trash item: %w", err)
	}
	return nil
}

func (s *TrashStore) DeleteTrashItemsBySpace(ctx context.Context, spaceID int64) error {
	sqlQuery, args, err := s.qb.
		Delete("trash_items").
		Where(sq.Eq{"space_id": spaceID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build SQL query for DeleteTrashItemsBySpace: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("failed to delete trash items by space: %w", err)
	}
	return nil
}
