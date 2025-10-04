package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
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
	memorylimit  int64
	cpunanolimit int64
}

// NewContainerManager creates a new container manager
func NewContainerManager(maxWorkers int, memorylimit, cpunanolimit int64) (*ContainerManager, error) {
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.45"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}

	logger := logrus.New()

	// Use a standard log directory
	logDir := "/var/log/engine"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Open or create the log file
	logFile, err := os.OpenFile(filepath.Join(logDir, "container.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// Multi-output: file and colored terminal
	logger.SetOutput(os.Stdout)              // Default to stdout
	logger.AddHook(&fileHook{file: logFile}) // Hook for file logging
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true, // Enable colors in terminal
		FullTimestamp: true,
	})

	return &ContainerManager{
		dockerClient: dockerClient,
		containers:   make(map[string]*ContainerInfo),
		logger:       logger,
		maxWorkers:   maxWorkers,
		memorylimit:  memorylimit,
		cpunanolimit: cpunanolimit,
	}, nil
}

// fileHook is a custom logrus hook to write logs to a file
type fileHook struct {
	file *os.File
}

func (h *fileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *fileHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	_, err = h.file.WriteString(line)
	return err
}

// InitializePool ensures the correct number of containers are running
func (cm *ContainerManager) InitializePool() error {
	containers, err := cm.dockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		cm.logger.WithFields(logrus.Fields{"error": err}).Error("Failed to list containers")
		return fmt.Errorf("failed to list containers: %v", err)
	}

	// Register existing worker containers
	for _, c := range containers {
		if c.Image == "lijuthomas/worker" {
			state := StateIdle
			if c.State != "running" {
				state = StateError
			}
			cm.mu.Lock()
			cm.containers[c.ID] = &ContainerInfo{ID: c.ID, State: state}
			cm.mu.Unlock()
			cm.logger.WithFields(logrus.Fields{
				"container_id": c.ID[:12],
				"state":        state,
			}).Info(color.GreenString("Found existing worker container"))
		}
	}

	// Adjust container count
	currentCount := len(cm.containers)
	if currentCount > cm.maxWorkers {
		cm.logger.WithFields(logrus.Fields{"count": currentCount}).Warn(color.YellowString("Found %d worker containers, removing excess...", currentCount))
		excess := currentCount - cm.maxWorkers
		cm.removeExcessContainers(excess)
	} else if currentCount < cm.maxWorkers {
		cm.logger.WithFields(logrus.Fields{
			"current": currentCount,
			"needed":  cm.maxWorkers - currentCount,
		}).Info(color.GreenString("Only %d worker containers found, creating %d more...", currentCount, cm.maxWorkers-currentCount))
		for i := 0; i < cm.maxWorkers-currentCount; i++ {
			if err := cm.StartContainer(); err != nil {
				cm.logger.WithFields(logrus.Fields{"error": err}).Error(color.RedString("Failed to start container"))
			}
		}
	}

	if len(cm.containers) == 0 {
		cm.logger.Error(color.RedString("Failed to initialize container pool: no containers available"))
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
		cm.logger.WithFields(logrus.Fields{"count": len(cm.containers)}).Warn(color.YellowString("Already have %d containers, not starting new one", len(cm.containers)))
		return nil
	}
	cm.mu.Unlock()

	config := &container.Config{
		Image: "lijuthomas/worker",
		Tty:   true,
	}

	// 	seccompProfile := `{
	//     "defaultAction": "SCMP_ACT_ERRNO",
	//     "architectures": ["SCMP_ARCH_X86_64"],
	//     "syscalls": [
	//         {
	//             "names": [
	//                 "fork", "vfork", "clone"
	//             ],
	//             "action": "SCMP_ACT_TRACE"
	//         }
	//     ]
	// }
	// `

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   (cm.memorylimit) * 1024 * 1024,
			NanoCPUs: (cm.cpunanolimit) * 1000_000,
			// PidsLimit: &[]int64{20}[0],
			// Ulimits:   []*container.Ulimit{{Name: "nproc", Hard: 70, Soft: 70}},
		},
		NetworkMode: "none",
		// SecurityOpt: []string{"seccomp=" + seccompProfile},
	}

	resp, err := cm.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		cm.logger.WithFields(logrus.Fields{"error": err}).Error(color.RedString("Failed to create container"))
		return fmt.Errorf("failed to create container: %v", err)
	}

	if err := cm.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		cm.dockerClient.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		cm.logger.WithFields(logrus.Fields{
			"container_id": resp.ID[:12],
			"error":        err,
		}).Error(color.RedString("Failed to start container"))
		return fmt.Errorf("failed to start container: %v", err)
	}

	cm.mu.Lock()
	cm.containers[resp.ID] = &ContainerInfo{ID: resp.ID, State: StateIdle}
	cm.mu.Unlock()
	cm.logger.WithFields(logrus.Fields{
		"container_id": resp.ID[:12],
	}).Info(color.GreenString("Started new worker container"))

	return nil
}

