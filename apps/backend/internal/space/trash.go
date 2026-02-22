package space

import "time"

type TrashItem struct {
	ID           int64     `json:"id"`
	SpaceID      int64     `json:"spaceId"`
	OriginalPath string    `json:"originalPath"`
	StoragePath  string    `json:"-"`
	ItemName     string    `json:"itemName"`
	IsDir        bool      `json:"isDir"`
	ItemSize     int64     `json:"itemSize"`
	DeletedBy    string    `json:"deletedBy"`
	DeletedAt    time.Time `json:"deletedAt"`
}

type CreateTrashItemRequest struct {
	SpaceID      int64
	OriginalPath string
	StoragePath  string
	ItemName     string
	IsDir        bool
	ItemSize     int64
	DeletedBy    string
}
