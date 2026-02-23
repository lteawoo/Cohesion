package space

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const defaultQuotaUsageCacheTTL = 10 * time.Second

type cachedSpaceUsage struct {
	usedBytes int64
	scannedAt time.Time
}

type SpaceUsage struct {
	SpaceID    int64     `json:"spaceId"`
	SpaceName  string    `json:"spaceName"`
	UsedBytes  int64     `json:"usedBytes"`
	QuotaBytes *int64    `json:"quotaBytes,omitempty"`
	OverQuota  bool      `json:"overQuota"`
	ScannedAt  time.Time `json:"scannedAt"`
}

type QuotaExceededError struct {
	SpaceID    int64
	SpaceName  string
	UsedBytes  int64
	QuotaBytes int64
	DeltaBytes int64
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("space quota exceeded (spaceId=%d, used=%d, quota=%d, delta=%d)", e.SpaceID, e.UsedBytes, e.QuotaBytes, e.DeltaBytes)
}

type QuotaService struct {
	spaceService *Service
	ttl          time.Duration

	mu    sync.RWMutex
	cache map[int64]cachedSpaceUsage
}

func NewQuotaService(spaceService *Service) *QuotaService {
	return &QuotaService{
		spaceService: spaceService,
		ttl:          defaultQuotaUsageCacheTTL,
		cache:        make(map[int64]cachedSpaceUsage),
	}
}

func (s *QuotaService) GetSpaceUsage(ctx context.Context, spaceID int64) (*SpaceUsage, error) {
	spaceData, err := s.spaceService.GetSpaceByID(ctx, spaceID)
	if err != nil {
		return nil, err
	}

	usedBytes, scannedAt, err := s.getUsedBytes(ctx, spaceData.ID, spaceData.SpacePath)
	if err != nil {
		return nil, err
	}

	overQuota := false
	if spaceData.QuotaBytes != nil {
		overQuota = usedBytes > *spaceData.QuotaBytes
	}

	usage := &SpaceUsage{
		SpaceID:    spaceData.ID,
		SpaceName:  spaceData.SpaceName,
		UsedBytes:  usedBytes,
		QuotaBytes: spaceData.QuotaBytes,
		OverQuota:  overQuota,
		ScannedAt:  scannedAt,
	}

	return usage, nil
}

func (s *QuotaService) EnsureCanWrite(ctx context.Context, spaceID int64, deltaBytes int64) error {
	usage, err := s.GetSpaceUsage(ctx, spaceID)
	if err != nil {
		return err
	}
	if usage.QuotaBytes == nil {
		return nil
	}
	if deltaBytes < 0 {
		return nil
	}

	quota := *usage.QuotaBytes
	projected := usage.UsedBytes + deltaBytes
	if projected <= quota {
		return nil
	}

	return &QuotaExceededError{
		SpaceID:    usage.SpaceID,
		SpaceName:  usage.SpaceName,
		UsedBytes:  usage.UsedBytes,
		QuotaBytes: quota,
		DeltaBytes: deltaBytes,
	}
}

func (s *QuotaService) CalculatePathSize(ctx context.Context, absPath string) (int64, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}

	var total int64
	err = filepath.WalkDir(absPath, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry == nil || entry.IsDir() {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		total += entryInfo.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}

	return total, nil
}

func (s *QuotaService) Invalidate(spaceID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, spaceID)
}

func (s *QuotaService) InvalidateMany(spaceIDs ...int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, spaceID := range spaceIDs {
		delete(s.cache, spaceID)
	}
}

func (s *QuotaService) getUsedBytes(ctx context.Context, spaceID int64, spacePath string) (int64, time.Time, error) {
	now := time.Now()

	s.mu.RLock()
	cached, ok := s.cache[spaceID]
	s.mu.RUnlock()
	if ok && now.Sub(cached.scannedAt) <= s.ttl {
		return cached.usedBytes, cached.scannedAt, nil
	}

	usedBytes, err := s.scanSpaceUsage(ctx, spacePath)
	if err != nil {
		return 0, time.Time{}, err
	}
	scannedAt := time.Now()

	s.mu.Lock()
	s.cache[spaceID] = cachedSpaceUsage{
		usedBytes: usedBytes,
		scannedAt: scannedAt,
	}
	s.mu.Unlock()

	return usedBytes, scannedAt, nil
}

func (s *QuotaService) scanSpaceUsage(ctx context.Context, spacePath string) (int64, error) {
	var usedBytes int64
	err := filepath.WalkDir(spacePath, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsPermission(walkErr) {
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return walkErr
		}
		if entry == nil || entry.IsDir() {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		info, err := entry.Info()
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		usedBytes += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}

	return usedBytes, nil
}