// RemoveContainer safely removes a container
func (cm *ContainerManager) RemoveContainer(containerID string) {
	ctx := context.Background()

	if err := cm.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		cm.logger.WithFields(logrus.Fields{
			"container_id": containerID[:12],
			"error":        err,
		}).Error(color.RedString("Failed to remove container"))
	}

	delete(cm.containers, containerID)
	cm.logger.WithFields(logrus.Fields{
		"container_id": containerID[:12],
	}).Info(color.GreenString("Removed container"))
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
		cm.logger.WithFields(logrus.Fields{"error": err}).Error(color.RedString("Failed to list containers"))
		return
	}

	cm.logger.WithFields(logrus.Fields{"count": len(containers)}).Debug("Checking container health")

	runningWorkers := make(map[string]bool)
	for _, c := range containers {
		if c.Image == "lijuthomas/worker" {
			if _, exists := cm.containers[c.ID]; exists {
				if cm.containers[c.ID].State != StateError {
					runningWorkers[c.ID] = true
				}
			}
		}
	}

	// fmt.Print("length : ", len(containers), " workers : ", len(runningWorkers), " \n")

	cm.mu.Lock()
	var toRemove []string
	for id := range cm.containers {
		if !runningWorkers[id] {
			cm.logger.WithFields(logrus.Fields{
				"container_id": id[:12],
			}).Warn(color.YellowString("Container not running, marking for removal"))
			toRemove = append(toRemove, id)
		}
	}
	currentCount := len(cm.containers) - len(toRemove)
	cm.mu.Unlock()

	for _, id := range toRemove {
		cm.RemoveContainer(id)
	}

	if currentCount < cm.maxWorkers {
		cm.logger.WithFields(logrus.Fields{
			"current": currentCount,
			"needed":  cm.maxWorkers - currentCount,
		}).Info(color.GreenString("Starting %d replacement containers", cm.maxWorkers-currentCount))
		for i := 0; i < cm.maxWorkers-currentCount; i++ {
			if err := cm.StartContainer(); err != nil {
				cm.logger.WithFields(logrus.Fields{"error": err}).Error(color.RedString("Failed to start replacement container"))
			}
		}
	} else if len(runningWorkers) > cm.maxWorkers {
		excess := len(runningWorkers) - cm.maxWorkers
		cm.logger.WithFields(logrus.Fields{"excess": excess}).Warn(color.YellowString("Removing %d excess containers", excess))
		cm.removeExcessContainers(excess)
	}
}

