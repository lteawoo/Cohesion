package webdav

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/webdav"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/space"
)

// SpaceFS는 WebDAV FileSystem 인터페이스를 구현하여
// 루트에서 모든 Space를 가상 디렉토리로 노출한다.
type SpaceFS struct {
	spaceService   *space.Service
	accountService *account.Service
}

func NewSpaceFS(spaceService *space.Service, accountService *account.Service) webdav.FileSystem {
	return &SpaceFS{
		spaceService:   spaceService,
		accountService: accountService,
	}
}

// parsePath는 WebDAV 경로에서 spaceName과 나머지 경로를 분리한다.
// "/" → ("", "")
// "/myspace" → ("myspace", "/")
// "/myspace/folder/file.txt" → ("myspace", "/folder/file.txt")
func parsePath(name string) (spaceName, remainder string) {
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		return "", ""
	}

	parts := strings.SplitN(name, "/", 2)
	spaceName = parts[0]
	if len(parts) > 1 {
		remainder = "/" + parts[1]
	} else {
		remainder = "/"
	}
	return spaceName, remainder
}

// resolveRealPath는 spaceName과 나머지 경로를 실제 OS 경로로 변환한다.
func (sfs *SpaceFS) resolveRealPath(ctx context.Context, spaceName, remainder string) (string, error) {
	sp, err := sfs.spaceService.GetSpaceByName(ctx, spaceName)
	if err != nil {
		return "", os.ErrNotExist
	}

	realPath := filepath.Join(sp.SpacePath, filepath.FromSlash(remainder))
	realPath = filepath.Clean(realPath)

	// Space 경로 밖으로 나가는 것 방지
	if !isPathWithinSpace(realPath, sp.SpacePath) {
		return "", os.ErrPermission
	}

	return realPath, nil
}

func (sfs *SpaceFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	spaceName, remainder := parsePath(name)
	if spaceName == "" {
		return os.ErrPermission
	}

	realPath, err := sfs.resolveRealPath(ctx, spaceName, remainder)
	if err != nil {
		return err
	}

	return os.Mkdir(realPath, perm)
}

func (sfs *SpaceFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	spaceName, remainder := parsePath(name)

	// 루트: 가상 디렉토리 (Space 목록)
	if spaceName == "" {
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
			return nil, os.ErrPermission
		}
		return &spaceRootDir{
			spaceService:   sfs.spaceService,
			accountService: sfs.accountService,
			ctx:            ctx,
		}, nil
	}

	// Space 루트: 실제 디렉토리를 열되, 이름은 Space 이름으로 표시
	realPath, err := sfs.resolveRealPath(ctx, spaceName, remainder)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(realPath, flag, perm)
	if err != nil {
		return nil, err
	}

	// Space 루트 디렉토리인 경우, Stat에서 Space 이름을 반환하도록 래핑
	if remainder == "/" {
		return &spaceNamedFile{File: f, name: spaceName}, nil
	}

	return f, nil
}

func (sfs *SpaceFS) RemoveAll(ctx context.Context, name string) error {
	spaceName, remainder := parsePath(name)
	if spaceName == "" || remainder == "/" {
		return os.ErrPermission
	}

	realPath, err := sfs.resolveRealPath(ctx, spaceName, remainder)
	if err != nil {
		return err
	}

	return os.RemoveAll(realPath)
}

func (sfs *SpaceFS) Rename(ctx context.Context, oldName, newName string) error {
	oldSpace, oldRem := parsePath(oldName)
	newSpace, newRem := parsePath(newName)

	if oldSpace == "" || newSpace == "" || oldRem == "/" || newRem == "/" {
		return os.ErrPermission
	}

	oldPath, err := sfs.resolveRealPath(ctx, oldSpace, oldRem)
	if err != nil {
		return err
	}

	newPath, err := sfs.resolveRealPath(ctx, newSpace, newRem)
	if err != nil {
		return err
	}

	return os.Rename(oldPath, newPath)
}

