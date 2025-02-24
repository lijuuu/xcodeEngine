package executor

// ContainerState represents the current state of a container
type ContainerState string

const (
	StateIdle  ContainerState = "idle"
	StateBusy  ContainerState = "busy"
	StateError ContainerState = "error"
)

// ContainerInfo holds information about a container
type ContainerInfo struct {
	ID    string
	State ContainerState
}

// Job represents a code execution request
type Job struct {
	Language string
	Code     string
	Result   chan Result
}

// Result contains the output of code execution
type Result struct {
	Output        string
	Success       bool
	Error         error
	ExecutionTime string
}
