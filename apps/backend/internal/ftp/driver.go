package ftp

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	goftp "github.com/goftp/server"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/space"
)

type driverFactory struct {
	spaceService   *space.Service
	accountService *account.Service
}

func (f *driverFactory) NewDriver() (goftp.Driver, error) {
	return &spaceDriver{
		spaceService:   f.spaceService,
		accountService: f.accountService,
		perm:           goftp.NewSimplePerm("cohesion", "cohesion"),
	}, nil
}

type spaceDriver struct {
	spaceService   *space.Service
	accountService *account.Service
	perm           goftp.Perm
	conn           *goftp.Conn
}

func (d *spaceDriver) Init(conn *goftp.Conn) {
	d.conn = conn
}

func (d *spaceDriver) Stat(virtualPath string) (goftp.FileInfo, error) {
	cleanPath := normalizeVirtualPath(virtualPath)
	if cleanPath == "/" {
		return newVirtualDirInfo("/", "cohesion", "cohesion", time.Now()), nil
	}

	spaceObj, absPath, relPath, err := d.resolvePath(cleanPath, account.PermissionRead)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	name := info.Name()
	if relPath == "" {
		name = spaceObj.SpaceName
	}
	return d.wrapFileInfo(cleanPath, info, name)
}

func (d *spaceDriver) ChangeDir(virtualPath string) error {
	fileInfo, err := d.Stat(virtualPath)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

func (d *spaceDriver) ListDir(virtualPath string, callback func(goftp.FileInfo) error) error {
	cleanPath := normalizeVirtualPath(virtualPath)
	if cleanPath == "/" {
		return d.listAccessibleSpaces(callback)
	}

	_, absPath, _, err := d.resolvePath(cleanPath, account.PermissionRead)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}
		entryVirtual := path.Join(cleanPath, entry.Name())
		wrapped, err := d.wrapFileInfo(entryVirtual, info, info.Name())
		if err != nil {
			return err
		}
		if err := callback(wrapped); err != nil {
			return err
		}
	}

	return nil
}

func (d *spaceDriver) DeleteDir(virtualPath string) error {
	cleanPath := normalizeVirtualPath(virtualPath)
	if cleanPath == "/" {
		return os.ErrPermission
	}

	_, absPath, relPath, err := d.resolvePath(cleanPath, account.PermissionWrite)
	if err != nil {
		return err
	}
	if relPath == "" {
		return os.ErrPermission
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("not a directory")
	}

	return os.Remove(absPath)
}

func (d *spaceDriver) DeleteFile(virtualPath string) error {
	cleanPath := normalizeVirtualPath(virtualPath)
	_, absPath, relPath, err := d.resolvePath(cleanPath, account.PermissionWrite)
	if err != nil {
		return err
	}
	if relPath == "" {
		return os.ErrPermission
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("not a file")
	}

	return os.Remove(absPath)
}

func (d *spaceDriver) Rename(fromPath string, toPath string) error {
	fromClean := normalizeVirtualPath(fromPath)
	toClean := normalizeVirtualPath(toPath)

	fromSpace, fromRel, err := splitVirtualPath(fromClean)
	if err != nil {
		return err
	}
	toSpace, toRel, err := splitVirtualPath(toClean)
	if err != nil {
		return err
	}
	if fromSpace != toSpace || fromRel == "" || toRel == "" {
		return os.ErrPermission
	}

	if _, err := d.resolveSpaceByName(fromSpace, account.PermissionWrite); err != nil {
		return err
	}
	spaceObj, err := d.resolveSpaceByName(toSpace, account.PermissionWrite)
	if err != nil {
		return err
	}

	absFrom := filepath.Join(spaceObj.SpacePath, filepath.FromSlash(fromRel))
	absTo := filepath.Join(spaceObj.SpacePath, filepath.FromSlash(toRel))
	if !isPathWithinSpace(absFrom, spaceObj.SpacePath) || !isPathWithinSpace(absTo, spaceObj.SpacePath) {
		return os.ErrPermission
	}

	return os.Rename(absFrom, absTo)
}

func (d *spaceDriver) MakeDir(virtualPath string) error {
	cleanPath := normalizeVirtualPath(virtualPath)
	spaceObj, absPath, relPath, err := d.resolvePath(cleanPath, account.PermissionWrite)
	if err != nil {
		return err
	}
	if relPath == "" || !isPathWithinSpace(absPath, spaceObj.SpacePath) {
		return os.ErrPermission
	}
	return os.MkdirAll(absPath, 0755)
}

func (d *spaceDriver) GetFile(virtualPath string, offset int64) (int64, io.ReadCloser, error) {
	cleanPath := normalizeVirtualPath(virtualPath)
	_, absPath, relPath, err := d.resolvePath(cleanPath, account.PermissionRead)
	if err != nil {
		return 0, nil, err
	}
	if relPath == "" {
		return 0, nil, os.ErrPermission
	}

	file, err := os.Open(absPath)
	if err != nil {
		return 0, nil, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return 0, nil, err
	}
	if info.IsDir() {
		file.Close()
		return 0, nil, errors.New("not a file")
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		file.Close()
		return 0, nil, err
	}

	return info.Size(), file, nil
}

