package space

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"taeu.kr/cohesion/internal/space"
)

type Store struct {
	db *sql.DB
	qb sq.StatementBuilderType
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		db: db,
		qb: sq.StatementBuilder.PlaceholderFormat(sq.Question),
	}
}

func (s *Store) GetAll(ctx context.Context) ([]*space.Space, error) {
	sqlQuery, args, err := s.qb.
		Select(
			"id",
			"space_name",
			"space_desc",
			"space_path",
			"icon",
			"space_category",
			"quota_bytes",
			"created_at",
			"created_user_id",
			"updated_at",
			"updated_user_id",
		).
		From("space").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for GetAll: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query space: %w", err)
	}
	defer rows.Close()

	var spaces []*space.Space

	for rows.Next() {
		var sp space.Space
		if err := rows.Scan(
			&sp.ID,
			&sp.SpaceName,
			&sp.SpaceDesc,
			&sp.SpacePath,
			&sp.Icon,
			&sp.SpaceCategory,
			&sp.QuotaBytes,
			&sp.CreatedAt,
			&sp.CreatedUserID,
			&sp.UpdatedAt,
			&sp.UpdatedUserID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan space row: %w", err)
		}
		spaces = append(spaces, &sp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error in GetAll: %w", err)
	}

	if spaces == nil {
		spaces = []*space.Space{}
	}

	return spaces, nil
}

func (s *Store) GetByName(ctx context.Context, name string) (*space.Space, error) {
	sqlQuery, args, err := s.qb.
		Select(
			"id",
			"space_name",
			"space_desc",
			"space_path",
			"icon",
			"space_category",
			"quota_bytes",
			"created_at",
			"created_user_id",
			"updated_at",
			"updated_user_id",
		).
		From("space").
		Where(sq.Eq{"space_name": name}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for GetAll: %w", err)
	}

	row := s.db.QueryRowContext(ctx, sqlQuery, args...)

	var sp space.Space
	if err := row.Scan(
		&sp.ID,
		&sp.SpaceName,
		&sp.SpaceDesc,
		&sp.SpacePath,
		&sp.Icon,
		&sp.SpaceCategory,
		&sp.QuotaBytes,
		&sp.CreatedAt,
		&sp.CreatedUserID,
		&sp.UpdatedAt,
		&sp.UpdatedUserID,
	); err != nil {
		return nil, fmt.Errorf("failed to scan space row: %w", err)
	}

	return &sp, nil
}

func (s *Store) GetByID(ctx context.Context, id int64) (*space.Space, error) {
	sqlQuery, args, err := s.qb.
		Select(
			"id",
			"space_name",
			"space_desc",
			"space_path",
			"icon",
			"space_category",
			"quota_bytes",
			"created_at",
			"created_user_id",
			"updated_at",
			"updated_user_id",
		).
		From("space").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for GetByID: %w", err)
	}

	row := s.db.QueryRowContext(ctx, sqlQuery, args...)

	var sp space.Space
	if err := row.Scan(
		&sp.ID,
		&sp.SpaceName,
		&sp.SpaceDesc,
		&sp.SpacePath,
		&sp.Icon,
		&sp.SpaceCategory,
		&sp.QuotaBytes,
		&sp.CreatedAt,
		&sp.CreatedUserID,
		&sp.UpdatedAt,
		&sp.UpdatedUserID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("space with id %d not found", id)
		}
		return nil, fmt.Errorf("failed to scan space row: %w", err)
	}

	return &sp, nil
}

// Create는 새로운 Space를 데이터베이스에 저장합니다
func (s *Store) Create(ctx context.Context, req *space.CreateSpaceRequest) (*space.Space, error) {
	now := time.Now()

	// INSERT 쿼리 빌드
	sqlQuery, args, err := s.qb.
		Insert("space").
		Columns(
			"space_name",
			"space_desc",
			"space_path",
			"icon",
			"space_category",
			"quota_bytes",
			"created_at",
		).
		Values(
			req.SpaceName,
			req.SpaceDesc,
			req.SpacePath,
			req.Icon,
			req.SpaceCategory,
			req.QuotaBytes,
			now,
		).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for Create: %w", err)
	}

	// 실행 및 ID 가져오기
	result, err := s.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		// UNIQUE 제약조건 위반 체크 (SQLite)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("space with this name or path already exists: %w", err)
		}
		return nil, fmt.Errorf("failed to insert space: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	// 생성된 Space 객체 반환
	return &space.Space{
		ID:            id,
		SpaceName:     req.SpaceName,
		SpaceDesc:     req.SpaceDesc,
		SpacePath:     req.SpacePath,
		Icon:          req.Icon,
		SpaceCategory: req.SpaceCategory,
		QuotaBytes:    req.QuotaBytes,
		CreatedAt:     now,
		CreatedUserID: nil,
		UpdatedAt:     nil,
		UpdatedUserID: nil,
	}, nil
}

func (s *Store) UpdateQuota(ctx context.Context, id int64, quotaBytes *int64) (*space.Space, error) {
	sqlQuery, args, err := s.qb.
		Update("space").
		Set("quota_bytes", quotaBytes).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for UpdateQuota: %w", err)
	}

	result, err := s.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update space quota: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected for UpdateQuota: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("space with id %d not found", id)
	}

	return s.GetByID(ctx, id)
}

// Delete는 Space를 삭제합니다
func (s *Store) Delete(ctx context.Context, id int64) error {
	sqlQuery, args, err := s.qb.
		Delete("space").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return fmt.Errorf("failed to build SQL query for Delete: %w", err)
	}

	result, err := s.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("failed to delete space: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("space with id %d not found", id)
	}

	return nil
}
