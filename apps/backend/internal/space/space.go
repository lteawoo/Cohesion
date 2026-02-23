package space

import (
	"errors"
	"os"
	"time"
)

type Space struct {
	ID            int64      `db:"id" json:"id"`
	SpaceName     string     `db:"space_name" json:"space_name"`
	SpaceDesc     *string    `db:"space_desc" json:"space_desc,omitempty"`
	SpacePath     string     `db:"space_path" json:"space_path"`
	Icon          *string    `db:"icon" json:"icon,omitempty"`
	SpaceCategory *string    `db:"space_category" json:"space_category,omitempty"`
	QuotaBytes    *int64     `db:"quota_bytes" json:"quota_bytes,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	CreatedUserID *string    `db:"created_user_id" json:"created_user_id,omitempty"`
	UpdatedAt     *time.Time `db:"updated_at" json:"updated_at,omitempty"`
	UpdatedUserID *string    `db:"updated_user_id" json:"updated_user_id,omitempty"`
}

// CreateSpaceRequest는 Space 생성 요청 데이터를 정의합니다
type CreateSpaceRequest struct {
	SpaceName     string  `json:"space_name"`
	SpaceDesc     *string `json:"space_desc,omitempty"`
	SpacePath     string  `json:"space_path"`
	Icon          *string `json:"icon,omitempty"`
	SpaceCategory *string `json:"space_category,omitempty"`
	QuotaBytes    *int64  `json:"quota_bytes,omitempty"`
}

// Validate는 CreateSpaceRequest의 유효성을 검사합니다
func (req *CreateSpaceRequest) Validate() error {
	if req.SpaceName == "" {
		return errors.New("space_name is required")
	}
	if req.SpacePath == "" {
		return errors.New("space_path is required")
	}

	// Space 이름 길이 제한
	if len(req.SpaceName) > 100 {
		return errors.New("space_name must be less than 100 characters")
	}
	if req.QuotaBytes != nil && *req.QuotaBytes < 0 {
		return errors.New("quota_bytes must be greater than or equal to 0")
	}

	// 경로 유효성 검사
	if _, err := os.Stat(req.SpacePath); os.IsNotExist(err) {
		return errors.New("space_path does not exist")
	}

	return nil
}

// CreateSpaceResponse는 Space 생성 응답 데이터를 정의합니다
type CreateSpaceResponse struct {
	ID        int64  `json:"id"`
	SpaceName string `json:"space_name"`
	Message   string `json:"message"`
}