// GetAvailableContainer finds an idle container
func (cm *ContainerManager) GetAvailableContainer() (string, error) {
	const maxRetries = 10
	const retryDelay = 200 * time.Millisecond

	cm.mu.Lock()
	for i := 0; i < maxRetries; i++ {

		for id, info := range cm.containers {
			if info.State == StateIdle {
				info.State = StateBusy
				cm.mu.Unlock()
				cm.logger.WithFields(logrus.Fields{
					"container_id": id[:12],
				}).Info(color.GreenString("Assigned container to job"))
				return id, nil
			}
		}
		time.Sleep(retryDelay)
	}
	cm.mu.Unlock()
	cm.logger.WithFields(logrus.Fields{"retries": maxRetries}).Error(color.RedString("No available containers after %d retries", maxRetries))
	return "", fmt.Errorf("no available containers after %d retries", maxRetries)
}

// SetContainerState updates the state of a container
func (cm *ContainerManager) SetContainerState(containerID string, state ContainerState) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if container, exists := cm.containers[containerID]; exists {
		container.State = state
		cm.logger.WithFields(logrus.Fields{
			"container_id": containerID[:12],
			"state":        state,
		}).Info(color.GreenString("Updated container state"))
	}
}

// Shutdown cleans up all containers
func (cm *ContainerManager) Shutdown() {
	//grab list of containers under lock, then release lock
	cm.mu.Lock()
	containers := make([]string, 0, len(cm.containers))
	for id := range cm.containers {
		containers = append(containers, id)
	}
	cm.mu.Unlock()

	ctx := context.Background()
	for _, id := range containers {
		//remove container safely without holding lock
		if err := cm.dockerClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
			cm.logger.WithFields(logrus.Fields{
				"container_id": id[:12],
				"error":        err,
			}).Error(color.RedString("Failed to remove container during shutdown"))
			continue
		}
		cm.logger.WithFields(logrus.Fields{
			"container_id": id[:12],
		}).Info(color.GreenString("Shutdown: Removed container"))
	}

	// finally, clear the map safely
	cm.mu.Lock()
	cm.containers = make(map[string]*ContainerInfo)
	cm.mu.Unlock()

	cm.logger.Info(color.GreenString("Shutdown complete"))
}

// ContainerCount returns the current number of containers
func (cm *ContainerManager) ContainerCount() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.containers)
}

func (cm *ContainerManager) CheckResourceOutsurge(containerID string) bool {
	ctx := context.Background()
	info, err := cm.dockerClient.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		cm.logger.WithFields(logrus.Fields{"error": err}).Error(color.RedString("Failed to inspect container"))
		return false
	}
	defer info.Body.Close()

	//HOTFIX: read all bytes once, avoid streaming decode
	data, err := io.ReadAll(info.Body)
	if err != nil {
		cm.logger.WithFields(logrus.Fields{"error": err}).Error(color.RedString("Failed to read container stats"))
		return false
	}

	//HOTFIX: only parse necessary fields for CPU/memory
	var stats struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"cpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
	}

	if err := json.Unmarshal(data, &stats); err != nil {
		cm.logger.WithFields(logrus.Fields{"error": err}).Error(color.RedString("Failed to unmarshal stats"))
		return false
	}

	//Calculate CPU percentage
	cpuPercent := 0.0
	if stats.CPUStats.SystemCPUUsage > 0 {
		cpuPercent = (float64(stats.CPUStats.CPUUsage.TotalUsage) / float64(stats.CPUStats.SystemCPUUsage)) * 100
	}

	//Calculate memory percentage
	memPercent := 0.0
	if stats.MemoryStats.Limit > 0 {
		memPercent = (float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit)) * 100
	}

	//TAGGED HOTFIX: threshold check
	if cpuPercent > 99.0 || memPercent > 99.0 {
		cm.logger.WithFields(logrus.Fields{
			"container_id":   containerID[:12],
			"cpu_percent":    fmt.Sprintf("%.2f%%", cpuPercent),
			"memory_percent": fmt.Sprintf("%.2f%%", memPercent),
		}).Error(color.MagentaString("Resource outsurge detected [TAG:resource-hotfix]"))
		return true
	}

	return false
}
