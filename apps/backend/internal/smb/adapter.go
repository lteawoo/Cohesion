package smb

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/lteawoo/smb-core"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/space"
)

type coreAuthenticator struct {
	accountService *account.Service
}

func (a *coreAuthenticator) Authenticate(ctx context.Context, username, password string) (string, error) {
	ok, err := a.accountService.Authenticate(ctx, username, password)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", os.ErrPermission
	}
	return username, nil
}

func (a *coreAuthenticator) ResolveSMBPassword(ctx context.Context, username string) (string, error) {
	return a.accountService.ResolveSMBPassword(ctx, username)
}

type coreAuthorizer struct {
	spaceService   *space.Service
	accountService *account.Service
}

func (a *coreAuthorizer) CanAccessSpace(ctx context.Context, principal, spaceName string, required smbcore.Permission) (bool, error) {
	spaceObj, err := a.spaceService.GetSpaceByName(ctx, spaceName)
	if err != nil {
		return false, os.ErrNotExist
	}
	allowed, err := a.accountService.CanAccessSpaceByID(ctx, principal, spaceObj.ID, toAccountPermission(required))
	if err != nil {
		return false, err
	}
	return allowed, nil
}

type coreFileSystem struct {
	spaceService   *space.Service
	accountService *account.Service
}

func (f *coreFileSystem) Stat(ctx context.Context, principal, virtualPath string) (smbcore.DirEntry, error) {
	guard := NewGuard(f.spaceService, f.accountService, principal)
	spaceObj, absPath, relPath, err := guard.ResolvePath(virtualPath, account.PermissionRead)
	if err != nil {
		return smbcore.DirEntry{}, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return smbcore.DirEntry{}, err
	}

	name := info.Name()
	if relPath == "" {
		name = spaceObj.SpaceName
	}
	return smbcore.DirEntry{
		Name:  name,
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}, nil
}

func (f *coreFileSystem) List(ctx context.Context, principal, virtualPath string) ([]smbcore.DirEntry, error) {
	guard := NewGuard(f.spaceService, f.accountService, principal)
	_, absPath, _, err := guard.ResolvePath(virtualPath, account.PermissionRead)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	result := make([]smbcore.DirEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		result = append(result, smbcore.DirEntry{
			Name:  info.Name(),
			IsDir: info.IsDir(),
			Size:  info.Size(),
		})
	}
	return result, nil
}

func (f *coreFileSystem) Read(ctx context.Context, principal, virtualPath string, offset, limit int64) ([]byte, error) {
	guard := NewGuard(f.spaceService, f.accountService, principal)
	_, absPath, relPath, err := guard.ResolvePath(virtualPath, account.PermissionRead)
	if err != nil {
		return nil, err
	}
	if relPath == "" {
		return nil, os.ErrPermission
	}
	if offset < 0 {
		return nil, os.ErrInvalid
	}
	if limit < 0 {
		return nil, os.ErrInvalid
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, os.ErrPermission
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	reader := io.Reader(file)
	if limit > 0 {
		reader = io.LimitReader(file, limit)
	}
	return io.ReadAll(reader)
}

func (f *coreFileSystem) CreateOrTruncate(ctx context.Context, principal, virtualPath string) (smbcore.DirEntry, error) {
	guard := NewGuard(f.spaceService, f.accountService, principal)
	_, absPath, relPath, err := guard.ResolvePath(virtualPath, account.PermissionWrite)
	if err != nil {
		return smbcore.DirEntry{}, err
	}
	if relPath == "" {
		return smbcore.DirEntry{}, os.ErrPermission
	}

	parentDir := filepath.Dir(absPath)
	if parentDir == "" || parentDir == "." {
		return smbcore.DirEntry{}, os.ErrPermission
	}
	if info, err := os.Stat(parentDir); err != nil {
		return smbcore.DirEntry{}, err
	} else if !info.IsDir() {
		return smbcore.DirEntry{}, os.ErrNotExist
	}

	file, err := os.OpenFile(absPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return smbcore.DirEntry{}, err
	}
	defer file.Close()

	if err := file.Truncate(0); err != nil {
		return smbcore.DirEntry{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return smbcore.DirEntry{}, err
	}

	info, err := file.Stat()
	if err != nil {
		return smbcore.DirEntry{}, err
	}
	if info.IsDir() {
		return smbcore.DirEntry{}, os.ErrPermission
	}

	return smbcore.DirEntry{
		Name:  info.Name(),
		IsDir: false,
		Size:  info.Size(),
	}, nil
}

func (f *coreFileSystem) Write(ctx context.Context, principal, virtualPath string, offset int64, data []byte) (int64, error) {
	guard := NewGuard(f.spaceService, f.accountService, principal)
	_, absPath, relPath, err := guard.ResolvePath(virtualPath, account.PermissionWrite)
	if err != nil {
		return 0, err
	}
	if relPath == "" {
		return 0, os.ErrPermission
	}
	if offset < 0 {
		return 0, os.ErrInvalid
	}

	file, err := os.OpenFile(absPath, os.O_WRONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, os.ErrNotExist
		}
		return 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return 0, os.ErrPermission
	}

	n, err := file.WriteAt(data, offset)
	if err != nil {
		return int64(n), err
	}
	return int64(n), nil
}

func (f *coreFileSystem) Mkdir(ctx context.Context, principal, virtualPath string) (smbcore.DirEntry, error) {
	guard := NewGuard(f.spaceService, f.accountService, principal)
	_, absPath, relPath, err := guard.ResolvePath(virtualPath, account.PermissionManage)
	if err != nil {
		return smbcore.DirEntry{}, err
	}
	if relPath == "" {
		return smbcore.DirEntry{}, errors.Join(smbcore.ErrPermissionDenied, os.ErrPermission)
	}
	if err := os.Mkdir(absPath, 0755); err != nil {
		return smbcore.DirEntry{}, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return smbcore.DirEntry{}, err
	}
	return smbcore.DirEntry{Name: info.Name(), IsDir: true, Size: 0}, nil
}

func (f *coreFileSystem) Rename(ctx context.Context, principal, fromPath, toPath string) error {
	guard := NewGuard(f.spaceService, f.accountService, principal)
	if err := guard.ValidateRename(fromPath, toPath); err != nil {
		return err
	}
	_, absFrom, relFrom, err := guard.ResolvePath(fromPath, account.PermissionManage)
	if err != nil {
		return err
	}
	_, absTo, relTo, err := guard.ResolvePath(toPath, account.PermissionManage)
	if err != nil {
		return err
	}
	if relFrom == "" || relTo == "" {
		return errors.Join(smbcore.ErrPermissionDenied, os.ErrPermission)
	}
	return os.Rename(absFrom, absTo)
}

func (f *coreFileSystem) Delete(ctx context.Context, principal, virtualPath string, isDir bool) error {
	guard := NewGuard(f.spaceService, f.accountService, principal)
	_, absPath, relPath, err := guard.ResolvePath(virtualPath, account.PermissionManage)
	if err != nil {
		return err
	}
	if relPath == "" {
		return errors.Join(smbcore.ErrPermissionDenied, os.ErrPermission)
	}
	if isDir {
		return os.Remove(absPath)
	}
	return os.Remove(absPath)
}

func toAccountPermission(permission smbcore.Permission) account.Permission {
	switch permission {
	case smbcore.PermissionWrite:
		return account.PermissionWrite
	case smbcore.PermissionManage:
		return account.PermissionManage
	default:
		return account.PermissionRead
	}
}
