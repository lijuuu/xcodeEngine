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

	container "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

const (
	MaxWorkers = 2
)

// Language execution configurations
var configs = map[string]struct {
	timeout time.Duration
	args    func(code string) []string
}{
	"go": {
		timeout: 2 * time.Second,
		args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.go && go run /app/temp/code.go`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
	"js": {
		timeout: 2 * time.Second,
		args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.js && node /app/temp/code.js`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
	"python": {
		timeout: 2 * time.Second,
		args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.py && python3 /app/temp/code.py`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
	"cpp": {
		timeout: 2 * time.Second,
		args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.cpp && g++ -o /app/temp/exe /app/temp/code.cpp && /app/temp/exe`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
}

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
	Error         error
	ExecutionTime time.Duration
}

// WorkerPool manages a pool of docker containers for code execution
type WorkerPool struct {
	jobs         chan Job
	containers   map[string]*ContainerInfo
	mu           sync.Mutex
	dockerClient *client.Client
	logger       *log.Logger
}

// NewWorkerPool creates a new worker pool with specified number of containers
func NewWorkerPool() *WorkerPool {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	pool := &WorkerPool{
		jobs:         make(chan Job, 10),
		containers:   make(map[string]*ContainerInfo),
		dockerClient: dockerClient,
		logger:       log.New(log.Writer(), "[WorkerPool] ", log.LstdFlags),
	}

	// Initialize the container pool
	if err := pool.initializeContainerPool(); err != nil {
		log.Fatalf("Failed to initialize container pool: %v", err)
	}

	// Start worker goroutines
	for i := 0; i < MaxWorkers; i++ {
		go pool.worker(i + 1)
	}

	// Start container monitor
	go pool.monitorContainers()

	return pool
}

// initializeContainerPool makes sure we have the right number of containers
func (p *WorkerPool) initializeContainerPool() error {
	// List all containers
	containers, err := p.dockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	// Find existing worker containers
	var workerContainers []string
	for _, c := range containers {
		if c.Image == "worker" {
			workerContainers = append(workerContainers, c.ID)
			state := StateIdle
			if c.State != "running" {
				state = StateError
			}
			p.containers[c.ID] = &ContainerInfo{
				ID:    c.ID,
				State: state,
			}
			p.logger.Printf("Found existing worker container: %s (state: %s)", shortID(c.ID), state)
		}
	}

	// Handle container count
	existingCount := len(workerContainers)
	if existingCount > MaxWorkers {
		p.logger.Printf("Found %d worker containers, removing excess...", existingCount)
		for _, cID := range workerContainers[MaxWorkers:] {
			p.removeContainer(cID)
		}
	} else if existingCount < MaxWorkers {
		p.logger.Printf("Only %d worker containers found, creating %d more...",
			existingCount, MaxWorkers-existingCount)
		for i := 0; i < MaxWorkers-existingCount; i++ {
			if err := p.startContainer(); err != nil {
				p.logger.Printf("Failed to start container: %v", err)
			}
		}
	}

	// Verify we have containers
	if len(p.containers) == 0 {
		return errors.New("failed to initialize container pool")
	}

	return nil
}

// startContainer creates and starts a new worker container
func (p *WorkerPool) startContainer() error {
	ctx := context.Background()

	// Check if we already have enough containers
	p.mu.Lock()
	containerCount := len(p.containers)
	p.mu.Unlock()

	if containerCount >= MaxWorkers {
		p.logger.Printf("Already have %d containers, not starting new one", containerCount)
		return nil
	}

	// Container configuration
	config := &container.Config{
		Image: "worker",
		Tty:   true,
	}
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   500 * 1024 * 1024, // 500MB
			NanoCPUs: 1000000000,        // 1 CPU
		},
		NetworkMode: "none", // Disable networking
	}

	// Create container
	resp, err := p.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	// Start container
	if err := p.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		p.dockerClient.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return fmt.Errorf("failed to start container: %v", err)
	}

	// Register container in our pool
	p.mu.Lock()
	p.containers[resp.ID] = &ContainerInfo{
		ID:    resp.ID,
		State: StateIdle,
	}
	p.mu.Unlock()

	p.logger.Printf("Started new worker container: %s", shortID(resp.ID))
	return nil
}

// removeContainer removes a container from the pool
func (p *WorkerPool) removeContainer(containerID string) {
	ctx := context.Background()

	// Try to remove the container
	if err := p.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		p.logger.Printf("Failed to remove container %s: %v", shortID(containerID), err)
	} else {
		p.logger.Printf("Removed container: %s", shortID(containerID))
	}

	// Remove from our tracking
	p.mu.Lock()
	delete(p.containers, containerID)
	p.mu.Unlock()
}

