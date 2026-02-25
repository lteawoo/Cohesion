package system

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SelfUpdateStatus struct {
	State          string `json:"state"`
	Message        string `json:"message,omitempty"`
	CurrentVersion string `json:"currentVersion,omitempty"`
	TargetVersion  string `json:"targetVersion,omitempty"`
	ReleaseURL     string `json:"releaseUrl,omitempty"`
	StartedAt      string `json:"startedAt,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
	Error          string `json:"error,omitempty"`
}

type SelfUpdateManagerConfig struct {
	RepoOwner      string
	RepoName       string
	RequestTimeout time.Duration
	APIBaseURL     string
}

type SelfUpdateManager struct {
	repoOwner    string
	repoName     string
	apiBaseURL   string
	client       *http.Client
	shutdownChan chan struct{}

	mu      sync.Mutex
	running bool
	status  SelfUpdateStatus
}

var (
	ErrSelfUpdateUnsupportedBuild = errors.New("self-update unsupported build")
	ErrSelfUpdateAlreadyRunning   = errors.New("self-update already running")
)

type githubRelease struct {
	TagName string               `json:"tag_name"`
	HTMLURL string               `json:"html_url"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func NewSelfUpdateManager(cfg SelfUpdateManagerConfig, shutdownChan chan struct{}) *SelfUpdateManager {
	repoOwner := strings.TrimSpace(cfg.RepoOwner)
	if repoOwner == "" {
		repoOwner = defaultRepoOwner
	}

	repoName := strings.TrimSpace(cfg.RepoName)
	if repoName == "" {
		repoName = defaultRepoName
	}

	requestTimeout := cfg.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 30 * time.Second
	}

	return &SelfUpdateManager{
		repoOwner:  repoOwner,
		repoName:   repoName,
		apiBaseURL: strings.TrimSpace(cfg.APIBaseURL),
		client: &http.Client{
			Timeout: requestTimeout,
		},
		shutdownChan: shutdownChan,
		status: SelfUpdateStatus{
			State: "idle",
		},
	}
}

func (m *SelfUpdateManager) GetStatus() SelfUpdateStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *SelfUpdateManager) Start(currentVersion string, force bool) error {
	normalizedCurrent := normalizeVersionTag(currentVersion)
	if normalizedCurrent == "dev" {
		return ErrSelfUpdateUnsupportedBuild
	}

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return ErrSelfUpdateAlreadyRunning
	}
	m.running = true
	m.status = SelfUpdateStatus{
		State:          "checking",
		Message:        "업데이트 준비 중입니다",
		CurrentVersion: normalizedCurrent,
		StartedAt:      time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	m.mu.Unlock()

	go m.run(normalizedCurrent, force)
	return nil
}

func (m *SelfUpdateManager) run(currentVersion string, force bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := m.execute(ctx, currentVersion, force); err != nil {
		m.fail(err)
		return
	}
}

func (m *SelfUpdateManager) execute(ctx context.Context, currentVersion string, force bool) error {
	m.setStatus("checking", "최신 릴리즈를 확인하고 있습니다", "", "")

	release, err := m.fetchLatestRelease(ctx)
	if err != nil {
		return err
	}

	targetVersion := normalizeVersionTag(release.TagName)
	if !force && !isNewerVersion(currentVersion, targetVersion) {
		m.complete("idle", "이미 최신 버전입니다", targetVersion, release.HTMLURL)
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "cohesion-update-*")
	if err != nil {
		return err
	}
	cleanupTempDir := true
	defer func() {
		if cleanupTempDir {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	archiveName, archiveURL, err := pickArchiveAsset(release.Assets, release.TagName, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	checksumURL, ok := findAssetDownloadURL(release.Assets, "checksums.txt")
	if !ok {
		return errors.New("checksums.txt 자산을 찾을 수 없습니다")
	}

	archivePath := filepath.Join(tmpDir, archiveName)
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")

	m.setStatus("downloading", "릴리즈 파일을 다운로드하고 있습니다", targetVersion, release.HTMLURL)
	if err := m.downloadFile(ctx, archiveURL, archivePath); err != nil {
		return err
	}
	if err := m.downloadFile(ctx, checksumURL, checksumsPath); err != nil {
		return err
	}

	expectedChecksum, err := findChecksumForAsset(checksumsPath, archiveName)
	if err != nil {
		return err
	}
	if err := verifyFileSHA256(archivePath, expectedChecksum); err != nil {
		return err
	}

	m.setStatus("staging", "새 바이너리를 준비하고 있습니다", targetVersion, release.HTMLURL)
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	executablePath = filepath.Clean(executablePath)

	updaterPath := filepath.Join(filepath.Dir(executablePath), updaterBinaryName(runtime.GOOS))
	if _, err := os.Stat(updaterPath); err != nil {
		return fmt.Errorf("업데이터 바이너리를 찾을 수 없습니다: %s", updaterPath)
	}

	replacementPath := filepath.Join(tmpDir, appBinaryName(runtime.GOOS))
	if err := extractBinaryFromArchive(archivePath, appBinaryName(runtime.GOOS), replacementPath); err != nil {
		return err
	}
	if err := os.Chmod(replacementPath, 0o755); err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		workDir = filepath.Dir(executablePath)
	}

	argsPath := filepath.Join(tmpDir, "args.json")
	argsPayload, err := json.Marshal(os.Args[1:])
	if err != nil {
		return err
	}
	if err := os.WriteFile(argsPath, argsPayload, 0o600); err != nil {
		return err
	}

	m.setStatus("switching", "업데이터를 실행했습니다. 서버를 종료합니다", targetVersion, release.HTMLURL)
	cmd := exec.Command(
		updaterPath,
		"--pid", strconv.Itoa(os.Getpid()),
		"--target", executablePath,
		"--replacement", replacementPath,
		"--workdir", workDir,
		"--args-file", argsPath,
		"--cleanup-dir", tmpDir,
	)
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return err
	}

	cleanupTempDir = false
	m.setStatus("switching", "업데이트 적용을 위해 서버를 종료합니다", targetVersion, release.HTMLURL)
	if m.shutdownChan != nil {
		select {
		case m.shutdownChan <- struct{}{}:
		default:
		}
		return nil
	}

	// 종료 채널이 없으면 전환 상태를 마무리 처리한다.
	m.complete("idle", "업데이터 실행이 완료되었습니다", targetVersion, release.HTMLURL)
	return nil
}

func (m *SelfUpdateManager) setStatus(state, message, targetVersion, releaseURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State = state
	m.status.Message = message
	if targetVersion != "" {
		m.status.TargetVersion = targetVersion
	}
	if releaseURL != "" {
		m.status.ReleaseURL = releaseURL
	}
	m.status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (m *SelfUpdateManager) complete(state, message, targetVersion, releaseURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	m.status.State = state
	m.status.Message = message
	if targetVersion != "" {
		m.status.TargetVersion = targetVersion
	}
	if releaseURL != "" {
		m.status.ReleaseURL = releaseURL
	}
	m.status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	m.status.Error = ""
}

func (m *SelfUpdateManager) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	m.status.State = "failed"
	m.status.Message = "업데이트 적용에 실패했습니다"
	m.status.Error = err.Error()
	m.status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (m *SelfUpdateManager) fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", m.repoOwner, m.repoName)
	if m.apiBaseURL != "" {
		requestURL = strings.TrimRight(m.apiBaseURL, "/") + "/releases/latest"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cohesion-self-updater")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("latest release request failed with status %d", resp.StatusCode)
	}

	var payload githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.TagName) == "" {
		return nil, errors.New("latest release tag is empty")
	}
	return &payload, nil
}

