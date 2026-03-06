package space

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	spacepkg "taeu.kr/cohesion/internal/space"
)

type SearchIndexStore struct {
	db *sql.DB
	qb sq.StatementBuilderType
}

func NewSearchIndexStore(db *sql.DB) *SearchIndexStore {
	return &SearchIndexStore{
		db: db,
		qb: sq.StatementBuilder.PlaceholderFormat(sq.Question),
	}
}

func (s *SearchIndexStore) EnsureSpaceStates(ctx context.Context, spaceIDs []int64) error {
	if len(spaceIDs) == 0 {
		return nil
	}

	now := time.Now()
	for _, spaceID := range spaceIDs {
		if _, err := s.db.ExecContext(
			ctx,
			`INSERT INTO file_search_index_state(space_id, dirty, updated_at)
			 VALUES (?, 1, ?)
			 ON CONFLICT(space_id) DO NOTHING`,
			spaceID,
			now,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *SearchIndexStore) ListDirtySpaceIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT space_id FROM file_search_index_state WHERE dirty = 1 ORDER BY space_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spaceIDs := []int64{}
	for rows.Next() {
		var spaceID int64
		if err := rows.Scan(&spaceID); err != nil {
			return nil, err
		}
		spaceIDs = append(spaceIDs, spaceID)
	}
	return spaceIDs, rows.Err()
}

func (s *SearchIndexStore) ReplaceSpaceEntries(ctx context.Context, spaceID int64, entries []spacepkg.SearchIndexEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM file_search_index WHERE space_id = ?`, spaceID); err != nil {
		return err
	}

	now := time.Now()
	for _, entry := range entries {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO file_search_index(space_id, path, name, parent_path, is_dir, size, mod_time)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			spaceID,
			entry.Path,
			entry.Name,
			entry.ParentPath,
			boolToInt(entry.IsDir),
			entry.Size,
			entry.ModTime,
		); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO file_search_index_state(space_id, dirty, last_indexed_at, last_error, updated_at)
		 VALUES (?, 0, ?, NULL, ?)
		 ON CONFLICT(space_id) DO UPDATE SET
		   dirty = 0,
		   last_indexed_at = excluded.last_indexed_at,
		   last_error = NULL,
		   updated_at = excluded.updated_at`,
		spaceID,
		now,
		now,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SearchIndexStore) SearchEntries(ctx context.Context, spaceIDs []int64, queryLower string) ([]spacepkg.SearchIndexResult, error) {
	if len(spaceIDs) == 0 {
		return []spacepkg.SearchIndexResult{}, nil
	}

	query, args, err := s.qb.
		Select(
			"idx.space_id",
			"sp.space_name",
			"idx.name",
			"idx.path",
			"idx.parent_path",
			"idx.is_dir",
			"idx.size",
			"idx.mod_time",
		).
		From("file_search_index idx").
		Join("space sp ON sp.id = idx.space_id").
		Join("file_search_index_state st ON st.space_id = idx.space_id").
		Where(sq.Eq{"idx.space_id": spaceIDs}).
		Where(sq.Eq{"st.dirty": 0}).
		Where("LOWER(idx.name) LIKE ?", "%"+queryLower+"%").
		OrderBy("idx.name ASC", "idx.path ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []spacepkg.SearchIndexResult{}
	for rows.Next() {
		var item spacepkg.SearchIndexResult
		var isDir int
		if err := rows.Scan(
			&item.SpaceID,
			&item.SpaceName,
			&item.Name,
			&item.Path,
			&item.ParentPath,
			&isDir,
			&item.Size,
			&item.ModTime,
		); err != nil {
			return nil, err
		}
		item.IsDir = isDir == 1
		results = append(results, item)
	}
	return results, rows.Err()
}

func (s *SearchIndexStore) MarkSpaceDirty(ctx context.Context, spaceID int64) error {
	if spaceID <= 0 {
		return nil
	}
	now := time.Now()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO file_search_index_state(space_id, dirty, updated_at)
		 VALUES (?, 1, ?)
		 ON CONFLICT(space_id) DO UPDATE SET
		   dirty = 1,
		   updated_at = excluded.updated_at`,
		spaceID,
		now,
	)
	return err
}

func (s *SearchIndexStore) MarkSpacesDirty(ctx context.Context, spaceIDs []int64) error {
	for _, spaceID := range spaceIDs {
		if err := s.MarkSpaceDirty(ctx, spaceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SearchIndexStore) RecordIndexFailure(ctx context.Context, spaceID int64, failure string) error {
	if spaceID <= 0 || failure == "" {
		return nil
	}

	now := time.Now()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO file_search_index_state(space_id, dirty, last_error, updated_at)
		 VALUES (?, 1, ?, ?)
		 ON CONFLICT(space_id) DO UPDATE SET
		   dirty = 1,
		   last_error = excluded.last_error,
		   updated_at = excluded.updated_at`,
		spaceID,
		failure,
		now,
	)
	return err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
