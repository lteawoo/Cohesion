package space

import (
	"context"
	"database/sql"
	"encoding/json"
)

type TransferStore struct {
	db *sql.DB
}

func NewTransferStore(db *sql.DB) *TransferStore {
	return &TransferStore{db: db}
}

func (s *TransferStore) ListArchiveJobs(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, snapshot FROM archive_download_jobs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make(map[string]json.RawMessage)
	for rows.Next() {
		var id string
		var snapshot []byte
		if err := rows.Scan(&id, &snapshot); err != nil {
			return nil, err
		}
		jobs[id] = snapshot
	}
	return jobs, rows.Err()
}

func (s *TransferStore) SaveArchiveJob(ctx context.Context, id string, snapshot []byte) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO archive_download_jobs (id, snapshot) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET snapshot = excluded.snapshot
	`, id, snapshot)
	return err
}

func (s *TransferStore) DeleteArchiveJob(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM archive_download_jobs WHERE id = ?", id)
	return err
}

func (s *TransferStore) SaveUploadReservation(ctx context.Context, id string, spaceID, deltaBytes int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO upload_quota_reservations (id, space_id, delta_bytes) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET space_id = excluded.space_id, delta_bytes = excluded.delta_bytes
	`, id, spaceID, deltaBytes)
	return err
}

func (s *TransferStore) DeleteUploadReservation(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM upload_quota_reservations WHERE id = ?", id)
	return err
}

func (s *TransferStore) AddUploadStage(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO upload_staging_files (path) VALUES (?)", path)
	return err
}

func (s *TransferStore) AbandonUploadStage(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE upload_staging_files SET abandoned = 1 WHERE path = ?", path)
	return err
}

func (s *TransferStore) DeleteUploadStage(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM upload_staging_files WHERE path = ?", path)
	return err
}

func (s *TransferStore) RecoverUploadTransfers(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM upload_quota_reservations"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE upload_staging_files SET abandoned = 1"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *TransferStore) ListAbandonedUploadStages(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT path FROM upload_staging_files WHERE abandoned = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
