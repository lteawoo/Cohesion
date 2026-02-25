package browse

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/disk"
)

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

type Service struct {
	initialBrowseRoot string     // 사용자의 홈 디렉토리 저장
	baseDirectories   []FileInfo // User home, disks list
}

func normalizeBrowsePath(pathValue string) string {
	return normalizeBrowsePathForOS(pathValue, runtime.GOOS)
}

func normalizeBrowsePathForOS(pathValue string, osName string) string {
	if pathValue == "" {
		return filepath.Clean(pathValue)
	}
	if osName == "windows" {
		return normalizeWindowsBrowsePath(pathValue)
	}
	return filepath.Clean(pathValue)
}

func normalizeWindowsBrowsePath(pathValue string) string {
	slashPath := strings.ReplaceAll(pathValue, `\`, "/")
	if slashPath == "" {
		return "."
	}

	// UNC 경로: //server/share[/...]
	if strings.HasPrefix(slashPath, "//") {
		return normalizeWindowsUNCPath(slashPath)
	}

	// 드라이브 절대/상대 경로: C:, C:\, C:/, C:foo, C:/foo
	if len(slashPath) >= 2 && slashPath[1] == ':' && isWindowsDriveLetter(slashPath[0]) {
		drive := strings.ToUpper(string(slashPath[0])) + ":"
		rest := slashPath[2:]

		if rest == "" || rest == "/" || rest == "/." {
			return drive + `\`
		}
		if !strings.HasPrefix(rest, "/") {
			// Windows의 drive-relative(C:foo)를 절대 루트 기준(C:\foo)으로 강제 정규화.
			rest = "/" + rest
		}
		cleanedRest := path.Clean(rest)
		if cleanedRest == "." || cleanedRest == "/" {
			return drive + `\`
		}
		return drive + `\` + strings.ReplaceAll(strings.TrimPrefix(cleanedRest, "/"), "/", `\`)
	}

	// 루트 상대(\foo)나 일반 경로는 구분자만 Windows 스타일로 통일.
	cleaned := path.Clean(strings.ReplaceAll(slashPath, `\`, "/"))
	if cleaned == "." {
		return "."
	}
	return strings.ReplaceAll(cleaned, "/", `\`)
}

func normalizeWindowsUNCPath(slashPath string) string {
	parts := strings.Split(strings.TrimPrefix(slashPath, "//"), "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		segments = append(segments, part)
	}

	if len(segments) == 0 {
		return `\\`
	}
	if len(segments) == 1 {
		return `\\` + segments[0]
	}

	server := segments[0]
	share := segments[1]
	stack := make([]string, 0, len(segments)-2)
	for _, segment := range segments[2:] {
		if segment == ".." {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		stack = append(stack, segment)
	}

	base := `\\` + server + `\` + share
	if len(stack) == 0 {
		return base
	}
	return base + `\` + strings.Join(stack, `\`)
}

func isWindowsDriveLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func mountpointDedupKey(pathValue string, osName string) string {
	if osName == "windows" {
		return strings.ToLower(pathValue)
	}
	return pathValue
}

func NewService() *Service {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		// 실패 시 애플리케이션 실행 폴더를 기준으로 함
		log.Warn().Err(err).Msg("Fail to get user home directory, using executable path instead.")
		executablePath, execErr := os.Executable()
		if execErr != nil {
			log.Fatal().Err(execErr).Msg("Fail to get executable path.")
		}
		userHomeDir = filepath.Dir(executablePath)
	} else {
		log.Info().Msgf("초기 디렉토리: %s", userHomeDir)
	}

	// 디스크 파티션 정보 가져오기
	partitions, err := disk.Partitions(false)
	if err != nil {
		log.Error().Err(err).Msg("Fail to get disk partitions.")
	}

	var baseDirectories []FileInfo
	baseDirectories = append(baseDirectories, FileInfo{
		Name:  "Home",
		Path:  userHomeDir,
		IsDir: true,
	})

	osName := runtime.GOOS
	seen := make(map[string]bool)
	for _, p := range partitions {
		mountpoint := normalizeBrowsePathForOS(p.Mountpoint, osName)
		seenKey := mountpointDedupKey(mountpoint, osName)
		if seen[seenKey] {
			continue
		}
		seen[seenKey] = true
		baseDirectories = append(baseDirectories, FileInfo{
			Name:  mountpoint,
			Path:  mountpoint,
			IsDir: true,
		})
	}

	return &Service{initialBrowseRoot: userHomeDir, baseDirectories: baseDirectories}
}

func (s *Service) GetInitialBrowseRoot() string {
	return s.initialBrowseRoot
}

func (s *Service) GetBaseDirectories() []FileInfo {
	return s.baseDirectories
}

func (s *Service) ListDirectory(isOnlyDir bool, path string) ([]FileInfo, error) {
	cleanPath := normalizeBrowsePath(path)

	// 디렉토리 읽기
	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("fail to read directory: %w", err)
	}

	var files []FileInfo

	for _, entry := range entries {
		// 숨김 파일/폴더는 제외
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(cleanPath, entry.Name())
		if isOnlyDir && !entry.IsDir() { // Directory 전용 모드인 경우 파일 무시
			continue
		}

		info, err := entry.Info()
		var size int64
		var modTime time.Time
		if err == nil {
			size = info.Size()
			modTime = info.ModTime()
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    fullPath,
			IsDir:   entry.IsDir(),
			Size:    size,
			ModTime: modTime,
		})
	}

	// 정렬
	sort.Slice(files, func(i, j int) bool {
		if files[i].Name == ".." {
			return true
		}
		if files[j].Name == ".." {
			return false
		}
		if files[i].IsDir && !files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	return files, nil
}
