package model

// ExecutionRequest represents the request structure for code execution
type ExecutionRequest struct {
	Code     string `json:"code" binding:"required"`
	Language string `json:"language" binding:"required"`
}

// ExecutionResponse represents the response structure for executed code
type ExecutionResponse struct {
	Output        string `json:"output"`
	Error         string `json:"error,omitempty"`
	StatusMessage string `json:"status_message"`
	Success       bool   `json:"success"`
	ExecutionTime string `json:"execution_time,omitempty"`
}

// ContainerStats represents the overall JSON structure.
type ContainerStats struct {
	Name         string       `json:"name"`
	ID           string       `json:"id"`
	Read         string       `json:"read"`
	PreRead      string       `json:"preread"`
	PidsStats    PidsStats    `json:"pids_stats"`
	BlkioStats   BlkioStats   `json:"blkio_stats"`
	NumProcs     int          `json:"num_procs"`
	StorageStats StorageStats `json:"storage_stats"`
	CPUStats     CPUStats     `json:"cpu_stats"`
	PreCPUStats  CPUStats     `json:"precpu_stats"`
	MemoryStats  MemoryStats  `json:"memory_stats"`
}

// PidsStats represents process statistics.
type PidsStats struct {
	Current int `json:"current"`
	Limit   int `json:"limit"`
}

// BlkioStats represents block I/O statistics.
type BlkioStats struct {
	IOServiceBytesRecursive []IOServiceBytes `json:"io_service_bytes_recursive"`
}

// IOServiceBytes represents a single I/O operation.
type IOServiceBytes struct {
	Major int    `json:"major"`
	Minor int    `json:"minor"`
	Op    string `json:"op"`
	Value int64  `json:"value"`
}

// StorageStats represents storage-related statistics.
type StorageStats struct{}

// CPUStats represents CPU statistics.
type CPUStats struct {
	CPUUsage       CPUUsage       `json:"cpu_usage"`
	SystemCPUUsage int64          `json:"system_cpu_usage"`
	OnlineCPUs     int            `json:"online_cpus"`
	ThrottlingData ThrottlingData `json:"throttling_data"`
}

// CPUUsage represents detailed CPU usage statistics.
type CPUUsage struct {
	TotalUsage        int64 `json:"total_usage"`
	UsageInKernelMode int64 `json:"usage_in_kernelmode"`
	UsageInUserMode   int64 `json:"usage_in_usermode"`
}

// ThrottlingData represents CPU throttling statistics.
type ThrottlingData struct {
	Periods          int `json:"periods"`
	ThrottledPeriods int `json:"throttled_periods"`
	ThrottledTime    int `json:"throttled_time"`
}

// MemoryStats represents memory statistics.
type MemoryStats struct {
	Usage int64         `json:"usage"`
	Stats MemoryDetails `json:"stats"`
	Limit int64         `json:"limit"`
}

// MemoryDetails represents detailed memory statistics.
type MemoryDetails struct {
	ActiveAnon            int `json:"active_anon"`
	ActiveFile            int `json:"active_file"`
	Anon                  int `json:"anon"`
	AnonThp               int `json:"anon_thp"`
	File                  int `json:"file"`
	FileDirty             int `json:"file_dirty"`
	FileMapped            int `json:"file_mapped"`
	FileWriteback         int `json:"file_writeback"`
	InactiveAnon          int `json:"inactive_anon"`
	InactiveFile          int `json:"inactive_file"`
	KernelStack           int `json:"kernel_stack"`
	PgActivate            int `json:"pgactivate"`
	PgDeactivate          int `json:"pgdeactivate"`
	PgFault               int `json:"pgfault"`
	PgLazyFree            int `json:"pglazyfree"`
	PgLazyFreed           int `json:"pglazyfreed"`
	PgMajFault            int `json:"pgmajfault"`
	PgRefill              int `json:"pgrefill"`
	PgScan                int `json:"pgscan"`
	PgSteal               int `json:"pgsteal"`
	Shmem                 int `json:"shmem"`
	Slab                  int `json:"slab"`
	SlabReclaimable       int `json:"slab_reclaimable"`
	SlabUnreclaimable     int `json:"slab_unreclaimable"`
	Sock                  int `json:"sock"`
	THPCollapseAlloc      int `json:"thp_collapse_alloc"`
	THPFaultAlloc         int `json:"thp_fault_alloc"`
	Unevictable           int `json:"unevictable"`
	WorkingSetActivate    int `json:"workingset_activate"`
	WorkingSetNodereclaim int `json:"workingset_nodereclaim"`
	WorkingSetRefault     int `json:"workingset_refault"`
}

// Stats represents the full Docker stats JSON structure
type Stats struct {
	Name        string      `json:"name"`
	ID          string      `json:"id"`
	Read        string      `json:"read"`
	Preread     string      `json:"preread"`
	PidsStats   PidsStats   `json:"pids_stats"`
	CPUStats    CPUStats    `json:"cpu_stats"`
	PreCPUStats CPUStats    `json:"precpu_stats"`
	MemoryStats MemoryStats `json:"memory_stats"`
}
