package space

import (
	"context"
	"database/sql"

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
	var spaces []*space.Space

	sqlQuery, args, err := s.qb.
		Select("*").
		From("spaces").
		ToSql()
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&spaces)
	if err != nil {
		return nil, err
	}

	return spaces, nil
}
