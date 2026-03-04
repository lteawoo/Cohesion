package smb

import (
	"context"
	"errors"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/space"
	"taeu.kr/cohesion/pkg/smbcore"
)

type Guard struct {
	spaceService   *space.Service
	accountService *account.Service
	username       string
}

func NewGuard(spaceService *space.Service, accountService *account.Service, username string) *Guard {
	return &Guard{
		spaceService:   spaceService,
		accountService: accountService,
		username:       username,
	}
}

func (g *Guard) ResolvePath(virtualPath string, required account.Permission) (*space.Space, string, string, error) {
	cleanPath := NormalizeVirtualPath(virtualPath)
	spaceName, relPath, err := SplitVirtualPath(cleanPath)
	if err != nil {
		return nil, "", "", err
	}

	spaceObj, err := g.resolveSpaceByName(spaceName, required)
	if err != nil {
		return nil, "", "", err
	}

	absPath := spaceObj.SpacePath
	if relPath != "" {
		absPath = filepath.Join(spaceObj.SpacePath, filepath.FromSlash(relPath))
	}
	if !IsPathWithinSpace(absPath, spaceObj.SpacePath) {
		return nil, "", "", errors.Join(smbcore.ErrPathBoundary, os.ErrPermission)
	}

	return spaceObj, absPath, relPath, nil
}

func (g *Guard) ValidateRename(fromPath, toPath string) error {
	fromClean := NormalizeVirtualPath(fromPath)
	toClean := NormalizeVirtualPath(toPath)

	fromSpace, fromRel, err := SplitVirtualPath(fromClean)
	if err != nil {
		return err
	}
	toSpace, toRel, err := SplitVirtualPath(toClean)
	if err != nil {
		return err
	}

	if fromSpace != toSpace || fromRel == "" || toRel == "" {
		return errors.Join(smbcore.ErrPathBoundary, os.ErrPermission)
	}

	spaceObj, err := g.resolveSpaceByName(fromSpace, account.PermissionManage)
	if err != nil {
		return err
	}

	absFrom := filepath.Join(spaceObj.SpacePath, filepath.FromSlash(fromRel))
	absTo := filepath.Join(spaceObj.SpacePath, filepath.FromSlash(toRel))
	if !IsPathWithinSpace(absFrom, spaceObj.SpacePath) || !IsPathWithinSpace(absTo, spaceObj.SpacePath) {
		return errors.Join(smbcore.ErrPathBoundary, os.ErrPermission)
	}

	return nil
}

func (g *Guard) resolveSpaceByName(spaceName string, required account.Permission) (*space.Space, error) {
	spaceObj, err := g.spaceService.GetSpaceByName(context.Background(), spaceName)
	if err != nil {
		return nil, os.ErrNotExist
	}

	allowed, err := g.accountService.CanAccessSpaceByID(context.Background(), g.username, spaceObj.ID, required)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, errors.Join(smbcore.ErrPermissionDenied, os.ErrPermission)
	}

	return spaceObj, nil
}

func NormalizeVirtualPath(pathValue string) string {
	pathValue = strings.ReplaceAll(pathValue, "\\", "/")
	cleaned := pathpkg.Clean("/" + strings.TrimPrefix(pathValue, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func SplitVirtualPath(cleanPath string) (spaceName string, relativePath string, err error) {
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

func IsPathWithinSpace(pathValue, spacePath string) bool {
	cleanPath := filepath.Clean(pathValue)
	cleanSpace := filepath.Clean(spacePath)

	rel, err := filepath.Rel(cleanSpace, cleanPath)
	if err != nil {
		return false
	}

	return rel == "." || !strings.HasPrefix(rel, "..")
}
