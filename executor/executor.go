package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// ContainerState represents the current state of a container
type ContainerState string

// Container states
const (
	StateIdle      ContainerState = "idle"
	StateBusy      ContainerState = "busy"
	StateError     ContainerState = "error"
	StateStarting  ContainerState = "starting"
	StateShutdown  ContainerState = "shutdown"
	
	// Configuration constants
	MaxWorkers               int           = 2
	ContainerStartTimeout    time.Duration = 30 * time.Second
	ContainerExecuteTimeout  time.Duration = 10 * time.Second
	ContainerMonitorInterval time.Duration = 5 * time.Second
	MaxJobQueueSize          int           = 20
	MaxExecutionMemory       int64         = 500 * 1024 * 1024 // 500MB
	MaxExecutionCPU          int64         = 1000000000        // 1 CPU core
	ContainerImageName       string        = "worker"
)

// ErrPool defines pool-level errors
var (
	ErrNoContainersAvailable = errors.New("no containers available")
	ErrPoolShutdown          = errors.New("worker pool is shutting down")
	ErrContainerNotFound     = errors.New("container not found")
	ErrContainerCreation     = errors.New("failed to create container")
	ErrContainerStart        = errors.New("failed to start container")
	ErrDockerClientInit      = errors.New("failed to initialize Docker client")
	ErrExecutionTimeout      = errors.New("execution timed out")
)

// ContainerInfo holds information about a container
type ContainerInfo struct {
	ID            string
	State         ContainerState
	CreatedAt     time.Time
	LastUsed      time.Time
	ExecutionCount int
	Errors        int
}

// WorkerPool manages a pool of docker containers for code execution
type WorkerPool struct {
	jobs           chan Job
	containers     map[string]*ContainerInfo
	mu             sync.RWMutex
	dockerClient   *client.Client
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	shuttingDown   bool
	logger         *log.Logger
}

// Job represents a code execution request
type Job struct {
	Language   string
	Code       string
	Result     chan Result
	ctx        context.Context
	timeout    time.Duration
}

// Result contains the output of code execution
type Result struct {
	Output    string
	Error     error
	Container string
	Duration  time.Duration
}

// NewWorkerPool initializes a new worker pool with container management
func NewWorkerPool(logger *log.Logger) (*WorkerPool, error) {
	if logger == nil {
		logger = log.New(log.Writer(), "[WorkerPool] ", log.LstdFlags)
	}

	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		logger.Printf("Failed to create Docker client: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrDockerClientInit, err)
	}

	// Check Docker connectivity
	ping, err := dockerClient.Ping(context.Background())
	if err != nil {
		logger.Printf("Cannot connect to Docker daemon: %v", err)
		return nil, fmt.Errorf("docker daemon connection error: %v", err)
	}
	logger.Printf("Connected to Docker (API Version: %s)", ping.APIVersion)

	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &WorkerPool{
		jobs:         make(chan Job, MaxJobQueueSize),
		containers:   make(map[string]*ContainerInfo),
		dockerClient: dockerClient,
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
	}

	if err := pool.ensureContainerPool(); err != nil {
		cancel() // Clean up context before returning error
		return nil, err
	}

	// Start monitoring routine
	pool.wg.Add(1)
	go func() {
		defer pool.wg.Done()
		pool.monitorContainers()
	}()

	// Start worker routines
	for i := 0; i < MaxWorkers; i++ {
		pool.wg.Add(1)
		go func(workerID int) {
			defer pool.wg.Done()
			pool.worker(workerID)
		}(i + 1)
	}

	return pool, nil
}

