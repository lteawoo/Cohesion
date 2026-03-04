package smb

type ReadinessState string

const (
	StateHealthy     ReadinessState = "healthy"
	StateUnhealthy   ReadinessState = "unhealthy"
	StateUnavailable ReadinessState = "unavailable"
)

type FailureStage string

const (
	StageNone    FailureStage = ""
	StageBind    FailureStage = "bind"
	StageAccept  FailureStage = "accept"
	StageSession FailureStage = "session"
	StageStop    FailureStage = "stop"
)

const (
	ReasonDisabled        = "disabled"
	ReasonReady           = "ready"
	ReasonBindNotReady    = "bind_not_ready"
	ReasonAcceptFailed    = "accept_failed"
	ReasonRuntimeNotReady = "runtime_not_ready"
	ReasonRuntimeError    = "runtime_error"
)

type Readiness struct {
	State        ReadinessState
	Reason       string
	Stage        FailureStage
	Message      string
	Port         int
	EndpointMode string
	RolloutPhase string
	PolicySource string
	MinVersion   string
	MaxVersion   string
	BindReady    bool
	RuntimeReady bool
}
