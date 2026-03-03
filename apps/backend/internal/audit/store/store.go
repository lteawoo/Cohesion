package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"taeu.kr/cohesion/internal/audit"
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

func (s *Store) Create(ctx context.Context, event *audit.Event) error {
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal audit metadata: %w", err)
	}

	sqlQuery, args, err := s.qb.
		Insert("audit_logs").
		Columns(
			"occurred_at",
			"actor",
			"action",
			"result",
			"target",
			"request_id",
			"space_id",
			"metadata_json",
		).
		Values(
			event.OccurredAt,
			event.Actor,
			event.Action,
			string(event.Result),
			event.Target,
			event.RequestID,
			event.SpaceID,
			string(metadataJSON),
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build SQL query for Create audit log: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

func (s *Store) List(ctx context.Context, filter audit.ListFilter) ([]*audit.Log, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	queryBuilder := s.qb.
		Select(
			"id",
			"occurred_at",
			"actor",
			"action",
			"result",
			"target",
			"request_id",
			"space_id",
			"metadata_json",
		).
		From("audit_logs")

	queryBuilder = applyFilters(queryBuilder, filter)

	countQuery, countArgs, err := applyFilters(
		s.qb.Select("COUNT(*)").From("audit_logs"),
		filter,
	).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build SQL query for audit count: %w", err)
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to query audit count: %w", err)
	}

	offset := (filter.Page - 1) * filter.PageSize
	sqlQuery, args, err := queryBuilder.
		OrderBy("occurred_at DESC", "id DESC").
		Limit(uint64(filter.PageSize)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build SQL query for audit list: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	items := make([]*audit.Log, 0, filter.PageSize)
	for rows.Next() {
		item, scanErr := scanAuditLog(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("row iteration error in audit list: %w", err)
	}

	return items, total, nil
}

func (s *Store) GetByID(ctx context.Context, id int64) (*audit.Log, error) {
	sqlQuery, args, err := s.qb.
		Select(
			"id",
			"occurred_at",
			"actor",
			"action",
			"result",
			"target",
			"request_id",
			"space_id",
			"metadata_json",
		).
		From("audit_logs").
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL query for GetByID audit log: %w", err)
	}

	row := s.db.QueryRowContext(ctx, sqlQuery, args...)

	var (
		item         audit.Log
		resultRaw    string
		spaceID      sql.NullInt64
		metadataText string
	)
	if err := row.Scan(
		&item.ID,
		&item.OccurredAt,
		&item.Actor,
		&item.Action,
		&resultRaw,
		&item.Target,
		&item.RequestID,
		&spaceID,
		&metadataText,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("audit log with id %d not found", id)
		}
		return nil, fmt.Errorf("failed to scan audit log row: %w", err)
	}

	item.Result = audit.Result(resultRaw)
	if spaceID.Valid {
		item.SpaceID = &spaceID.Int64
	}
	item.Metadata = map[string]any{}
	if strings.TrimSpace(metadataText) != "" {
		if err := json.Unmarshal([]byte(metadataText), &item.Metadata); err != nil {
			return nil, fmt.Errorf("failed to decode audit metadata: %w", err)
		}
	}

	return &item, nil
}

func applyFilters(builder sq.SelectBuilder, filter audit.ListFilter) sq.SelectBuilder {
	if filter.From != nil {
		builder = builder.Where(sq.GtOrEq{"occurred_at": filter.From.UTC()})
	}
	if filter.To != nil {
		builder = builder.Where(sq.LtOrEq{"occurred_at": filter.To.UTC()})
	}
	if user := strings.TrimSpace(filter.User); user != "" {
		builder = builder.Where(sq.Eq{"actor": user})
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		builder = builder.Where(sq.Eq{"action": action})
	}
	if filter.SpaceID != nil {
		builder = builder.Where(sq.Eq{"space_id": *filter.SpaceID})
	}
	if filter.Result != "" {
		builder = builder.Where(sq.Eq{"result": string(filter.Result)})
	}
	return builder
}

func scanAuditLog(rows *sql.Rows) (*audit.Log, error) {
	var (
		item         audit.Log
		resultRaw    string
		spaceID      sql.NullInt64
		metadataText string
	)
	if err := rows.Scan(
		&item.ID,
		&item.OccurredAt,
		&item.Actor,
		&item.Action,
		&resultRaw,
		&item.Target,
		&item.RequestID,
		&spaceID,
		&metadataText,
	); err != nil {
		return nil, fmt.Errorf("failed to scan audit log row: %w", err)
	}

	item.Result = audit.Result(resultRaw)
	if spaceID.Valid {
		item.SpaceID = &spaceID.Int64
	}
	item.Metadata = map[string]any{}
	if strings.TrimSpace(metadataText) != "" {
		if err := json.Unmarshal([]byte(metadataText), &item.Metadata); err != nil {
			return nil, fmt.Errorf("failed to decode audit metadata: %w", err)
		}
	}

	return &item, nil
}

var _ interface {
	Create(context.Context, *audit.Event) error
	List(context.Context, audit.ListFilter) ([]*audit.Log, int64, error)
	GetByID(context.Context, int64) (*audit.Log, error)
} = (*Store)(nil)
