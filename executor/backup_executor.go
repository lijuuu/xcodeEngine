package executor

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"log"
// 	"os"
// 	"os/exec"
// 	"strings"
// 	"sync"
// 	"time"

// 	container "github.com/docker/docker/api/types/container"
// 	"github.com/docker/docker/client"
// 	logrus "github.com/sirupsen/logrus"
// )

// // Language execution configurations
// var configs = map[string]struct {
// 	timeout time.Duration
// 	args    func(code string) []string
// }{
// 	"go": {
// 		timeout: 10 * time.Second,
// 		args: func(code string) []string {
// 			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.go && go run /app/temp/code.go`,
// 				strings.ReplaceAll(code, "'", "'\\''"))}
// 		},
// 	},
// 	"js": {
// 		timeout: 10 * time.Second,
// 		args: func(code string) []string {
// 			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.js && node /app/temp/code.js`,
// 				strings.ReplaceAll(code, "'", "'\\''"))}
// 		},
// 	},
// 	"python": {
// 		timeout: 10 * time.Second,
// 		args: func(code string) []string {
// 			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.py && python3 /app/temp/code.py`,
// 				strings.ReplaceAll(code, "'", "'\\''"))}
// 		},
// 	},
// 	"cpp": {
// 		timeout: 10 * time.Second,
// 		args: func(code string) []string {
// 			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.cpp && g++ -o /app/temp/exe /app/temp/code.cpp && /app/temp/exe`,
// 				strings.ReplaceAll(code, "'", "'\\''"))}
// 		},
// 	},
// }

// // ContainerState represents the current state of a container
// type ContainerState string

// const (
// 	StateIdle  ContainerState = "idle"
// 	StateBusy  ContainerState = "busy"
// 	StateError ContainerState = "error"
// )

// // ContainerInfo holds information about a container
// type ContainerInfo struct {
// 	ID    string
// 	State ContainerState
// }

// // Job represents a code execution request
// type Job struct {
// 	Language string
// 	Code     string
// 	Result   chan Result
// }

// // Result contains the output of code execution
// type Result struct {
// 	Output        string
// 	Success       bool
// 	Error         error
// 	ExecutionTime string
// }

// // WorkerPool manages a pool of docker containers for code execution
// type WorkerPool struct {
// 	jobs         chan Job
// 	containers   map[string]*ContainerInfo
// 	mu           sync.Mutex
// 	dockerClient *client.Client
// 	logrus       *logrus.Logger
// 	maxWorkers   int
// 	maxJobCount  int
// }

// // NewWorkerPool creates a new worker pool with specified number of containers
// func NewWorkerPool(maxWorkers, maxJobCount int) *WorkerPool {
// 	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
// 	if err != nil {
// 		logrus.Fatalf("Failed to create Docker client: %v", err)
// 	}

// 	pool := &WorkerPool{
// 		jobs:         make(chan Job, 10),
// 		containers:   make(map[string]*ContainerInfo),
// 		dockerClient: dockerClient,
// 		logrus:       logrus.New(),
// 		maxWorkers:   maxWorkers,
// 		maxJobCount:  maxJobCount,
// 	}

// 	logFile, err := os.OpenFile("logs/container.log", os.O_WRONLY|os.O_APPEND, 0644)
// 	if err != nil {
// 		logrus.Fatalf("Failed to open log file: %v", err)
// 	}
// 	pool.logrus.SetOutput(logFile)

// 	// Initialize the container pool
// 	if err := pool.initializeContainerPool(); err != nil {
// 		logrus.Fatalf("Failed to initialize container pool: %v", err)
// 	}

// 	// Start worker goroutines
// 	for i := 0; i < pool.maxWorkers; i++ {
// 		go pool.worker(i + 1)
// 	}

// 	// Start container monitor
// 	go pool.monitorContainers()

// 	return pool
// }

