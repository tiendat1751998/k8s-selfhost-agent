package incident

// Type represents the kind of Kubernetes incident.
type Type string

const (
	TypeCrashLoopBackOff Type = "CrashLoopBackOff"
	TypeOOMKilled        Type = "OOMKilled"
	TypeImagePullBackOff Type = "ImagePullBackOff"
	TypeFailedScheduling Type = "FailedScheduling"
	TypeNodeNotReady     Type = "NodeNotReady"
	TypeProbeFailed      Type = "ProbeFailed"
	TypeNetworkFailure   Type = "NetworkFailure"
	TypeStorageFailure   Type = "StorageFailure"
	TypeHPAFailure       Type = "HPAFailure"
	TypeResourceExhaust  Type = "ResourceExhaustion"
	TypeIngressFailure   Type = "IngressFailure"
	TypeServiceUnhealthy Type = "ServiceUnhealthy"
)

// validTypes is the set of all valid incident types.
var validTypes = map[Type]bool{
	TypeCrashLoopBackOff: true,
	TypeOOMKilled:        true,
	TypeImagePullBackOff: true,
	TypeFailedScheduling: true,
	TypeNodeNotReady:     true,
	TypeProbeFailed:      true,
	TypeNetworkFailure:   true,
	TypeStorageFailure:   true,
	TypeHPAFailure:       true,
	TypeResourceExhaust:  true,
	TypeIngressFailure:   true,
	TypeServiceUnhealthy: true,
}

// IsValid returns true if the Type is a recognized incident type.
func (t Type) IsValid() bool {
	return validTypes[t]
}

// String returns the string representation of the Type.
func (t Type) String() string {
	return string(t)
}

// Status represents the lifecycle state of an incident.
type Status string

const (
	StatusDetected    Status = "detected"
	StatusAnalyzing   Status = "analyzing"
	StatusRemediating Status = "remediating"
	StatusResolved    Status = "resolved"
	StatusFailed      Status = "failed"
)

// String returns the string representation of the Status.
func (s Status) String() string {
	return string(s)
}

// Severity represents the severity level of an incident.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// validSeverities is the set of all valid severity levels.
var validSeverities = map[Severity]bool{
	SeverityCritical: true,
	SeverityHigh:     true,
	SeverityMedium:   true,
	SeverityLow:      true,
}

// IsValid returns true if the Severity is a recognized level.
func (s Severity) IsValid() bool {
	return validSeverities[s]
}

// String returns the string representation of the Severity.
func (s Severity) String() string {
	return string(s)
}
