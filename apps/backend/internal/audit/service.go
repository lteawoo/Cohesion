package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/platform/logging"
)

type Storer interface {
	Create(ctx context.Context, event *Event) error
	List(ctx context.Context, filter ListFilter) ([]*Log, int64, error)
	StreamAll(ctx context.Context, filter ListFilter, yield func(*Log) error) error
	GetByID(ctx context.Context, id int64) (*Log, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type Config struct {
	BufferSize int
}

type Service struct {
	store Storer

	buffer chan Event
	once   sync.Once
	wg     sync.WaitGroup
}

func NewService(store Storer, cfg Config) *Service {
	bufferSize := cfg.BufferSize
	if bufferSize <= 0 {
		bufferSize = 256
	}

	svc := &Service{
		store:  store,
		buffer: make(chan Event, bufferSize),
	}
	svc.wg.Add(1)
	go svc.runWriter()
	return svc
}

func (s *Service) RecordBestEffort(event Event) {
	event = normalizeEvent(event)

	select {
	case s.buffer <- event:
	default:
		logging.Event(log.Warn(), logging.ComponentAudit, "warn.audit.dropped").
			Str("action", event.Action).
			Str("actor", event.Actor).
			Msg("audit event dropped due to full buffer")
	}
}

func (s *Service) List(ctx context.Context, filter ListFilter) (*ListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	items, total, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &ListResult{
		Items:    items,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*Log, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid audit log id: %d", id)
	}
	return s.store.GetByID(ctx, id)
}

func (s *Service) Export(ctx context.Context, filter ListFilter, yield func(*Log) error) error {
	return s.store.StreamAll(ctx, filter, yield)
}

func (s *Service) CleanupOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	if cutoff.IsZero() {
		return 0, fmt.Errorf("invalid cleanup cutoff")
	}
	return s.store.DeleteOlderThan(ctx, cutoff.UTC())
}

func (s *Service) Close(ctx context.Context) error {
	s.once.Do(func() {
		close(s.buffer)
	})

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) runWriter() {
	defer s.wg.Done()

	for event := range s.buffer {
		eventCopy := event
		if err := s.store.Create(context.Background(), &eventCopy); err != nil {
			logging.Event(log.Warn(), logging.ComponentAudit, "warn.audit.persist_failed").
				Err(err).
				Str("action", eventCopy.Action).
				Str("actor", eventCopy.Actor).
				Msg("failed to persist audit event")
		}
	}
}

func normalizeEvent(event Event) Event {
	now := time.Now().UTC()
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	} else {
		event.OccurredAt = event.OccurredAt.UTC()
	}

	event.Actor = strings.TrimSpace(event.Actor)
	if event.Actor == "" {
		event.Actor = "unknown"
	}

	event.Action = strings.TrimSpace(event.Action)
	if event.Action == "" {
		event.Action = "unknown"
	}

	if !IsValidResult(event.Result) {
		event.Result = ResultFailure
	}

	event.Target = sanitizePathLikeString(strings.TrimSpace(event.Target))
	if event.Target == "" {
		event.Target = "-"
	}

	event.RequestID = strings.TrimSpace(event.RequestID)
	if event.RequestID == "" {
		event.RequestID = generateRequestID()
	}

	event.Metadata = sanitizeMetadata(event.Action, event.Metadata)
	return event
}

func generateRequestID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return "req_" + hex.EncodeToString(buf)
}

var sensitiveKeyTokens = []string{
	"password",
	"token",
	"authorization",
	"cookie",
	"secret",
}

var metadataAllowlistByAction = map[string]map[string]struct{}{
	"file.upload": {
		"filename":       {},
		"size":           {},
		"status":         {},
		"conflictPolicy": {},
	},
	"file.rename": {
		"path":    {},
		"newName": {},
	},
	"file.delete": {
		"path":        {},
		"trashItemId": {},
	},
	"file.delete-multiple": {
		"total":     {},
		"succeeded": {},
		"failed":    {},
	},
	"file.move": {
		"sourceCount": {},
		"succeeded":   {},
		"failed":      {},
		"skipped":     {},
		"fromSpaceId": {},
		"toSpaceId":   {},
	},
	"file.copy": {
		"sourceCount": {},
		"succeeded":   {},
		"failed":      {},
		"skipped":     {},
		"fromSpaceId": {},
		"toSpaceId":   {},
	},
	"file.mkdir": {
		"path": {},
		"name": {},
	},
	"file.download": {
		"path":        {},
		"filename":    {},
		"size":        {},
		"format":      {},
		"sourceCount": {},
		"status":      {},
	},
	"file.download-ticket": {
		"path":     {},
		"filename": {},
		"size":     {},
		"format":   {},
		"status":   {},
	},
	"file.download-multiple": {
		"sourceCount": {},
		"filename":    {},
		"format":      {},
		"status":      {},
	},
	"file.download-multiple-ticket": {
		"sourceCount": {},
		"filename":    {},
		"size":        {},
		"format":      {},
		"status":      {},
	},
	"account.create": {
		"userId":        {},
		"username":      {},
		"role":          {},
		"changedFields": {},
	},
	"account.update": {
		"userId":        {},
		"username":      {},
		"role":          {},
		"changedFields": {},
	},
	"account.delete": {
		"userId":   {},
		"username": {},
	},
	"account.permissions.replace": {
		"userId":      {},
		"count":       {},
		"spaceIds":    {},
		"permissions": {},
	},
	"role.create": {
		"name": {},
	},
	"role.delete": {
		"name": {},
	},
	"role.permissions.replace": {
		"name":        {},
		"permissions": {},
		"count":       {},
	},
	"config.update": {
		"before": {},
		"after":  {},
	},
	"system.restart": {
		"port": {},
	},
	"system.update.start": {
		"force": {},
	},
	"audit.logs.cleanup": {
		"retentionDays": {},
		"deletedCount":  {},
		"cutoff":        {},
	},
}

var commonMetadataAllowlist = map[string]struct{}{
	"reason":        {},
	"code":          {},
	"status":        {},
	"changedFields": {},
}

func sanitizeMetadata(action string, metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}

	allowlist := metadataAllowlistByAction[action]

	sanitized := make(map[string]any)
	for key, value := range metadata {
		if _, ok := allowlist[key]; !ok {
			if _, common := commonMetadataAllowlist[key]; !common {
				continue
			}
		}
		if isSensitiveKey(key) {
			continue
		}
		safeValue, ok := sanitizeValue(value)
		if !ok {
			continue
		}
		sanitized[key] = safeValue
	}

	return sanitized
}

func sanitizeValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", true
		}
		return sanitizePathLikeString(trimmed), true
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed, true
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizePathLikeString(strings.TrimSpace(item)))
		}
		return out, true
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			safeItem, ok := sanitizeValue(item)
			if ok {
				out = append(out, safeItem)
			}
		}
		return out, true
	case map[string]any:
		out := make(map[string]any)
		for key, item := range typed {
			if isSensitiveKey(key) {
				continue
			}
			safeItem, ok := sanitizeValue(item)
			if ok {
				out[key] = safeItem
			}
		}
		return out, true
	default:
		return fmt.Sprintf("%v", typed), true
	}
}

func sanitizePathLikeString(input string) string {
	if input == "" {
		return input
	}
	if filepath.IsAbs(input) {
		return "[REDACTED_PATH]"
	}
	return input
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "" {
		return false
	}
	for _, token := range sensitiveKeyTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

var _ Recorder = (*Service)(nil)

var ErrRecordDropped = errors.New("audit event dropped")
