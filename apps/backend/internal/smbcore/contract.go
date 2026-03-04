package smbcore

import (
	"context"
	"errors"
	"net"
)

type Dialect string

const (
	Dialect210 Dialect = "2.1"
	Dialect300 Dialect = "3.0"
	Dialect302 Dialect = "3.0.2"
	Dialect311 Dialect = "3.1.1"
)

type Permission string

const (
	PermissionRead   Permission = "read"
	PermissionWrite  Permission = "write"
	PermissionManage Permission = "manage"
)

type RolloutPhase string

const (
	RolloutPhaseReadOnly  RolloutPhase = "readonly"
	RolloutPhaseWriteSafe RolloutPhase = "write-safe"
	RolloutPhaseWriteFull RolloutPhase = "write-full"
)

const (
	DenyReasonReadonlyPhaseDenied = "readonly_phase_denied"
	DenyReasonPermissionDenied    = "permission_denied"
	DenyReasonPathBoundary        = "path_boundary_violation"
)

var (
	ErrPermissionDenied    = errors.New(DenyReasonPermissionDenied)
	ErrPathBoundary        = errors.New(DenyReasonPathBoundary)
	ErrReadonlyPhaseDenied = errors.New(DenyReasonReadonlyPhaseDenied)
)

type Authenticator interface {
	Authenticate(ctx context.Context, username, password string) (principal string, err error)
	ResolveSMBPassword(ctx context.Context, username string) (password string, err error)
}

type Authorizer interface {
	CanAccessSpace(ctx context.Context, principal, spaceName string, required Permission) (bool, error)
}

type FileSystem interface {
	Stat(ctx context.Context, principal, virtualPath string) (DirEntry, error)
	List(ctx context.Context, principal, virtualPath string) ([]DirEntry, error)
	Read(ctx context.Context, principal, virtualPath string, offset, limit int64) ([]byte, error)
	CreateOrTruncate(ctx context.Context, principal, virtualPath string) (DirEntry, error)
	Write(ctx context.Context, principal, virtualPath string, offset int64, data []byte) (int64, error)
	Mkdir(ctx context.Context, principal, virtualPath string) (DirEntry, error)
	Rename(ctx context.Context, principal, fromPath, toPath string) error
	Delete(ctx context.Context, principal, virtualPath string, isDir bool) error
}

type DirEntry struct {
	Name  string
	IsDir bool
	Size  int64
}

type Event struct {
	Stage  string
	Reason string
	Err    error
}

type Telemetry interface {
	OnEvent(event Event)
}

type Runtime interface {
	HandleConn(ctx context.Context, conn net.Conn) error
	Supports(dialect Dialect) bool
	IsReadOnly() bool
	Phase() RolloutPhase
}