// // initializeContainerPool makes sure we have the right number of containers
// func (p *WorkerPool) initializeContainerPool() error {
// 	// List all containers
// 	containers, err := p.dockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
// 	if err != nil {
// 		logrus.Errorf("failed to list containers: %v", err)
// 		return fmt.Errorf("failed to list containers: %v", err)
// 	}

// 	// Find existing worker containers
// 	var workerContainers []string
// 	for _, c := range containers {
// 		if c.Image == "worker" {
// 			workerContainers = append(workerContainers, c.ID)
// 			state := StateIdle
// 			if c.State != "running" {
// 				state = StateError
// 			}
// 			p.containers[c.ID] = &ContainerInfo{
// 				ID:    c.ID,
// 				State: state,
// 			}
// 			p.logrus.Printf("Found existing worker container: %s (state: %s)", (c.ID), state)
// 		}
// 	}

// 	// Handle container count
// 	existingCount := len(workerContainers)
// 	if existingCount > p.maxWorkers {
// 		p.logrus.Printf("Found %d worker containers, removing excess...", existingCount)
// 		for _, cID := range workerContainers[p.maxWorkers:] {
// 			p.removeContainer(cID)
// 		}
// 	} else if existingCount < p.maxWorkers {
// 		p.logrus.Printf("Only %d worker containers found, creating %d more...",
// 			existingCount, p.maxWorkers-existingCount)
// 		for i := 0; i < p.maxWorkers-existingCount; i++ {
// 			if err := p.startContainer(); err != nil {
// 				p.logrus.Printf("Failed to start container: %v", err)
// 			}
// 		}
// 	}

// 	// Verify we have containers
// 	if len(p.containers) == 0 {
// 		return errors.New("failed to initialize container pool")
// 	}

// 	return nil
// }

// // startContainer creates and starts a new worker container
// func (p *WorkerPool) startContainer() error {
// 	ctx := context.Background()

// 	// Check if we already have enough containers
// 	p.mu.Lock()
// 	containerCount := len(p.containers)
// 	p.mu.Unlock()

// 	if containerCount >= p.maxWorkers {
// 		p.logrus.Printf("Already have %d containers, not starting new one", containerCount)
// 		return nil
// 	}

// 	// Container configuration
// 	config := &container.Config{
// 		Image: "worker",
// 		Tty:   true,
// 	}
// 	seccompProfile := `{
// 		"defaultAction": "SCMP_ACT_ALLOW",
// 		"architectures": ["SCMP_ARCH_X86_64"],
// 		"syscalls": [
// 			{ "names": [ "setuid", "setgid", "kill", "clone", "fork", "vfork",
//       "socket", "connect", "bind", "accept",
//       "ptrace", "personality",
//       "syslog", "sysctl"], "action": "SCMP_ACT_DENY" }
// 		]
// 	}`
// 	// Validate JSON format (optional but recommended)
// 	var jsonCheck map[string]interface{}
// 	if err := json.Unmarshal([]byte(seccompProfile), &jsonCheck); err != nil {
// 		log.Fatalf("Invalid Seccomp JSON: %v", err)
// 	}

// 	hostConfig := &container.HostConfig{
// 		Resources: container.Resources{
// 			Memory:   200 * 1024 * 1024, // 500MB
// 			NanoCPUs: 1000000000,        // 1 CPU
// 		},
// 		NetworkMode: "none", // Disable networking
// 		// SecurityOpt: []string{"seccomp=" + seccompProfile, "no-new-privileges"}, // Pass JSON directly
// 	}
// 	start := time.Now()

// 	// Create container
// 	resp, err := p.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
// 	if err != nil {
// 		logrus.Errorf("failed to create container: %v", err)
// 		return fmt.Errorf("failed to create container: %v", err)
// 	}

// 	// Start container
// 	if err := p.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
// 		p.dockerClient.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
// 		logrus.Errorf("failed to start container: %v", err)
// 		return fmt.Errorf("failed to start container: %v", err)
// 	}

// 	// Register container in our pool
// 	p.mu.Lock()
// 	p.containers[resp.ID] = &ContainerInfo{
// 		ID:    resp.ID,
// 		State: StateIdle,
// 	}
// 	p.mu.Unlock()

