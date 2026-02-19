package sftp

import (
	"context"
	"errors"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	pkgsftp "github.com/pkg/sftp"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/space"
)

type spaceHandlers struct {
	spaceService   *space.Service
	accountService *account.Service
	username       string
}

func newSpaceHandlers(spaceService *space.Service, accountService *account.Service, username string) *spaceHandlers {
	return &spaceHandlers{
		spaceService:   spaceService,
		accountService: accountService,
		username:       username,
	}
}

func (h *spaceHandlers) Fileread(req *pkgsftp.Request) (io.ReaderAt, error) {
	cleanPath := normalizeVirtualPath(req.Filepath)
	_, absPath, relPath, err := h.resolvePath(cleanPath, account.PermissionRead)
	if err != nil {
		return nil, err
	}
	if relPath == "" {
		return nil, os.ErrPermission
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, errors.New("not a file")
	}

	return file, nil
}

func (h *spaceHandlers) Filewrite(req *pkgsftp.Request) (io.WriterAt, error) {
	cleanPath := normalizeVirtualPath(req.Filepath)
	spaceObj, absPath, relPath, err := h.resolvePath(cleanPath, account.PermissionWrite)
	if err != nil {
		return nil, err
	}
	if relPath == "" {
		return nil, os.ErrPermission
	}

	parent := filepath.Dir(absPath)
	if !isPathWithinSpace(parent, spaceObj.SpacePath) {
		return nil, os.ErrPermission
	}
	info, err := os.Stat(parent)
	if err != nil {
		return nil, os.ErrNotExist
	}
	if !info.IsDir() {
		return nil, os.ErrInvalid
	}

	openFlags := req.Pflags()
	flags := os.O_WRONLY
	if openFlags.Read && openFlags.Write {
		flags = os.O_RDWR
	}
	if openFlags.Creat {
		flags |= os.O_CREATE
	}
	if openFlags.Trunc {
		flags |= os.O_TRUNC
	}
	if openFlags.Excl {
		flags |= os.O_EXCL
	}

	file, err := os.OpenFile(absPath, flags, 0644)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (h *spaceHandlers) Filecmd(req *pkgsftp.Request) error {
	switch req.Method {
	case "Setstat":
		// Ignore chmod/chown/timestamp changes for now.
		return nil
	case "Rename":
		return h.rename(req.Filepath, req.Target)
	case "Rmdir":
		return h.deleteDir(req.Filepath)
	case "Mkdir":
		return h.makeDir(req.Filepath)
	case "Remove":
		return h.deleteFile(req.Filepath)
	case "Link", "Symlink":
		return os.ErrPermission
	default:
		return os.ErrInvalid
	}
}

func (h *spaceHandlers) Filelist(req *pkgsftp.Request) (pkgsftp.ListerAt, error) {
	cleanPath := normalizeVirtualPath(req.Filepath)

	switch req.Method {
	case "List":
		if cleanPath == "/" {
			entries, err := h.listAccessibleSpaces()
			if err != nil {
				return nil, err
			}
			return &fileInfoLister{entries: entries}, nil
		}

		_, absPath, _, err := h.resolvePath(cleanPath, account.PermissionRead)
		if err != nil {
			return nil, err
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, errors.New("not a directory")
		}

		dirEntries, err := os.ReadDir(absPath)
		if err != nil {
			return nil, err
		}

		fileInfos := make([]os.FileInfo, 0, len(dirEntries))
		for _, dirEntry := range dirEntries {
			entryInfo, err := dirEntry.Info()
			if err != nil {
				return nil, err
			}
			fileInfos = append(fileInfos, entryInfo)
		}
		return &fileInfoLister{entries: fileInfos}, nil

	case "Stat":
		if cleanPath == "/" {
			return &fileInfoLister{entries: []os.FileInfo{newVirtualDirInfo("/", time.Now())}}, nil
		}

		spaceObj, absPath, relPath, err := h.resolvePath(cleanPath, account.PermissionRead)
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

		return &fileInfoLister{entries: []os.FileInfo{&namedFileInfo{
			FileInfo: info,
			name:     name,
		}}}, nil

	case "Readlink":
		return nil, os.ErrPermission
	default:
		return nil, os.ErrInvalid
	}
}

func (h *spaceHandlers) rename(fromPath, toPath string) error {
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

	spaceObj, err := h.resolveSpaceByName(fromSpace, account.PermissionWrite)
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

func (h *spaceHandlers) deleteDir(virtualPath string) error {
	cleanPath := normalizeVirtualPath(virtualPath)
	if cleanPath == "/" {
		return os.ErrPermission
	}

	_, absPath, relPath, err := h.resolvePath(cleanPath, account.PermissionWrite)
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

func (h *spaceHandlers) deleteFile(virtualPath string) error {
	cleanPath := normalizeVirtualPath(virtualPath)
	_, absPath, relPath, err := h.resolvePath(cleanPath, account.PermissionWrite)
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

func (h *spaceHandlers) makeDir(virtualPath string) error {
	cleanPath := normalizeVirtualPath(virtualPath)
	spaceObj, absPath, relPath, err := h.resolvePath(cleanPath, account.PermissionWrite)
	if err != nil {
		return err
	}
	if relPath == "" || !isPathWithinSpace(absPath, spaceObj.SpacePath) {
		return os.ErrPermission
	}
	return os.MkdirAll(absPath, 0755)
}

func (h *spaceHandlers) listAccessibleSpaces() ([]os.FileInfo, error) {
	spaces, err := h.spaceService.GetAllSpaces(context.Background())
	if err != nil {
		return nil, err
	}

	entries := make([]os.FileInfo, 0, len(spaces))
	for _, sp := range spaces {
		allowed, err := h.accountService.CanAccessSpaceByID(context.Background(), h.username, sp.ID, account.PermissionRead)
		if err != nil {
			return nil, err
		}
		if !allowed {
			continue
		}

		modTime := time.Now()
		if stat, err := os.Stat(sp.SpacePath); err == nil {
			modTime = stat.ModTime()
		}
		entries = append(entries, newVirtualDirInfo(sp.SpaceName, modTime))
	}

	return entries, nil
}

func (h *spaceHandlers) resolvePath(cleanPath string, required account.Permission) (*space.Space, string, string, error) {
	spaceName, relPath, err := splitVirtualPath(cleanPath)
	if err != nil {
		return nil, "", "", err
	}

	spaceObj, err := h.resolveSpaceByName(spaceName, required)
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

func (h *spaceHandlers) resolveSpaceByName(spaceName string, required account.Permission) (*space.Space, error) {
	spaceObj, err := h.spaceService.GetSpaceByName(context.Background(), spaceName)
	if err != nil {
		return nil, os.ErrNotExist
	}

	allowed, err := h.accountService.CanAccessSpaceByID(context.Background(), h.username, spaceObj.ID, required)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, os.ErrPermission
	}

	return spaceObj, nil
}

func normalizeVirtualPath(pathValue string) string {
	pathValue = strings.ReplaceAll(pathValue, "\\", "/")
	cleaned := pathpkg.Clean("/" + strings.TrimPrefix(pathValue, "/"))
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
		relativePath = pathpkg.Clean(pathpkg.Join(parts[1:]...))
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

type fileInfoLister struct {
	entries []os.FileInfo
}

func (l *fileInfoLister) ListAt(target []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l.entries)) {
		return 0, io.EOF
	}

	n := copy(target, l.entries[offset:])
	if int(offset)+n >= len(l.entries) {
		return n, io.EOF
	}

	return n, nil
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

type namedFileInfo struct {
	os.FileInfo
	name string
}

func (n *namedFileInfo) Name() string {
	return n.name
}

func newVirtualDirInfo(name string, modTime time.Time) os.FileInfo {
	return &staticDirInfo{name: name, modTime: modTime}
}
