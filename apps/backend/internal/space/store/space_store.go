package space

import (
	"context"
	"database/sql"
	"fmt"

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