func (m *SelfUpdateManager) downloadFile(ctx context.Context, url, destinationPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "cohesion-self-updater")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s (%d)", url, resp.StatusCode)
	}

	out, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}

	return nil
}

func pickArchiveAsset(assets []githubReleaseAsset, tag, goos, goarch string) (string, string, error) {
	candidates := buildArchiveNameCandidates(tag, goos, goarch)
	for _, candidate := range candidates {
		if downloadURL, ok := findAssetDownloadURL(assets, candidate); ok {
			return candidate, downloadURL, nil
		}
	}
	return "", "", fmt.Errorf("지원되는 아카이브 자산을 찾을 수 없습니다 (tag=%s, os=%s, arch=%s)", tag, goos, goarch)
}

func findAssetDownloadURL(assets []githubReleaseAsset, fileName string) (string, bool) {
	target := strings.TrimSpace(fileName)
	for _, asset := range assets {
		if strings.TrimSpace(asset.Name) == target {
			return strings.TrimSpace(asset.BrowserDownloadURL), true
		}
	}
	return "", false
}

func buildArchiveNameCandidates(tag, goos, goarch string) []string {
	version := strings.TrimSpace(tag)
	if version == "" {
		return nil
	}

	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}

	candidates := []string{
		fmt.Sprintf("cohesion_%s_%s_%s%s", version, goos, goarch, ext),
	}

	if strings.HasPrefix(version, "v") || strings.HasPrefix(version, "V") {
		withoutPrefix := strings.TrimSpace(version[1:])
		if withoutPrefix != "" {
			candidates = append(candidates, fmt.Sprintf("cohesion_%s_%s_%s%s", withoutPrefix, goos, goarch, ext))
		}
	} else {
		candidates = append(candidates, fmt.Sprintf("cohesion_v%s_%s_%s%s", version, goos, goarch, ext))
	}

	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		unique = append(unique, candidate)
	}

	return unique
}

func findChecksumForAsset(checksumFilePath, assetName string) (string, error) {
	file, err := os.Open(checksumFilePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		hashValue := strings.TrimSpace(fields[0])
		name := strings.TrimPrefix(strings.TrimSpace(fields[len(fields)-1]), "*")
		if name == assetName {
			if len(hashValue) != 64 {
				return "", fmt.Errorf("invalid checksum length for %s", assetName)
			}
			return strings.ToLower(hashValue), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("checksum not found for %s", assetName)
}

func verifyFileSHA256(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	expected := strings.ToLower(strings.TrimSpace(expectedChecksum))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected=%s actual=%s", expected, actual)
	}
	return nil
}

func extractBinaryFromArchive(archivePath, binaryName, outputPath string) error {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractBinaryFromZip(archivePath, binaryName, outputPath)
	}
	return extractBinaryFromTarGz(archivePath, binaryName, outputPath)
}

func extractBinaryFromTarGz(archivePath, binaryName, outputPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(header.Name) != binaryName {
			continue
		}

		return writeBinaryFromReader(tarReader, outputPath)
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func extractBinaryFromZip(archivePath, binaryName, outputPath string) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		if filepath.Base(file.Name) != binaryName {
			continue
		}

		reader, err := file.Open()
		if err != nil {
			return err
		}
		defer reader.Close()

		return writeBinaryFromReader(reader, outputPath)
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func writeBinaryFromReader(reader io.Reader, outputPath string) error {
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return err
	}

	return nil
}

func appBinaryName(goos string) string {
	if goos == "windows" {
		return "cohesion.exe"
	}
	return "cohesion"
}

func updaterBinaryName(goos string) string {
	if goos == "windows" {
		return "cohesion-updater.exe"
	}
	return "cohesion-updater"
}
