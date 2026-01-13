package browse

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/disk"
)

type FileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type Service struct {
	initialBrowseRoot string     // 사용자의 홈 디렉토리 저장
	baseDirectories   []FileInfo // User home, disks list
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

	for _, p := range partitions {
		baseDirectories = append(baseDirectories, FileInfo{
			Name:  p.Mountpoint,
			Path:  p.Mountpoint,
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
	cleanPath := filepath.Clean(path)

	// 디렉토리 읽기
	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("fail to read directory: %w", err)
	}

	var files []FileInfo
	// parentDir := filepath.Dir(cleanPath)

	// if parentDir != cleanPath {
	// 	files = append(files, FileInfo{
	// 		Name:  "..",
	// 		Path:  parentDir,
	// 		IsDir: true,
	// 	})
	// }

	for _, entry := range entries {
		// 숨김 파일/폴더는 제외
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(cleanPath, entry.Name())
		if isOnlyDir && !entry.IsDir() { // Directory 전용 모드인 경우 파일 무시
			continue
		}
		files = append(files, FileInfo{
			Name:  entry.Name(),
			Path:  fullPath,
			IsDir: entry.IsDir(),
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
