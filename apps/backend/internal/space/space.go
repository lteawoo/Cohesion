package space

import "time"

type Space struct {
	ID            int64     `db:"id"`
	SpaceName     string    `db:"space_name"`
	SpaceDesc     string    `db:"space_desc"`
	SpacePath     string    `db:"space_path"`
	Icon          string    `db:"icon"`
	SpaceCategory string    `db:"space_category"`
	CreatedAt     time.Time `db:"created_at"`
	CreatedUserID string    `db:"created_user_id"`
	UpdatedAt     time.Time `db:"updated_at"`
	UpdatedUserID string    `db:"updated_user_id"`
}
