package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const lifecycleStatusPathEnv = "COHESION_SYSTEM_STATUS_PATH"

var ErrRestartAlreadyInProgress = errors.New("restart already in progress")

type RestartRequest struct {
	Actor     string `json:"actor,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Port      string `json:"port,omitempty"`
}

type persistedLifecycleStatus struct {
	Status         SelfUpdateStatus `json:"status"`
	PendingRestart *RestartRequest  `json:"pendingRestart,omitempty"`
}

type StatusStore struct {
	mu      sync.Mutex
	path    string
	pathErr error
}

func NewStatusStore() *StatusStore {
	path, err := resolveLifecycleStatusPath()
	return &StatusStore{
		path:    path,
		pathErr: err,
	}
}

func (s *StatusStore) Load() (SelfUpdateStatus, error) {
	snapshot, err := s.LoadSnapshot()
	return snapshot.Status, err
}

func (s *StatusStore) LoadSnapshot() (persistedLifecycleStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadSnapshotLocked()
}

func (s *StatusStore) Save(status SelfUpdateStatus) error {
	_, err := s.Update(func(snapshot *persistedLifecycleStatus) error {
		snapshot.Status = normalizeLifecycleStatus(status)
		return nil
	})
	return err
}

func (s *StatusStore) Update(apply func(snapshot *persistedLifecycleStatus) error) (SelfUpdateStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadSnapshotLocked()
	if err != nil {
		if s.pathErr != nil {
			return defaultLifecycleStatus(), err
		}
		snapshot = persistedLifecycleStatus{Status: defaultLifecycleStatus()}
	}
	if err := apply(&snapshot); err != nil {
		return snapshot.Status, err
	}
	snapshot.Status = normalizeLifecycleStatus(snapshot.Status)
	if err := s.writeSnapshotLocked(snapshot); err != nil {
		return snapshot.Status, err
	}
	return snapshot.Status, nil
}

func (s *StatusStore) MarkRestartAccepted(req RestartRequest, currentVersion string) (SelfUpdateStatus, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.Update(func(snapshot *persistedLifecycleStatus) error {
		if snapshot.Status.Operation == "restart" && snapshot.Status.State == "restarting" {
			return ErrRestartAlreadyInProgress
		}

		status := snapshot.Status
		previousOperation := status.Operation
		status.Operation = "restart"
		status.State = "restarting"
		status.Message = "재시작 요청이 접수되었습니다"
		status.Error = ""
		status.TargetVersion = ""
		status.ReleaseURL = ""
		if strings.TrimSpace(status.StartedAt) == "" || previousOperation != "restart" {
			status.StartedAt = now
		}
		status.UpdatedAt = now
		status.RuntimeState = "restarting"
		status.RuntimeMessage = "서버 재시작을 준비하고 있습니다"
		if normalizedVersion := normalizeVersionTag(currentVersion); normalizedVersion != "" {
			status.CurrentVersion = normalizedVersion
		}

		snapshot.Status = status
		snapshot.PendingRestart = &RestartRequest{
			Actor:     strings.TrimSpace(req.Actor),
			RequestID: strings.TrimSpace(req.RequestID),
			Port:      strings.TrimSpace(req.Port),
		}
		return nil
	})
}

func (s *StatusStore) MarkRestartFailed(err error) (SelfUpdateStatus, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.Update(func(snapshot *persistedLifecycleStatus) error {
		status := snapshot.Status
		status.Operation = "restart"
		status.State = "failed"
		status.Message = "서버 재시작에 실패했습니다"
		status.Error = strings.TrimSpace(err.Error())
		if strings.TrimSpace(status.StartedAt) == "" {
			status.StartedAt = now
		}
		status.UpdatedAt = now
		status.RuntimeState = "failed"
		status.RuntimeMessage = "재시작 중 오류가 발생했습니다"
		snapshot.Status = status
		return nil
	})
}

func (s *StatusStore) MarkServerReady(currentVersion string) (SelfUpdateStatus, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.Update(func(snapshot *persistedLifecycleStatus) error {
		status := snapshot.Status
		status.RuntimeState = "healthy"
		status.RuntimeMessage = "서버가 정상 동작 중입니다"
		if normalizedVersion := normalizeVersionTag(currentVersion); normalizedVersion != "" {
			status.CurrentVersion = normalizedVersion
		}

		if status.Operation == "restart" {
			switch status.State {
			case "restarting":
				status.State = "succeeded"
				status.Message = "서버 재시작이 완료되었습니다"
				status.Error = ""
				status.UpdatedAt = now
			case "failed":
				if strings.TrimSpace(status.UpdatedAt) == "" {
					status.UpdatedAt = now
				}
			}
			snapshot.PendingRestart = nil
		}
		if status.Operation == "update" && strings.TrimSpace(status.UpdatedAt) == "" {
			status.UpdatedAt = now
		}

		snapshot.Status = status
		return nil
	})
}

func (s *StatusStore) MarkUpdateVerifying(targetVersion string) (SelfUpdateStatus, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.Update(func(snapshot *persistedLifecycleStatus) error {
		status := snapshot.Status
		status.Operation = "update"
		status.State = "verifying"
		status.Message = "새 버전 기동을 확인하고 있습니다"
		status.Error = ""
		status.TargetVersion = normalizeVersionTag(targetVersion)
		if strings.TrimSpace(status.StartedAt) == "" {
			status.StartedAt = now
		}
		status.UpdatedAt = now
		status.RuntimeState = "starting"
		status.RuntimeMessage = "새 버전 시작을 기다리고 있습니다"
		snapshot.Status = status
		return nil
	})
}

func (s *StatusStore) MarkUpdateRollingBack(err error) (SelfUpdateStatus, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.Update(func(snapshot *persistedLifecycleStatus) error {
		status := snapshot.Status
		status.Operation = "update"
		status.State = "rolling_back"
		status.Message = "이전 버전으로 롤백하고 있습니다"
		status.Error = strings.TrimSpace(err.Error())
		if strings.TrimSpace(status.StartedAt) == "" {
			status.StartedAt = now
		}
		status.UpdatedAt = now
		status.RuntimeState = "starting"
		status.RuntimeMessage = "이전 버전 복구를 기다리고 있습니다"
		snapshot.Status = status
		return nil
	})
}

func (s *StatusStore) MarkUpdateSucceeded(targetVersion string) (SelfUpdateStatus, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.Update(func(snapshot *persistedLifecycleStatus) error {
		status := snapshot.Status
		status.Operation = "update"
		status.State = "succeeded"
		status.Message = "업데이트가 완료되었습니다"
		status.Error = ""
		normalizedTarget := normalizeVersionTag(targetVersion)
		status.CurrentVersion = normalizedTarget
		status.TargetVersion = normalizedTarget
		if strings.TrimSpace(status.StartedAt) == "" {
			status.StartedAt = now
		}
		status.UpdatedAt = now
		status.RuntimeState = "healthy"
		status.RuntimeMessage = "서버가 정상 동작 중입니다"
		snapshot.Status = status
		return nil
	})
}

func (s *StatusStore) MarkUpdateRolledBack(err error) (SelfUpdateStatus, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.Update(func(snapshot *persistedLifecycleStatus) error {
		status := snapshot.Status
		status.Operation = "update"
		status.State = "failed"
		status.Message = "새 버전 검증에 실패해 이전 버전으로 롤백했습니다"
		status.Error = strings.TrimSpace(err.Error())
		if strings.TrimSpace(status.StartedAt) == "" {
			status.StartedAt = now
		}
		status.UpdatedAt = now
		status.RuntimeState = "healthy"
		status.RuntimeMessage = "서버가 정상 동작 중입니다"
		snapshot.Status = status
		return nil
	})
}

func (s *StatusStore) MarkUpdateFailed(err error) (SelfUpdateStatus, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.Update(func(snapshot *persistedLifecycleStatus) error {
		status := snapshot.Status
		status.Operation = "update"
		status.State = "failed"
		status.Message = "업데이트 전환에 실패했습니다"
		status.Error = strings.TrimSpace(err.Error())
		if strings.TrimSpace(status.StartedAt) == "" {
			status.StartedAt = now
		}
		status.UpdatedAt = now
		status.RuntimeState = "failed"
		status.RuntimeMessage = "업데이트 전환 중 오류가 발생했습니다"
		snapshot.Status = status
		return nil
	})
}

func resolveLifecycleStatusPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(lifecycleStatusPathEnv)); override != "" {
		return filepath.Clean(override), nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve lifecycle status path: %w", err)
	}
	executablePath = filepath.Clean(executablePath)
	if resolvedPath, err := filepath.EvalSymlinks(executablePath); err == nil && strings.TrimSpace(resolvedPath) != "" {
		executablePath = resolvedPath
	}
	return filepath.Join(filepath.Dir(executablePath), "data", "system-status.json"), nil
}

func defaultLifecycleStatus() SelfUpdateStatus {
	return SelfUpdateStatus{
		State:          "idle",
		RuntimeState:   "healthy",
		RuntimeMessage: "서버가 정상 동작 중입니다",
	}
}

func normalizeLifecycleStatus(status SelfUpdateStatus) SelfUpdateStatus {
	if strings.TrimSpace(status.State) == "" {
		status.State = "idle"
	}
	if strings.TrimSpace(status.RuntimeState) == "" {
		status.RuntimeState = "healthy"
	}
	if strings.TrimSpace(status.RuntimeMessage) == "" && status.RuntimeState == "healthy" {
		status.RuntimeMessage = "서버가 정상 동작 중입니다"
	}
	return status
}

func (s *StatusStore) loadSnapshotLocked() (persistedLifecycleStatus, error) {
	if s.pathErr != nil {
		return persistedLifecycleStatus{Status: defaultLifecycleStatus()}, s.pathErr
	}
	if strings.TrimSpace(s.path) == "" {
		return persistedLifecycleStatus{Status: defaultLifecycleStatus()}, errors.New("lifecycle status path is empty")
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return persistedLifecycleStatus{Status: defaultLifecycleStatus()}, nil
		}
		return persistedLifecycleStatus{Status: defaultLifecycleStatus()}, err
	}
	if len(data) == 0 {
		return persistedLifecycleStatus{Status: defaultLifecycleStatus()}, nil
	}

	var snapshot persistedLifecycleStatus
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return persistedLifecycleStatus{Status: defaultLifecycleStatus()}, err
	}
	snapshot.Status = normalizeLifecycleStatus(snapshot.Status)
	return snapshot, nil
}

func (s *StatusStore) writeSnapshotLocked(snapshot persistedLifecycleStatus) error {
	if s.pathErr != nil {
		return s.pathErr
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	tempPath := s.path + ".tmp"
	if err := os.WriteFile(tempPath, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, s.path)
}
