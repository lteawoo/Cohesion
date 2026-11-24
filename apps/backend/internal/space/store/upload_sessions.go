package space

import (
	"context"
	"encoding/json"
)

// UploadSession holds the metadata needed to resume receiving or publishing a file.
type UploadSession struct {
	ID, Stage, Root, Path, Name, Policy   string
	Owner, SpaceID, Size, Offset, Expires int64
	Destination                           string
	PublishReady                          bool
}

func (s *TransferStore) SaveSession(ctx context.Context, u *UploadSession) error {
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO upload_sessions(id,stage_path,expires,snapshot) VALUES(?,?,?,?)
 ON CONFLICT(id) DO UPDATE SET expires=excluded.expires,snapshot=excluded.snapshot`, u.ID, u.Stage, u.Expires, data)
	return err
}

func (s *TransferStore) Session(ctx context.Context, id string) (*UploadSession, error) {
	var data []byte
	if err := s.db.QueryRowContext(ctx, "SELECT snapshot FROM upload_sessions WHERE id=?", id).Scan(&data); err != nil {
		return nil, err
	}
	var u UploadSession
	err := json.Unmarshal(data, &u)
	return &u, err
}

func (s *TransferStore) SessionIDs(ctx context.Context, before int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM upload_sessions WHERE expires <= ?", before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *TransferStore) RestoreSessionReservations(ctx context.Context, restore func(string, int64, int64)) error {
	rows, err := s.db.QueryContext(ctx, "SELECT id,space_id,delta_bytes FROM upload_quota_reservations")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var spaceID, delta int64
		if err := rows.Scan(&id, &spaceID, &delta); err != nil {
			return err
		}
		restore(id, spaceID, delta)
	}
	return rows.Err()
}

func (s *TransferStore) CloseSession(ctx context.Context, u *UploadSession, status string, expires int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if status != "" {
		if _, err = tx.ExecContext(ctx, "INSERT INTO upload_receipts(id,owner,space_id,filename,status,expires) VALUES(?,?,?,?,?,?)", u.ID, u.Owner, u.SpaceID, u.Destination, status, expires); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM upload_sessions WHERE id=?", u.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM upload_quota_reservations WHERE id=?", u.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE upload_staging_files SET abandoned=1 WHERE path=?", u.Stage); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *TransferStore) UploadReceipt(ctx context.Context, id string, owner, spaceID, now int64) (status, name string, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT status,filename FROM upload_receipts WHERE id=? AND owner=? AND space_id=? AND expires>?", id, owner, spaceID, now).Scan(&status, &name)
	return
}

func (s *TransferStore) ExpireReceipts(ctx context.Context, now int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM upload_receipts WHERE expires<=?", now)
	return err
}
