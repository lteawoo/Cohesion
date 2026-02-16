package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"taeu.kr/cohesion/internal/account"
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

func (s *Store) ListUsers(ctx context.Context) ([]*account.User, error) {
	query, args, err := s.qb.
		Select("id", "username", "password_hash", "nickname", "role", "created_at", "updated_at").
		From("users").
		OrderBy("id ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*account.User{}
	for rows.Next() {
		var user account.User
		var role string
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Nickname, &role, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		user.Role = account.Role(role)
		users = append(users, &user)
	}
	return users, rows.Err()
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*account.User, error) {
	query, args, err := s.qb.
		Select("id", "username", "password_hash", "nickname", "role", "created_at", "updated_at").
		From("users").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var user account.User
	var role string
	if err := s.db.QueryRowContext(ctx, query, args...).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Nickname, &role, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with id %d not found", id)
		}
		return nil, err
	}
	user.Role = account.Role(role)
	return &user, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*account.User, error) {
	query, args, err := s.qb.
		Select("id", "username", "password_hash", "nickname", "role", "created_at", "updated_at").
		From("users").
		Where(sq.Eq{"username": username}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var user account.User
	var role string
	if err := s.db.QueryRowContext(ctx, query, args...).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Nickname, &role, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with username %q not found", username)
		}
		return nil, err
	}
	user.Role = account.Role(role)
	return &user, nil
}

func (s *Store) CreateUser(ctx context.Context, req *account.CreateUserRequest, passwordHash string) (*account.User, error) {
	now := time.Now()
	query, args, err := s.qb.
		Insert("users").
		Columns("username", "password_hash", "nickname", "role", "created_at", "updated_at").
		Values(req.Username, passwordHash, req.Nickname, string(req.Role), now, now).
		ToSql()
	if err != nil {
		return nil, err
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("username already exists")
		}
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) UpdateUser(ctx context.Context, id int64, req *account.UpdateUserRequest, passwordHash *string) (*account.User, error) {
	builder := s.qb.Update("users").Set("updated_at", time.Now()).Where(sq.Eq{"id": id})
	if req.Nickname != nil {
		builder = builder.Set("nickname", *req.Nickname)
	}
	if req.Role != nil {
		builder = builder.Set("role", string(*req.Role))
	}
	if passwordHash != nil {
		builder = builder.Set("password_hash", *passwordHash)
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, fmt.Errorf("user with id %d not found", id)
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	query, args, err := s.qb.Delete("users").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("user with id %d not found", id)
	}
	return nil
}

func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	query, args, err := s.qb.Select("COUNT(*)").From("users").Where(sq.Eq{"role": string(account.RoleAdmin)}).ToSql()
	if err != nil {
		return 0, err
	}
	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) GetUserPermissions(ctx context.Context, userID int64) ([]*account.UserSpacePermission, error) {
	query, args, err := s.qb.
		Select("user_id", "space_id", "permission").
		From("user_space_permissions").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := []*account.UserSpacePermission{}
	for rows.Next() {
		var permission account.UserSpacePermission
		var rawPermission string
		if err := rows.Scan(&permission.UserID, &permission.SpaceID, &rawPermission); err != nil {
			return nil, err
		}
		permission.Permission = account.Permission(rawPermission)
		permissions = append(permissions, &permission)
	}
	return permissions, rows.Err()
}

func (s *Store) ReplaceUserPermissions(ctx context.Context, userID int64, permissions []*account.UserSpacePermission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	deleteQuery, deleteArgs, err := s.qb.Delete("user_space_permissions").Where(sq.Eq{"user_id": userID}).ToSql()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, deleteQuery, deleteArgs...); err != nil {
		return err
	}

	for _, permission := range permissions {
		insertQuery, insertArgs, err := s.qb.
			Insert("user_space_permissions").
			Columns("user_id", "space_id", "permission", "created_at", "updated_at").
			Values(permission.UserID, permission.SpaceID, string(permission.Permission), time.Now(), time.Now()).
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, insertQuery, insertArgs...); err != nil {
			return err
		}
	}

	return tx.Commit()
}