// 	p.logrus.Printf("Started new worker container: %s within time %v", (resp.ID), time.Since(start))
// 	return nil
// }

// // removeContainer removes a container from the pool
// func (p *WorkerPool) removeContainer(containerID string) {
// 	ctx := context.Background()

// 	// Try to remove the container
// 	if err := p.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
// 		p.logrus.Printf("Failed to remove container %s: %v", (containerID), err)
// 	} else {
// 		p.logrus.Printf("Removed container: %s", (containerID))
// 	}

// 	// Remove from our tracking
// 	p.mu.Lock()
// 	delete(p.containers, containerID)
// 	p.mu.Unlock()
// }

// // monitorContainers checks container health periodically
// func (p *WorkerPool) monitorContainers() {
// 	ticker := time.NewTicker(1 * time.Second)
// 	defer ticker.Stop()

// 	for range ticker.C {
// 		p.checkContainerHealth()
// 	}
// }

// // checkContainerHealth ensures container health and correct count
// func (p *WorkerPool) checkContainerHealth() {
// 	ctx := context.Background()

// 	// Get all containers from Docker
// 	containers, err := p.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
// 	if err != nil {
// 		p.logrus.Printf("Failed to list containers: %v", err)
// 		return
// 	}

// 	// Build map of running worker containers
// 	runningWorkers := make(map[string]bool)
// 	for _, c := range containers {
// 		if c.Image == "worker" && c.State == "running" {
// 			runningWorkers[c.ID] = true
// 		}
// 	}

// 	// Check our container states against Docker's view
// 	p.mu.Lock()
// 	containersToRemove := []string{}

// 	for id, _ := range p.containers {
// 		if !runningWorkers[id] {
// 			// Container is not running according to Docker
// 			p.logrus.Printf("Container %s not running, marking for removal", (id))
// 			containersToRemove = append(containersToRemove, id)
// 		}
// 	}

// 	// Get current container count
// 	currentCount := len(p.containers) - len(containersToRemove)
// 	p.mu.Unlock()

// 	// Remove marked containers
// 	for _, id := range containersToRemove {
// 		p.removeContainer(id)
// 	}

// 	// Start new containers if needed
// 	if currentCount < p.maxWorkers {
// 		p.logrus.Printf("Container count (%d) below target (%d), starting new containers",
// 			currentCount, p.maxWorkers)
// 		for i := 0; i < p.maxWorkers-currentCount; i++ {
// 			start := time.Now()
// 			fmt.Println("Starting new container")
// 			if err := p.startContainer(); err != nil {
// 				p.logrus.Printf("Failed to start replacement container: %v", err)
// 			}
// 			duration := time.Since(start)
// 			fmt.Println("Container started in ", duration)
// 		}
// 	} else if len(runningWorkers) > p.maxWorkers {
// 		p.logrus.Printf("Too many running worker containers (%d), target is %d",
// 			len(runningWorkers), p.maxWorkers)

// 		// Identify excess containers to remove
// 		p.mu.Lock()
// 		var idleContainers []string
// 		for id, info := range p.containers {
// 			if info.State == StateIdle {
// 				idleContainers = append(idleContainers, id)
// 			}
// 		}
// 		p.mu.Unlock()

// 		// Remove excess idle containers
// 		excessCount := len(runningWorkers) - p.maxWorkers
// 		for i := 0; i < excessCount && i < len(idleContainers); i++ {
// 			p.removeContainer(idleContainers[i])
// 		}
// 	}
// }

// // worker processes jobs from the queue
// func (p *WorkerPool) worker(id int) {
// 	p.logrus.Printf("Worker %d started", id)

// 	for job := range p.jobs {
// 		// Get an available container
// 		containerID, err := p.getAvailableContainer()
// 		if err != nil {
// 			p.logrus.Printf("Worker %d couldn't get container: %v", id, err)
// 			job.Result <- Result{Error: err}
// 			continue
// 		}

