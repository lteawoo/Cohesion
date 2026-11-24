package space

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const defaultQuotaUsageCacheTTL = 10 * time.Second

type cachedSpaceUsage struct {
	usedBytes int64
	scannedAt time.Time
}

type quotaReservation struct {
	spaceID    int64
	deltaBytes int64
}

type UploadReservationStore interface {
	SaveUploadReservation(ctx context.Context, id string, spaceID, deltaBytes int64) error
	DeleteUploadReservation(ctx context.Context, id string) error
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

	mu               sync.RWMutex
	cache            map[int64]cachedSpaceUsage
	reservations     map[string]quotaReservation
	reservedBySpace  map[int64]int64
	reservationStore UploadReservationStore
	uploadStages     sync.Map
}

func (s *QuotaService) SetUploadReservationStore(store UploadReservationStore) {
	s.reservationStore = store
}

func (s *QuotaService) TrackUploadStage(path string) {
	s.uploadStages.Store(path, struct{}{})
}

func (s *QuotaService) ForgetUploadStage(path string) {
	s.uploadStages.Delete(path)
}

func NewQuotaService(spaceService *Service) *QuotaService {
	return &QuotaService{
		spaceService:    spaceService,
		ttl:             defaultQuotaUsageCacheTTL,
		cache:           make(map[int64]cachedSpaceUsage),
		reservations:    make(map[string]quotaReservation),
		reservedBySpace: make(map[int64]int64),
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

func (s *QuotaService) AcquireWriteReservation(ctx context.Context, spaceID int64, reservationID string, deltaBytes int64) (*SpaceUsage, error) {
	if deltaBytes <= 0 {
		return s.GetSpaceUsage(ctx, spaceID)
	}

	usage, err := s.GetSpaceUsage(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if usage.QuotaBytes == nil {
		return usage, nil
	}

	quota := *usage.QuotaBytes

	s.mu.Lock()
	defer s.mu.Unlock()

	reservedBytes := s.reservedBySpace[spaceID]
	existing, alreadyReserved := s.reservations[reservationID]
	if alreadyReserved && existing.spaceID == spaceID {
		reservedBytes -= existing.deltaBytes
	}
	projected := usage.UsedBytes + reservedBytes + deltaBytes
	if projected > quota {
		return nil, &QuotaExceededError{
			SpaceID:    usage.SpaceID,
			SpaceName:  usage.SpaceName,
			UsedBytes:  usage.UsedBytes + reservedBytes,
			QuotaBytes: quota,
			DeltaBytes: deltaBytes,
		}
	}

	if s.reservationStore != nil {
		if err := s.reservationStore.SaveUploadReservation(ctx, reservationID, spaceID, deltaBytes); err != nil {
			return nil, fmt.Errorf("persist upload quota reservation: %w", err)
		}
	}
	if alreadyReserved {
		s.releaseReservationLocked(reservationID, existing)
	}
	s.reservations[reservationID] = quotaReservation{
		spaceID:    spaceID,
		deltaBytes: deltaBytes,
	}
	s.reservedBySpace[spaceID] = reservedBytes + deltaBytes

	return usage, nil
}

func (s *QuotaService) ReleaseWriteReservation(reservationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	reservation, ok := s.reservations[reservationID]
	if !ok {
		return
	}
	if s.reservationStore != nil {
		if err := s.reservationStore.DeleteUploadReservation(context.Background(), reservationID); err != nil {
			log.Error().Err(err).Str("reservation_id", reservationID).Msg("failed to delete closed upload reservation")
		}
	}
	s.releaseReservationLocked(reservationID, reservation)
}

func (s *QuotaService) releaseReservationLocked(reservationID string, reservation quotaReservation) {
	delete(s.reservations, reservationID)

	nextReserved := s.reservedBySpace[reservation.spaceID] - reservation.deltaBytes
	if nextReserved > 0 {
		s.reservedBySpace[reservation.spaceID] = nextReserved
		return
	}
	delete(s.reservedBySpace, reservation.spaceID)
}

func (s *QuotaService) ReservedBytes(spaceID int64) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reservedBySpace[spaceID]
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
	root, err := filepath.Abs(spacePath)
	if err != nil {
		return 0, err
	}
	var usedBytes int64
	err = filepath.WalkDir(root, func(currentPath string, entry os.DirEntry, walkErr error) error {
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
		if _, staged := s.uploadStages.Load(currentPath); staged {
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
