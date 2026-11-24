package space

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type FileRequest struct {
	ID        int64  `json:"id"`
	Owner     int64  `json:"-"`
	SpaceID   int64  `json:"-"`
	Root      string `json:"-"`
	Folder    string `json:"-"`
	Token     string `json:"token"`
	Title     string `json:"title"`
	Expires   int64  `json:"expires"`
	ByteLimit int64  `json:"byteLimit"`
	Received  int64  `json:"received"`
	Closed    bool   `json:"closed"`
}

const requestColumns = "id,owner,space_id,root,folder,token,title,expires,byte_limit,received,closed"

func scanFileRequest(row interface{ Scan(...any) error }) (*FileRequest, error) {
	var f FileRequest
	err := row.Scan(&f.ID, &f.Owner, &f.SpaceID, &f.Root, &f.Folder, &f.Token, &f.Title, &f.Expires, &f.ByteLimit, &f.Received, &f.Closed)
	return &f, err
}

func (s *TransferStore) CreateFileRequest(ctx context.Context, f *FileRequest) error {
	res, err := s.db.ExecContext(ctx, `INSERT INTO file_requests(owner,space_id,root,folder,token,title,expires,byte_limit) VALUES(?,?,?,?,?,?,?,?)`, f.Owner, f.SpaceID, f.Root, f.Folder, f.Token, f.Title, f.Expires, f.ByteLimit)
	if err == nil {
		f.ID, err = res.LastInsertId()
	}
	return err
}

func (s *TransferStore) FileRequests(ctx context.Context, owner, spaceID int64, folder string) ([]FileRequest, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+requestColumns+" FROM file_requests WHERE owner=? AND space_id=? AND folder=? AND closed=0 AND expires>? ORDER BY id DESC", owner, spaceID, folder, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []FileRequest{}
	for rows.Next() {
		f, err := scanFileRequest(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *f)
	}
	return list, rows.Err()
}

func (s *TransferStore) FileRequestForToken(ctx context.Context, token string) (*FileRequest, error) {
	return scanFileRequest(s.db.QueryRowContext(ctx, "SELECT "+requestColumns+" FROM file_requests WHERE token=? AND closed=0 AND expires>?", token, time.Now().Unix()))
}

func (s *TransferStore) CloseFileRequest(ctx context.Context, id, owner, spaceID int64) error {
	res, err := s.db.ExecContext(ctx, "UPDATE file_requests SET closed=1 WHERE id=? AND owner=? AND space_id=?", id, owner, spaceID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}

var ErrFileRequestFull = errors.New("file request capacity unavailable")

// Both the upload record and its share of the request's byte limit must commit together.
func (s *TransferStore) StartFileRequestUpload(ctx context.Context, u *UploadSession) error {
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO upload_sessions(id,stage_path,expires,snapshot) VALUES(?,?,?,?)", u.ID, u.Stage, u.Expires, data); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO file_request_uploads(upload_id,request_id,size)
 SELECT ?,id,? FROM file_requests WHERE id=? AND closed=0 AND expires>?
 AND ? <= byte_limit-received-(SELECT COALESCE(SUM(size),0) FROM file_request_uploads WHERE request_id=file_requests.id)
 AND (SELECT COUNT(*) FROM file_request_uploads WHERE request_id=file_requests.id)<MIN(32,10000-received_files)`, u.ID, u.Size, u.RequestID, time.Now().Unix(), u.Size)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrFileRequestFull
	}
	return tx.Commit()
}

func (s *TransferStore) InactiveRequestUploads(ctx context.Context, now int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.upload_id FROM file_request_uploads u LEFT JOIN file_requests f ON f.id=u.request_id WHERE f.id IS NULL OR f.closed=1 OR f.expires<=?`, now)
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