func (d *spaceDriver) PutFile(virtualPath string, data io.Reader, appendData bool) (int64, error) {
	cleanPath := normalizeVirtualPath(virtualPath)
	spaceObj, absPath, relPath, err := d.resolvePath(cleanPath, account.PermissionWrite)
	if err != nil {
		return 0, err
	}
	if relPath == "" {
		return 0, os.ErrPermission
	}

	parent := filepath.Dir(absPath)
	if !isPathWithinSpace(parent, spaceObj.SpacePath) {
		return 0, os.ErrPermission
	}
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return 0, os.ErrNotExist
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendData {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(absPath, flags, 0644)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	written, err := io.Copy(file, data)
	if err != nil {
		return 0, err
	}
	return written, nil
}

func (d *spaceDriver) listAccessibleSpaces(callback func(goftp.FileInfo) error) error {
	spaces, err := d.spaceService.GetAllSpaces(context.Background())
	if err != nil {
		return err
	}

	username := d.username()
	for _, sp := range spaces {
		allowed, err := d.accountService.CanAccessSpaceByID(context.Background(), username, sp.ID, account.PermissionRead)
		if err != nil {
			return err
		}
		if !allowed {
			continue
		}

		mod := time.Now()
		if stat, err := os.Stat(sp.SpacePath); err == nil {
			mod = stat.ModTime()
		}
		if err := callback(newVirtualDirInfo(sp.SpaceName, "cohesion", "cohesion", mod)); err != nil {
			return err
		}
	}
	return nil
}

func (d *spaceDriver) resolvePath(cleanPath string, required account.Permission) (*space.Space, string, string, error) {
	spaceName, relPath, err := splitVirtualPath(cleanPath)
	if err != nil {
		return nil, "", "", err
	}

	spaceObj, err := d.resolveSpaceByName(spaceName, required)
	if err != nil {
		return nil, "", "", err
	}

	absPath := spaceObj.SpacePath
	if relPath != "" {
		absPath = filepath.Join(spaceObj.SpacePath, filepath.FromSlash(relPath))
	}
	if !isPathWithinSpace(absPath, spaceObj.SpacePath) {
		return nil, "", "", os.ErrPermission
	}
	return spaceObj, absPath, relPath, nil
}

func (d *spaceDriver) resolveSpaceByName(spaceName string, required account.Permission) (*space.Space, error) {
	spaceObj, err := d.spaceService.GetSpaceByName(context.Background(), spaceName)
	if err != nil {
		return nil, os.ErrNotExist
	}

	allowed, err := d.accountService.CanAccessSpaceByID(context.Background(), d.username(), spaceObj.ID, required)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, os.ErrPermission
	}

	return spaceObj, nil
}

func (d *spaceDriver) username() string {
	if d.conn == nil {
		return ""
	}
	return d.conn.LoginUser()
}

func (d *spaceDriver) wrapFileInfo(virtualPath string, info os.FileInfo, name string) (goftp.FileInfo, error) {
	mode, err := d.perm.GetMode(virtualPath)
	if err != nil {
		return nil, err
	}
	owner, err := d.perm.GetOwner(virtualPath)
	if err != nil {
		return nil, err
	}
	group, err := d.perm.GetGroup(virtualPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		mode |= os.ModeDir
	}

	return &virtualFileInfo{
		FileInfo: info,
		name:     name,
		mode:     mode,
		owner:    owner,
		group:    group,
	}, nil
}

func normalizeVirtualPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	cleaned := path.Clean("/" + strings.TrimPrefix(p, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func splitVirtualPath(cleanPath string) (spaceName string, relativePath string, err error) {
	if cleanPath == "/" {
		return "", "", os.ErrPermission
	}

	trimmed := strings.TrimPrefix(cleanPath, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", os.ErrInvalid
	}

	spaceName = parts[0]
	if len(parts) > 1 {
		relativePath = path.Clean(path.Join(parts[1:]...))
		if relativePath == "." {
			relativePath = ""
		}
	}
	return spaceName, relativePath, nil
}

func isPathWithinSpace(pathValue, spacePath string) bool {
	cleanPath := filepath.Clean(pathValue)
	cleanSpace := filepath.Clean(spacePath)

	rel, err := filepath.Rel(cleanSpace, cleanPath)
	if err != nil {
		return false
	}

	return rel == "." || !strings.HasPrefix(rel, "..")
}

type virtualFileInfo struct {
	os.FileInfo
	name  string
	mode  os.FileMode
	owner string
	group string
}

func (f *virtualFileInfo) Name() string {
	return f.name
}

func (f *virtualFileInfo) Mode() os.FileMode {
	return f.mode
}

func (f *virtualFileInfo) Owner() string {
	return f.owner
}

func (f *virtualFileInfo) Group() string {
	return f.group
}

type staticDirInfo struct {
	name    string
	modTime time.Time
}

func (i *staticDirInfo) Name() string       { return i.name }
func (i *staticDirInfo) Size() int64        { return 0 }
func (i *staticDirInfo) Mode() os.FileMode  { return os.ModeDir | 0755 }
func (i *staticDirInfo) ModTime() time.Time { return i.modTime }
func (i *staticDirInfo) IsDir() bool        { return true }
func (i *staticDirInfo) Sys() interface{}   { return nil }

func newVirtualDirInfo(name, owner, group string, mod time.Time) goftp.FileInfo {
	return &virtualFileInfo{
		FileInfo: &staticDirInfo{name: name, modTime: mod},
		name:     name,
		mode:     os.ModeDir | 0755,
		owner:    owner,
		group:    group,
	}
}
