package account

import "time"

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type Permission string

const (
	PermissionRead   Permission = "read"
	PermissionWrite  Permission = "write"
	PermissionManage Permission = "manage"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Nickname     string    `json:"nickname"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type UserSpacePermission struct {
	UserID     int64      `json:"userId"`
	SpaceID    int64      `json:"spaceId"`
	Permission Permission `json:"permission"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Role     Role   `json:"role"`
}

type UpdateUserRequest struct {
	Nickname *string `json:"nickname,omitempty"`
	Password *string `json:"password,omitempty"`
	Role     *Role   `json:"role,omitempty"`
}

func (p Permission) Allows(required Permission) bool {
	rank := map[Permission]int{
		PermissionRead:   1,
		PermissionWrite:  2,
		PermissionManage: 3,
	}
	return rank[p] >= rank[required]
}
