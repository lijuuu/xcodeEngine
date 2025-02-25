package executor

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	logrus "github.com/sirupsen/logrus"
)

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


// ContainerManager manages Docker containers for the worker pool
type ContainerManager struct {
	dockerClient *client.Client
	containers   map[string]*ContainerInfo
	mu           sync.Mutex
	logger       *logrus.Logger
	maxWorkers   int
}

// NewContainerManager creates a new container manager
func NewContainerManager(maxWorkers int) (*ContainerManager, error) {
	dockerClient, err := client.NewClientWithOpts()
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}

	logger := logrus.New()
	logFile, err := os.OpenFile("logs/container.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	logger.SetOutput(logFile)

	return &ContainerManager{
		dockerClient: dockerClient,
		containers:   make(map[string]*ContainerInfo),
		logger:       logger,
		maxWorkers:   maxWorkers,
	}, nil
}



// InitializePool ensures the correct number of containers are running
func (cm *ContainerManager) InitializePool() error {
	containers, err := cm.dockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		cm.logger.Errorf("failed to list containers: %v", err)
		return fmt.Errorf("failed to list containers: %v", err)
	}

	// Register existing worker containers
	for _, c := range containers {
		if c.Image == "worker" {
			state := StateIdle
			if c.State != "running" {
				state = StateError
			}
			cm.mu.Lock()
			cm.containers[c.ID] = &ContainerInfo{ID: c.ID, State: state}
			cm.mu.Unlock()
			cm.logger.Printf("Found existing worker container: %s (state: %s)", c.ID[:12], state)
		}
	}

	// Adjust container count
	currentCount := len(cm.containers)
	if currentCount > cm.maxWorkers {
		cm.logger.Printf("Found %d worker containers, removing excess...", currentCount)
		excess := currentCount - cm.maxWorkers
		cm.removeExcessContainers(excess)
	} else if currentCount < cm.maxWorkers {
		cm.logger.Printf("Only %d worker containers found, creating %d more...", currentCount, cm.maxWorkers-currentCount)
		for i := 0; i < cm.maxWorkers-currentCount; i++ {
			if err := cm.StartContainer(); err != nil {
				cm.logger.Printf("Failed to start container: %v", err)
			}
		}
	}

	if len(cm.containers) == 0 {
		return fmt.Errorf("failed to initialize container pool: no containers available")
	}
	return nil
}

// StartContainer creates and starts a new worker container
func (cm *ContainerManager) StartContainer() error {
	ctx := context.Background()

	cm.mu.Lock()
	if len(cm.containers) >= cm.maxWorkers {
		cm.mu.Unlock()
		cm.logger.Printf("Already have %d containers, not starting new one", len(cm.containers))
		return nil
	}
	cm.mu.Unlock()

	config := &container.Config{
		Image: "worker",
		Tty:   true,
	}
	// seccompProfile := `{
	// 	"defaultAction": "SCMP_ACT_ALLOW",
	// 	"architectures": ["SCMP_ARCH_X86_64"],
	// 	"syscalls": [
	// 		{ "names": ["setuid", "setgid", "kill", "clone", "fork", "vfork", "socket", "connect", "bind", "accept", "ptrace", "personality", "syslog", "sysctl"], "action": "SCMP_ACT_DENY" }
	// 	]
	// }`
	// if err := json.Unmarshal([]byte(seccompProfile), &map[string]interface{}{}); err != nil {
	// 	return fmt.Errorf("invalid seccomp profile: %v", err)
	// }

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   200 * 1024 * 1024, // 200MB
			NanoCPUs: 1000000000,        // 1 CPU
		},
		NetworkMode: "none",
	}

	resp, err := cm.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		cm.logger.Errorf("failed to create container: %v", err)
		return fmt.Errorf("failed to create container: %v", err)
	}

	if err := cm.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		cm.dockerClient.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		cm.logger.Errorf("failed to start container %s: %v", resp.ID[:12], err)
		return fmt.Errorf("failed to start container: %v", err)
	}

	cm.mu.Lock()
	cm.containers[resp.ID] = &ContainerInfo{ID: resp.ID, State: StateIdle}
	cm.mu.Unlock()
	cm.logger.Printf("Started new worker container: %s", resp.ID[:12])

	return nil
}

// RemoveContainer safely removes a container
func (cm *ContainerManager) RemoveContainer(containerID string) {
	ctx := context.Background()

	if err := cm.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		cm.logger.Printf("Failed to remove container %s: %v", containerID[:12], err)
	}

	cm.mu.Lock()
	delete(cm.containers, containerID)
	cm.mu.Unlock()
	cm.logger.Printf("Removed container: %s", containerID[:12])
}

// removeExcessContainers removes excess containers beyond maxWorkers
func (cm *ContainerManager) removeExcessContainers(count int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var toRemove []string
	for id := range cm.containers {
		if len(toRemove) < count {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		cm.RemoveContainer(id)
	}
}

// MonitorContainers runs a health check loop
func (cm *ContainerManager) MonitorContainers(wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cm.checkHealth()
	}
}

// checkHealth ensures container health and count
func (cm *ContainerManager) checkHealth() {
	ctx := context.Background()

	containers, err := cm.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		cm.logger.Printf("Failed to list containers: %v", err)
		return
	}

	runningWorkers := make(map[string]bool)
	for _, c := range containers {
		if c.Image == "worker" && c.State == "running" {
			runningWorkers[c.ID] = true
		}
	}

	cm.mu.Lock()
	var toRemove []string
	for id := range cm.containers {
		//add one more layer of health check using container inspect then check state then state.health = healthy
		if !runningWorkers[id] {
			cm.logger.Printf("Container %s not running, marking for removal", id[:12])
			toRemove = append(toRemove, id)
		}
	}
	currentCount := len(cm.containers) - len(toRemove)
	cm.mu.Unlock()

	for _, id := range toRemove {
		cm.RemoveContainer(id)
	}

	if currentCount < cm.maxWorkers {
		for i := 0; i < cm.maxWorkers-currentCount; i++ {
			if err := cm.StartContainer(); err != nil {
				cm.logger.Printf("Failed to start replacement container: %v", err)
			}
		}
	} else if len(runningWorkers) > cm.maxWorkers {
		excess := len(runningWorkers) - cm.maxWorkers
		cm.removeExcessContainers(excess)
	}
}

// GetAvailableContainer finds an idle container
func (cm *ContainerManager) GetAvailableContainer() (string, error) {
	const maxRetries = 10
	const retryDelay = 200 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		cm.mu.Lock()
		for id, info := range cm.containers {
			if info.State == StateIdle {
				info.State = StateBusy
				cm.mu.Unlock()
				return id, nil
			}
		}
		cm.mu.Unlock()
		time.Sleep(retryDelay)
	}
	return "", fmt.Errorf("no available containers after %d retries", maxRetries)
}

// SetContainerState updates the state of a container
func (cm *ContainerManager) SetContainerState(containerID string, state ContainerState) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if container, exists := cm.containers[containerID]; exists {
		container.State = state
	}
}

// Shutdown cleans up all containers
func (cm *ContainerManager) Shutdown() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	ctx := context.Background()

	for id := range cm.containers {
		cm.dockerClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
		cm.logger.Printf("Shutdown: Removed container %s", id[:12])
	}
	cm.containers = make(map[string]*ContainerInfo)
}

// ContainerCount returns the current number of containers
func (cm *ContainerManager) ContainerCount() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.containers)
}