// ensureContainerPool makes sure the required number of containers are running
func (p *WorkerPool) ensureContainerPool() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	containers, err := p.dockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		p.logger.Printf("Failed to list containers: %v", err)
		return fmt.Errorf("failed to list containers: %v", err)
	}

	// Find existing worker containers
	var workerContainers []string
	for _, c := range containers {
		if c.Image == ContainerImageName {
			status := StateIdle
			if c.State != "running" {
				status = StateError
			}
			
			workerContainers = append(workerContainers, c.ID)
			p.containers[c.ID] = &ContainerInfo{
				ID:        c.ID,
				State:     status,
				CreatedAt: time.Now(), // Approximate
				LastUsed:  time.Now(),
			}
		}
	}

	// Handle container count adjustment
	existingCount := len(workerContainers)
	if existingCount > MaxWorkers {
		p.logger.Printf("Found %d worker containers, removing excess...", existingCount)
		for _, cID := range workerContainers[MaxWorkers:] {
			if err := p.dockerClient.ContainerRemove(context.Background(), cID, container.RemoveOptions{Force: true}); err != nil {
				p.logger.Printf("Warning: Failed to remove excess container %s: %v", cID, err)
			} else {
				delete(p.containers, cID)
			}
		}
	} else if existingCount < MaxWorkers {
		missing := MaxWorkers - existingCount
		p.logger.Printf("Only %d worker containers found, creating %d more...", existingCount, missing)
		
		var startErrors []error
		for i := 0; i < missing; i++ {
			if _, err := p.startContainer(); err != nil {
				startErrors = append(startErrors, err)
			}
		}
		
		if len(startErrors) > 0 {
			// We'll proceed even with partial container pool
			p.logger.Printf("Warning: Started with partial container pool. %d containers failed to start", len(startErrors))
		}
	}

	// Quick validation that we have at least one container
	if len(p.containers) == 0 {
		return errors.New("failed to initialize any containers in the pool")
	}

	return nil
}

// startContainer creates and starts a new worker container
func (p *WorkerPool) startContainer() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ContainerStartTimeout)
	defer cancel()

	config := &container.Config{
		Image: ContainerImageName,
		Tty:   true,
		Labels: map[string]string{
			"purpose": "code-execution",
			"managed": "true",
		},
	}
	
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   MaxExecutionMemory,
			NanoCPUs: MaxExecutionCPU,
		},
		NetworkMode: "none",
		AutoRemove:  false,
		SecurityOpt: []string{"no-new-privileges:true"},
	}

	resp, err := p.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		p.logger.Printf("Failed to create container: %v", err)
		return "", fmt.Errorf("%w: %v", ErrContainerCreation, err)
	}

	containerID := resp.ID

	// Mark as starting in our map before actually starting
	p.mu.Lock()
	p.containers[containerID] = &ContainerInfo{
		ID:        containerID,
		State:     StateStarting,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
	p.mu.Unlock()

	if err := p.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		p.mu.Lock()
		p.containers[containerID].State = StateError
		p.mu.Unlock()
		p.logger.Printf("Failed to start container %s: %v", containerID[:12], err)
		return "", fmt.Errorf("%w: %v", ErrContainerStart, err)
	}

	// Verify container is running
	inspect, err := p.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil || !inspect.State.Running {
		p.mu.Lock()
		p.containers[containerID].State = StateError
		p.mu.Unlock()
		return "", fmt.Errorf("container failed to enter running state: %v", err)
	}

	p.mu.Lock()
	p.containers[containerID].State = StateIdle
	p.mu.Unlock()
	
	p.logger.Printf("Started worker container: %s", containerID[:12])
	return containerID, nil
}

// monitorContainers periodically checks container health and restarts failed ones
func (p *WorkerPool) monitorContainers() {
	ticker := time.NewTicker(ContainerMonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Println("Container monitor shutting down")
			return
		case <-ticker.C:
			p.healthCheck()
		}
	}
}

// healthCheck performs health check on all containers
func (p *WorkerPool) healthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current containers
	running, err := p.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		p.logger.Printf("Health check failed to list containers: %v", err)
		return
	}

	runningMap := make(map[string]types.Container)
	for _, c := range running {
		runningMap[c.ID] = c
	}

	containerIDs := make([]string, 0)
	containersToRestart := make([]string, 0)

	// First pass: collect container IDs and mark containers for restart
	p.mu.Lock()
	for id, info := range p.containers {
		containerIDs = append(containerIDs, id)
		
		// Check if container exists in Docker's list
		container, exists := runningMap[id]
		
		// Handle missing or not-running containers
		if !exists || container.State != "running" {
			p.logger.Printf("Container %s marked for restart: missing=%v, state=%s", id[:12], !exists, info.State)
			info.State = StateError
			containersToRestart = append(containersToRestart, id)
			continue
		}
		
		// Check for containers stuck in busy state too long (5+ minutes)
		if info.State == StateBusy && time.Since(info.LastUsed) > 5*time.Minute {
			p.logger.Printf("Container %s appears stuck in busy state, marking for restart", id[:12])
			info.State = StateError
			containersToRestart = append(containersToRestart, id)
		}
		
		// Check for idle containers with high error count
		if info.Errors > 3 && info.State == StateIdle {
			p.logger.Printf("Container %s has high error count (%d), marking for restart", 
				id[:12], info.Errors)
			info.State = StateError
			containersToRestart = append(containersToRestart, id)
		}
	}
	p.mu.Unlock()

	// Second pass: restart containers (do this outside of the mutex lock)
	for _, id := range containersToRestart {
		p.logger.Printf("Restarting container: %s", id[:12])
		
		// First try to remove the container
		err := p.dockerClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
		if err != nil {
			p.logger.Printf("Failed to remove container %s: %v", id[:12], err)
		}
		
		// Then remove from our tracking map
		p.mu.Lock()
		delete(p.containers, id)
		p.mu.Unlock()
	}

	// Start new containers if needed
	p.mu.RLock()
	currentCount := len(p.containers)
	p.mu.RUnlock()
	
	if currentCount < MaxWorkers {
		needed := MaxWorkers - currentCount
		p.logger.Printf("Container pool below target size (%d/%d), starting %d new container(s)", 
			currentCount, MaxWorkers, needed)
		
		for i := 0; i < needed; i++ {
			_, err := p.startContainer()
			if err != nil {
				p.logger.Printf("Failed to create replacement container: %v", err)
			}
		}
	}
}