func (sfs *SpaceFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	spaceName, remainder := parsePath(name)

	// 루트
	if spaceName == "" {
		return &virtualDirInfo{name: "/", modTime: time.Now()}, nil
	}

	realPath, err := sfs.resolveRealPath(ctx, spaceName, remainder)
	if err != nil {
		return nil, err
	}

	fi, err := os.Stat(realPath)
	if err != nil {
		return nil, err
	}

	// Space 루트 디렉토리인 경우, 이름을 Space 이름으로 표시
	if remainder == "/" {
		return &virtualDirInfo{name: spaceName, modTime: fi.ModTime()}, nil
	}

	return fi, nil
}

// --- 가상 루트 디렉토리 (Space 목록을 반환하는 File) ---

type spaceRootDir struct {
	spaceService   *space.Service
	accountService *account.Service
	ctx            context.Context
	entries        []os.FileInfo
	pos            int
}

func (d *spaceRootDir) Read([]byte) (int, error) {
	return 0, fmt.Errorf("cannot read directory")
}

func (d *spaceRootDir) Write([]byte) (int, error) {
	return 0, os.ErrPermission
}

func (d *spaceRootDir) Seek(int64, int) (int64, error) {
	return 0, nil
}

func (d *spaceRootDir) Close() error {
	return nil
}

func (d *spaceRootDir) Stat() (fs.FileInfo, error) {
	return &virtualDirInfo{name: "/", modTime: time.Now()}, nil
}

func (d *spaceRootDir) Readdir(count int) ([]fs.FileInfo, error) {
	if d.entries == nil {
		username, ok := UsernameFromContext(d.ctx)
		if !ok {
			return nil, os.ErrPermission
		}

		spaces, err := d.spaceService.GetAllSpaces(d.ctx)
		if err != nil {
			return nil, err
		}

		d.entries = make([]os.FileInfo, 0, len(spaces))
		for _, sp := range spaces {
			allowed, err := d.accountService.CanAccessSpaceByID(d.ctx, username, sp.ID, account.PermissionRead)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}

			d.entries = append(d.entries, &virtualDirInfo{
				name:    sp.SpaceName,
				modTime: sp.CreatedAt,
			})
		}
	}

	if count <= 0 {
		entries := d.entries[d.pos:]
		d.pos = len(d.entries)
		return entries, nil
	}

	end := d.pos + count
	if end > len(d.entries) {
		end = len(d.entries)
	}
	entries := d.entries[d.pos:end]
	d.pos = end

	if d.pos >= len(d.entries) {
		return entries, fmt.Errorf("EOF")
	}
	return entries, nil
}

// --- Space 이름으로 표시되는 파일 래퍼 ---

type spaceNamedFile struct {
	*os.File
	name string
}

func (f *spaceNamedFile) Write([]byte) (int, error) {
	return 0, os.ErrPermission
}

func (f *spaceNamedFile) Stat() (fs.FileInfo, error) {
	fi, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &virtualDirInfo{name: f.name, modTime: fi.ModTime()}, nil
}

// --- 가상 디렉토리 FileInfo ---

type virtualDirInfo struct {
	name    string
	modTime time.Time
}

func (i *virtualDirInfo) Name() string       { return i.name }
func (i *virtualDirInfo) Size() int64        { return 0 }
func (i *virtualDirInfo) Mode() fs.FileMode  { return os.ModeDir | 0555 }
func (i *virtualDirInfo) ModTime() time.Time { return i.modTime }
func (i *virtualDirInfo) IsDir() bool        { return true }
func (i *virtualDirInfo) Sys() interface{}   { return nil }

func isPathWithinSpace(pathValue, spacePath string) bool {
	cleanPath := filepath.Clean(pathValue)
	cleanSpace := filepath.Clean(spacePath)

	rel, err := filepath.Rel(cleanSpace, cleanPath)
	if err != nil {
		return false
	}

	return rel == "." || !strings.HasPrefix(rel, "..")
}
