package audit

import "time"

type Result string

const (
	ResultSuccess Result = "success"
	ResultPartial Result = "partial"
	ResultFailure Result = "failure"
	ResultDenied  Result = "denied"
)

type Event struct {
	OccurredAt time.Time      `json:"occurredAt"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	Result     Result         `json:"result"`
	Target     string         `json:"target"`
	RequestID  string         `json:"requestId"`
	SpaceID    *int64         `json:"spaceId,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Log struct {
	ID         int64          `json:"id"`
	OccurredAt time.Time      `json:"occurredAt"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	Result     Result         `json:"result"`
	Target     string         `json:"target"`
	RequestID  string         `json:"requestId"`
	SpaceID    *int64         `json:"spaceId,omitempty"`
	Metadata   map[string]any `json:"metadata"`
}

type ListFilter struct {
	Page     int
	PageSize int
	From     *time.Time
	To       *time.Time
	User     string
	Action   string
	SpaceID  *int64
	Result   Result
}

type ListResult struct {
	Items    []*Log `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Total    int64  `json:"total"`
}

type Recorder interface {
	RecordBestEffort(event Event)
}

func IsValidResult(result Result) bool {
	switch result {
	case ResultSuccess, ResultPartial, ResultFailure, ResultDenied:
		return true
	default:
		return false
	}
}
