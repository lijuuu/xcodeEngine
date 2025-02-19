package executor

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	container "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type execConfig struct {
	timeout time.Duration
	args    func(code string) []string
}

var configs = map[string]execConfig{
	"go": {
		timeout: 10 * time.Second,
		args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > main.go && go run main.go || go run main.go`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
	"js": {
		timeout: 5 * time.Second,
		args: func(code string) []string {
			return []string{"node", "-e", code}
		},
	},
	"python": {
		timeout: 5 * time.Second,
		args: func(code string) []string {
			return []string{"python3", "-c", code}
		},
	},
	"cpp": {
		timeout: 5 * time.Second,
		args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > prog.cpp && g++ -o prog prog.cpp && ./prog || ./prog`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
}

// Job represents an execution request.
type Job struct {
	Container string
	Language  string
	Code      string
	Result    chan Result
}

type Result struct {
	Output string
	Error  error
}

type ContainerState string

const (
	StateIdle    ContainerState = "idle"
	StateWorking ContainerState = "working"
	StateError   ContainerState = "error"
)

type ContainerInfo struct {
	ID    string
	State ContainerState
}

// WorkerPool manages concurrent execution jobs.
type WorkerPool struct {
	jobs         chan Job
	numWorkers   int
	wg           sync.WaitGroup
	mu           sync.Mutex
	dockerClient *client.Client
}

// NewWorkerPool initializes the worker pool with a given number of workers.
func NewWorkerPool(numWorkers int) *WorkerPool {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	pool := &WorkerPool{
		jobs:         make(chan Job, 10),
		numWorkers:   numWorkers,
		dockerClient: dockerClient,
	}

	// Start containers for each worker
	for i := 0; i < numWorkers; i++ {
		if err := pool.startContainer(); err != nil {
			log.Fatalf("Failed to start container for worker %d: %v", i, err)
		}
	}

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		go pool.worker(i)
	}

	// Remove excess workers if any
	if err := pool.RemoveExcessWorkers(); err != nil {
		log.Printf("Failed to remove excess workers: %v", err)
	}

	// Start container health check
	go pool.monitorContainers()

	return pool
}

// Log file for Docker operations
var dockerLogFile *os.File

// Initialize the Docker log file
func init() {
	var err error
	dockerLogFile, err = os.OpenFile("/logs/container.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open Docker log file: %v", err)
	}
}

func (p *WorkerPool) startContainer() error {
	ctx := context.Background()

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
		logDockerOperation("Failed to create container: %v", err)
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := p.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		logDockerOperation("Failed to start container: %v", err)
		return fmt.Errorf("failed to start container: %w", err)
	}

	logDockerOperation("Started container: %s", resp.ID)
	return nil
}

// monitorContainers checks the number of running worker containers every 2 seconds.
func (p *WorkerPool) monitorContainers() {
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
	defer ticker.Stop()

	for {
		<-ticker.C
		containers, err := p.dockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
		if err != nil {
			log.Printf("Failed to list containers: %v", err)
			continue
		}

		var runningWorkers []string
		for _, container := range containers {
			if container.State == "running" && container.Image == "worker" {
				runningWorkers = append(runningWorkers, container.ID)
			}
		}

		runningCount := len(runningWorkers)

		if runningCount == 0 {
			logDockerOperation("⚠️ No running worker containers found. Creating a new container.")
			if err := p.startContainer(); err != nil {
				logDockerOperation("❌ Failed to create a new worker container: %v", err)
			}
		} else if runningCount > 2 {
			logDockerOperation("⚠️ More than 2 running worker containers found: %d. Removing excess containers.", runningCount)
			for _, containerID := range runningWorkers[2:] { // Keep only the first 2
				if err := p.dockerClient.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true}); err != nil {
					logDockerOperation("❌ Failed to remove container %s: %v", containerID, err)
				} else {
					logDockerOperation("✅ Removed excess container: %s", containerID)
				}
			}
		} else {
			logDockerOperation("✅ Running worker containers count is sufficient: %d", runningCount)
		}
	}
}

// worker listens for jobs and processes them.
func (p *WorkerPool) worker(id int) {
	for job := range p.jobs {
		containerId, err := p.GetAvailableContainer()
		if err != nil {
			job.Result <- Result{Error: fmt.Errorf("no available containers: %v", err)}
			continue
		}

		fmt.Printf("Worker %d processing job\n", id)
		output, err := Execute(containerId, job.Language, job.Code)
		job.Result <- Result{Output: output, Error: err}
	}
}

// ExecuteJob submits a job to the worker pool and waits for a result.
func (p *WorkerPool) ExecuteJob(language, code string) Result {
	result := make(chan Result, 1)
	job := Job{Language: language, Code: code, Result: result}

	p.jobs <- job
	return <-result // Return the Result struct containing both output and error
}

// Shutdown gracefully stops all workers and logs the operation.
func (p *WorkerPool) Shutdown() {
	close(p.jobs)
	p.wg.Wait()
}

// Execute runs the code inside a container and returns the output.
func Execute(containerName, language, code string) (string, error) {
	config, ok := configs[language]
	if !ok {
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()

	args := append([]string{"exec", containerName}, config.args(code)...)
	cmd := exec.CommandContext(ctx, "docker", args...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	start := time.Now()
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("execution timed out after %v", config.timeout)
		}
		return output.String(), fmt.Errorf("execution error: %w", err)
	}
	duration := time.Since(start)
	fmt.Println("Execution time:", duration)
	return output.String(), nil
}

// RemoveExcessWorkers removes orphan containers with the image "worker" if there are more than 2.
func (p *WorkerPool) RemoveExcessWorkers() error {
	ctx := context.Background()
	containers, err := p.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		logDockerOperation("Failed to list containers: %v", err)
		return fmt.Errorf("failed to list containers: %w", err)
	}

	var workerContainers []string
	for _, container := range containers {
		if container.Image == "worker" {
			workerContainers = append(workerContainers, container.ID)
			logDockerOperation("Found worker container: %s", container.ID)
		}
	}

	if len(workerContainers) == 0 {
		logDockerOperation("⚠️ No worker containers found.")
	} else if len(workerContainers) > 2 {
		logDockerOperation("More than 2 worker containers found: %d. Removing excess containers.", len(workerContainers))
		for _, containerID := range workerContainers[2:] { // Keep only the first 2
			if err := p.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
				logDockerOperation("❌ Failed to remove container %s: %v", containerID, err)
			} else {
				logDockerOperation("✅ Removed excess container: %s", containerID)
			}
		}
	} else {
		logDockerOperation("✅ Worker container count is within limit: %d", len(workerContainers))
	}

	return nil
}

// GetAvailableContainer dynamically finds an available container.
func (p *WorkerPool) GetAvailableContainer() (string, error) {
	ctx := context.Background()
	containers, err := p.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		if container.State == "running" && container.Image == "worker" {
			return container.ID, nil
		}
	}
	return "", fmt.Errorf("no available containers")
}

// LogDockerOperation logs Docker-related operations to the Docker log file.
func logDockerOperation(format string, args ...interface{}) {
	logMessage := fmt.Sprintf(format, args...)
	log.Printf(logMessage) // Log to console
	if dockerLogFile != nil {
		dockerLogFile.WriteString(fmt.Sprintf("%s: %s\n", time.Now().Format(time.RFC3339), logMessage))
	}
}
