package space

import (
	"context"
	"encoding/json"
	"path"
)

type DownloadLinkItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

const BundleDownloadName = "cohesion-files.zip"

type DownloadLink struct {
	Items     []DownloadLinkItem `json:"-"`
	ItemCount int                `json:"itemCount,omitempty"`
	ID        int64              `json:"id"`
	Token     string             `json:"token"`
	Snapshot  string             `json:"-"`
	Owner     int64              `json:"-"`
	SpaceID   int64              `json:"-"`
	Path      string             `json:"path"`
	Name      string             `json:"name"`
	Size      int64              `json:"size"`
	Expires   int64              `json:"expires"`
}

const linkColumns = "id, token, snapshot, owner, space_id, path, size, expires, manifest"

func scanDownloadLink(row interface{ Scan(...any) error }) (DownloadLink, error) {
	var l DownloadLink
	var manifest string
	err := row.Scan(&l.ID, &l.Token, &l.Snapshot, &l.Owner, &l.SpaceID, &l.Path, &l.Size, &l.Expires, &manifest)
	if err == nil && manifest != "" {
		err = json.Unmarshal([]byte(manifest), &l.Items)
		l.ItemCount = len(l.Items)
		l.Name = BundleDownloadName
	} else {
		l.Name = path.Base(l.Path)
	}
	return l, err
}

func (s *TransferStore) CreateDownloadLink(ctx context.Context, l *DownloadLink) error {
	var manifest string
	if len(l.Items) != 0 {
		data, err := json.Marshal(l.Items)
		if err != nil {
			return err
		}
		manifest = string(data)
	}
	return s.db.QueryRowContext(ctx, `INSERT INTO download_links(token,snapshot,owner,space_id,path,size,expires,manifest)
 VALUES(?,?,?,?,?,?,?,?) RETURNING id`, l.Token, l.Snapshot, l.Owner, l.SpaceID, l.Path, l.Size, l.Expires, manifest).Scan(&l.ID)
}

func (s *TransferStore) DownloadLinks(ctx context.Context, spaceID int64, filePath string, now int64) ([]DownloadLink, error) {
	query := "SELECT " + linkColumns + " FROM download_links WHERE space_id=? AND expires>?"
	args := []any{spaceID, now}
	if filePath != "" {
		query += " AND path=?"
		args = append(args, filePath)
	}
	rows, err := s.db.QueryContext(ctx, query+" ORDER BY id DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []DownloadLink{}
	for rows.Next() {
		l, err := scanDownloadLink(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (s *TransferStore) ActiveDownloadLink(ctx context.Context, token string, now int64) (DownloadLink, error) {
	return scanDownloadLink(s.db.QueryRowContext(ctx, "SELECT "+linkColumns+` FROM download_links WHERE token=? AND expires>? AND EXISTS(SELECT 1 FROM space WHERE id=space_id) AND EXISTS(SELECT 1 FROM users WHERE id=owner)`, token, now))
}

func (s *TransferStore) DeleteDownloadLink(ctx context.Context, id, spaceID int64) (string, error) {
	var snapshot string
	err := s.db.QueryRowContext(ctx, "DELETE FROM download_links WHERE id=? AND space_id=? RETURNING snapshot", id, spaceID).Scan(&snapshot)
	return snapshot, err
}

func (s *TransferStore) LiveDownloadSnapshots(ctx context.Context, now int64) (map[string]bool, error) {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM download_links WHERE expires<=? OR NOT EXISTS(SELECT 1 FROM space WHERE id=space_id) OR NOT EXISTS(SELECT 1 FROM users WHERE id=owner)`, now); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT snapshot FROM download_links")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	live := map[string]bool{}
	for rows.Next() {
		var snapshot string
		if err := rows.Scan(&snapshot); err != nil {
			return nil, err
		}
		live[snapshot] = true
	}
	return live, rows.Err()
}

func (s *TransferStore) DownloadSnapshotUsage(ctx context.Context, spaceID int64, now int64) (int64, error) {
	var size int64
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(size),0) FROM download_links WHERE space_id=? AND expires>?", spaceID, now).Scan(&size)
	return size, err
}