// worker handles job execution
func (p *WorkerPool) worker(id int) {
	p.logger.Printf("Worker %d started", id)
	defer p.logger.Printf("Worker %d shutting down", id)

	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				// Channel closed, exit worker
				return
			}
			
			// If we have a job-specific context, use it
			execCtx := context.Background()
			if job.ctx != nil {
				execCtx = job.ctx
			}

			// Add job timeout if specified
			if job.timeout > 0 {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithTimeout(execCtx, job.timeout)
				defer cancel()
			}

			// Try to get a container with timeout
			containerID, err := p.getAvailableContainerWithTimeout(execCtx, 5*time.Second)
			if err != nil {
				job.Result <- Result{
					Error: err,
				}
				continue
			}

			// Execute the job
			start := time.Now()
			p.setContainerState(containerID, StateBusy)
			p.logger.Printf("Worker %d executing job in container: %s", id, containerID[:12])
			
			output, err := p.executeInContainer(execCtx, containerID, job.Language, job.Code)
			
			duration := time.Since(start)
			p.mu.Lock()
			if container, exists := p.containers[containerID]; exists {
				container.LastUsed = time.Now()
				container.ExecutionCount++
				if err != nil {
					container.Errors++
				}
				container.State = StateIdle
			}
			p.mu.Unlock()

			// Send result back
			job.Result <- Result{
				Output:    output,
				Error:     err,
				Container: containerID[:12],
				Duration:  duration,
			}
		}
	}
}

// getAvailableContainerWithTimeout attempts to get an available container with timeout
func (p *WorkerPool) getAvailableContainerWithTimeout(ctx context.Context, timeout time.Duration) (string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Try immediately first
	containerID := p.getAvailableContainer()
	if containerID != "" {
		return containerID, nil
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctxWithTimeout.Done():
			p.mu.RLock()
			totalCount := len(p.containers)
			busyCount := 0
			errorCount := 0
			for _, c := range p.containers {
				if c.State == StateBusy {
					busyCount++
				} else if c.State == StateError {
					errorCount++
				}
			}
			p.mu.RUnlock()
			
			return "", fmt.Errorf("%w (busy=%d, error=%d, total=%d)", 
				ErrNoContainersAvailable, busyCount, errorCount, totalCount)
			
		case <-ticker.C:
			containerID := p.getAvailableContainer()
			if containerID != "" {
				return containerID, nil
			}
		}
	}
}

// getAvailableContainer returns an idle container ID if available
func (p *WorkerPool) getAvailableContainer() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// First try to find any idle container
	for id, info := range p.containers {
		if info.State == StateIdle {
			return id
		}
	}
	
	return ""
}

// setContainerState updates a container's state
func (p *WorkerPool) setContainerState(containerID string, state ContainerState) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if container, exists := p.containers[containerID]; exists {
		container.State = state
		container.LastUsed = time.Now()
	}
}