// monitorContainers checks container health periodically
func (p *WorkerPool) monitorContainers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		p.checkContainerHealth()
	}
}

// checkContainerHealth ensures container health and correct count
func (p *WorkerPool) checkContainerHealth() {
	ctx := context.Background()

	// Get all containers from Docker
	containers, err := p.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		p.logger.Printf("Failed to list containers: %v", err)
		return
	}

	// Build map of running worker containers
	runningWorkers := make(map[string]bool)
	for _, c := range containers {
		if c.Image == "worker" && c.State == "running" {
			runningWorkers[c.ID] = true
		}
	}

	// Check our container states against Docker's view
	p.mu.Lock()
	containersToRemove := []string{}

	for id, _ := range p.containers {
		if !runningWorkers[id] {
			// Container is not running according to Docker
			p.logger.Printf("Container %s not running, marking for removal", shortID(id))
			containersToRemove = append(containersToRemove, id)
		}
	}

	// Get current container count
	currentCount := len(p.containers) - len(containersToRemove)
	p.mu.Unlock()

	// Remove marked containers
	for _, id := range containersToRemove {
		p.removeContainer(id)
	}

	// Start new containers if needed
	if currentCount < MaxWorkers {
		p.logger.Printf("Container count (%d) below target (%d), starting new containers",
			currentCount, MaxWorkers)
		for i := 0; i < MaxWorkers-currentCount; i++ {
			if err := p.startContainer(); err != nil {
				p.logger.Printf("Failed to start replacement container: %v", err)
			}
		}
	} else if len(runningWorkers) > MaxWorkers {
		p.logger.Printf("Too many running worker containers (%d), target is %d",
			len(runningWorkers), MaxWorkers)

		// Identify excess containers to remove
		p.mu.Lock()
		var idleContainers []string
		for id, info := range p.containers {
			if info.State == StateIdle {
				idleContainers = append(idleContainers, id)
			}
		}
		p.mu.Unlock()

		// Remove excess idle containers
		excessCount := len(runningWorkers) - MaxWorkers
		for i := 0; i < excessCount && i < len(idleContainers); i++ {
			p.removeContainer(idleContainers[i])
		}
	}
}

// worker processes jobs from the queue
func (p *WorkerPool) worker(id int) {
	p.logger.Printf("Worker %d started", id)

	for job := range p.jobs {
		// Get an available container
		containerID, err := p.getAvailableContainer()
		if err != nil {
			p.logger.Printf("Worker %d couldn't get container: %v", id, err)
			job.Result <- Result{Error: err}
			continue
		}

		// Execute the job
		p.logger.Printf("Worker %d executing in container %s", id, shortID(containerID))
		p.setContainerState(containerID, StateBusy)

		start := time.Now()
		output, err := p.executeCode(containerID, job.Language, job.Code)
		duration := time.Since(start)
		p.setContainerState(containerID, StateIdle)
		job.Result <- Result{Output: output, Error: err, ExecutionTime: duration}
	}
}

// getAvailableContainer finds an idle container
func (p *WorkerPool) getAvailableContainer() (string, error) {
	maxRetries := 10
	retryDelay := 200 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		// Try to find an idle container
		p.mu.Lock()
		for id, info := range p.containers {
			if info.State == StateIdle {
				p.mu.Unlock()
				return id, nil
			}
		}
		p.mu.Unlock()

		// No idle container found, wait before retrying
		time.Sleep(retryDelay)
	}

	return "", errors.New("no available containers after retries")
}

// setContainerState updates a container's state
func (p *WorkerPool) setContainerState(containerID string, state ContainerState) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if container, exists := p.containers[containerID]; exists {
		container.State = state
	}
}

// executeCode runs code in a container with the proper language-specific settings
func (p *WorkerPool) executeCode(containerID, language, code string) (string, error) {
	config, ok := configs[language]
	if !ok {
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()

	args := append([]string{"exec", containerID}, config.args(code)...)
	cmd := exec.CommandContext(ctx, "docker", args...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("execution timed out after %v", config.timeout)
	}

	if err != nil {
		return output.String(), fmt.Errorf("execution error: %w", err)
	}

	return output.String(), nil
}

// ExecuteJob submits a job to the worker pool
func (p *WorkerPool) ExecuteJob(language, code string) Result {
	result := make(chan Result, 1)
	p.jobs <- Job{Language: language, Code: code, Result: result}
	return <-result
}

// Shutdown stops the worker pool
func (p *WorkerPool) Shutdown() {
	p.logger.Println("Shutting down worker pool...")
	close(p.jobs)

	// Clean up containers
	p.mu.Lock()
	for id := range p.containers {
		p.dockerClient.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	}
	p.mu.Unlock()
}

// shortID returns a shortened container ID for logging
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