// 		// Execute the job
// 		p.logrus.Printf("Worker %d executing in container %s", id, (containerID))
// 		p.setContainerState(containerID, StateBusy)

// 		start := time.Now()
// 		output, success, err := p.executeCode(containerID, job.Language, job.Code)
// 		duration := time.Since(start)
// 		p.setContainerState(containerID, StateIdle)
// 		job.Result <- Result{Output: output, Success: success, Error: err, ExecutionTime: fmt.Sprintf("%dms", duration.Milliseconds())}
// 	}
// }

// // getAvailableContainer finds an idle container
// func (p *WorkerPool) getAvailableContainer() (string, error) {
// 	maxRetries := 10
// 	retryDelay := 200 * time.Millisecond

// 	for i := 0; i < maxRetries; i++ {
// 		fmt.Println("Getting available container", p.containers)
// 		// Try to find an idle container
// 		p.mu.Lock()
// 		for id, info := range p.containers {
// 			fmt.Println("Container state:", info.State)
// 			if info.State == StateIdle {
// 				p.mu.Unlock()
// 				return id, nil
// 			}
// 		}
// 		fmt.Println("No idle container found ", p.containers)
// 		p.mu.Unlock()

// 		// No idle container found, wait before retrying
// 		time.Sleep(retryDelay)
// 	}

// 	return "", errors.New("no available containers after retries")
// }

// // setContainerState updates a container's state
// func (p *WorkerPool) setContainerState(containerID string, state ContainerState) {
// 	p.mu.Lock()
// 	defer p.mu.Unlock()

// 	if container, exists := p.containers[containerID]; exists {
// 		container.State = state
// 	}
// }

// // executeCode runs code in a container with the proper language-specific settings
// func (p *WorkerPool) executeCode(containerID, language, code string) (string, bool, error) {
// 	config, ok := configs[language]
// 	if !ok {
// 		return "", false, fmt.Errorf("unsupported language: %s", language)
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
// 	defer cancel()

// 	var output bytes.Buffer
// 	cmd := exec.CommandContext(ctx, "docker", append([]string{"exec", containerID}, config.args(code)...)...)
// 	cmd.Stdout = &output
// 	cmd.Stderr = &output

// 	start := time.Now()
// 	err := cmd.Run()
// 	duration := time.Since(start)

// 	if ctx.Err() == context.DeadlineExceeded {
// 		p.logrus.WithFields(logrus.Fields{
// 			"container": (containerID),
// 			"language":  language,
// 			"duration":  duration,
// 		}).Warn("Execution timeout after " + duration.String() + "removing and creating new container")

// 		p.removeContainer(containerID)
// 		p.startContainer()
// 		return "", false, fmt.Errorf("timeout after %v", config.timeout)
// 	}

// 	if err != nil {
// 		p.logrus.WithFields(logrus.Fields{
// 			"container": (containerID),
// 			"language":  language,
// 			"duration":  duration,
// 			"error":     err,
// 		}).Error("Execution failed")
// 		return output.String(), false, fmt.Errorf("execution error: %w", err)
// 	}

// 	p.logrus.WithFields(logrus.Fields{
// 		"container": (containerID),
// 		"language":  language,
// 		"duration":  duration,
// 	}).Debug("Execution completed")

// 	return output.String(), true, nil
// }

// // ExecuteJob submits a job to the worker pool
// func (p *WorkerPool) ExecuteJob(language, code string) Result {
// 	result := make(chan Result, p.maxJobCount)
// 	p.jobs <- Job{Language: language, Code: code, Result: result}
// 	return <-result
// }

// // Shutdown stops the worker pool
// func (p *WorkerPool) Shutdown() {
// 	p.logrus.Println("Shutting down worker pool...")
// 	close(p.jobs)

// 	// Clean up containers
// 	p.mu.Lock()
// 	for id := range p.containers {
// 		p.dockerClient.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
// 	}
// 	p.mu.Unlock()
// }