// executeInContainer executes code in the specified container
func (p *WorkerPool) executeInContainer(ctx context.Context, containerID, language, code string) (string, error) {
	// Create the execution command with proper escaping
	execCommand := fmt.Sprintf(`echo '%s' | %s`, strings.ReplaceAll(code, "'", "'\\''"), language)
	
	// Use context for timeout support
	cmd := exec.CommandContext(ctx, "docker", "exec", containerID, "sh", "-c", execCommand)

	var output bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &stderr

	err := cmd.Run()
	combinedOutput := output.String()
	if stderr.Len() > 0 {
		if combinedOutput != "" {
			combinedOutput += "\n--- stderr ---\n"
		}
		combinedOutput += stderr.String()
	}

	if ctx.Err() == context.DeadlineExceeded {
		p.logger.Printf("Execution in container %s timed out", containerID[:12])
		return combinedOutput, ErrExecutionTimeout
	}

	if err != nil {
		return combinedOutput, fmt.Errorf("execution error in container %s: %w", containerID[:12], err)
	}
	
	return combinedOutput, nil
}

// ExecuteJob submits a job to the worker pool
func (p *WorkerPool) ExecuteJob(language, code string) Result {
	return p.ExecuteJobWithOptions(language, code, nil, ContainerExecuteTimeout)
}

// ExecuteJobWithOptions submits a job with context and timeout options
func (p *WorkerPool) ExecuteJobWithOptions(language, code string, ctx context.Context, timeout time.Duration) Result {
	p.mu.RLock()
	shuttingDown := p.shuttingDown
	p.mu.RUnlock()

	if shuttingDown {
		return Result{Error: ErrPoolShutdown}
	}

	result := make(chan Result, 1)
	job := Job{
		Language: language, 
		Code: code, 
		Result: result,
		ctx: ctx,
		timeout: timeout,
	}
	
	select {
	case p.jobs <- job:
		return <-result
	case <-time.After(100 * time.Millisecond):
		// If job queue is backed up, check if any containers are available
		if p.getAvailableContainer() == "" {
			return Result{Error: ErrNoContainersAvailable}
		}
		// Otherwise, try again with longer timeout
		select {
		case p.jobs <- job:
			return <-result
		case <-time.After(5 * time.Second):
			return Result{Error: errors.New("job queue is full, try again later")}
		}
	}
}

// GetPoolStats returns statistics about the worker pool
func (p *WorkerPool) GetPoolStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := map[string]interface{}{
		"total_containers": len(p.containers),
		"job_queue_size":   len(p.jobs),
		"job_queue_capacity": cap(p.jobs),
		"max_workers":      MaxWorkers,
		"containers":       make([]map[string]interface{}, 0),
	}

	idleCount := 0
	busyCount := 0
	errorCount := 0

	for id, container := range p.containers {
		containerInfo := map[string]interface{}{
			"id":              id[:12],
			"state":           string(container.State),
			"age":             time.Since(container.CreatedAt).String(),
			"last_used":       time.Since(container.LastUsed).String(),
			"execution_count": container.ExecutionCount,
			"error_count":     container.Errors,
		}
		stats["containers"] = append(stats["containers"].([]map[string]interface{}), containerInfo)

		switch container.State {
		case StateIdle:
			idleCount++
		case StateBusy:
			busyCount++
		case StateError:
			errorCount++
		}
	}

	stats["idle_containers"] = idleCount
	stats["busy_containers"] = busyCount
	stats["error_containers"] = errorCount

	return stats
}

// Shutdown gracefully shuts down the worker pool
func (p *WorkerPool) Shutdown(timeout time.Duration) error {
	p.logger.Println("Worker pool shutting down...")
	
	// Mark as shutting down to prevent new jobs
	p.mu.Lock()
	p.shuttingDown = true
	p.mu.Unlock()
	
	// Signal workers to stop
	p.cancel()
	
	// Close job channel after a short delay to allow current jobs to be processed
	time.Sleep(100 * time.Millisecond)
	close(p.jobs)
	
	// Wait for all workers to finish with timeout
	c := make(chan struct{})
	go func() {
		defer close(c)
		p.wg.Wait()
	}()
	
	select {
	case <-c:
		// Workers finished gracefully
	case <-time.After(timeout):
		p.logger.Println("Shutdown timed out, some workers may still be running")
	}
	
	// Clean up containers
	return p.cleanupContainers(timeout)
}

// cleanupContainers removes all worker containers
func (p *WorkerPool) cleanupContainers(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	p.mu.Lock()
	containerIDs := make([]string, 0, len(p.containers))
	for id := range p.containers {
		containerIDs = append(containerIDs, id)
	}
	p.mu.Unlock()
	
	var lastErr error
	for _, id := range containerIDs {
		p.logger.Printf("Removing container %s", id[:12])
		if err := p.dockerClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
			p.logger.Printf("Failed to remove container %s: %v", id[:12], err)
			lastErr = err
		}
	}
	
	return lastErr
}