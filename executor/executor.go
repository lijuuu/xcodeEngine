package executor

import (
	"bytes"
	"context"
	"fmt"
	"log"
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
	jobs           chan Job
	numWorkers     int
	wg             sync.WaitGroup
	mu             sync.Mutex
	dockerClient   *client.Client
	containers     []ContainerInfo
	containerMutex sync.RWMutex
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
		containers:   make([]ContainerInfo, numWorkers),
	}

	// Start containers for each worker
	for i := 0; i < numWorkers; i++ {
		containerId, err := pool.startContainer()
		if err != nil {
			log.Fatalf("Failed to start container for worker %d: %v", i, err)
		}
		pool.containers[i] = ContainerInfo{
			ID:    containerId,
			State: StateIdle,
		}
	}

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		go pool.worker(i)
	}

	// Start container health check
	go pool.monitorContainers()

	return pool
}

func (p *WorkerPool) startContainer() (string, error) {
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
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := p.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

func (p *WorkerPool) getAvailableContainer() (string, error) {
	p.containerMutex.Lock()
	defer p.containerMutex.Unlock()

	for i, container := range p.containers {
		if container.State == StateIdle {
			p.containers[i].State = StateWorking
			return container.ID, nil
		}
	}
	return "", fmt.Errorf("no available containers")
}

func (p *WorkerPool) releaseContainer(containerId string) {
	p.containerMutex.Lock()
	defer p.containerMutex.Unlock()

	for i, container := range p.containers {
		if container.ID == containerId {
			p.containers[i].State = StateIdle
			return
		}
	}
}

func (p *WorkerPool) monitorContainers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		p.containerMutex.Lock()
		for i, container := range p.containers {
			ctx := context.Background()

			inspect, err := p.dockerClient.ContainerInspect(ctx, container.ID)
			if err != nil || !inspect.State.Running {
				log.Printf("Container %s is not healthy, starting new container", container.ID)

				_ = p.dockerClient.ContainerRemove(ctx, container.ID, container.RemoveOptions{Force: true})

				newContainerId, err := p.startContainer()
				if err != nil {
					log.Printf("Failed to start replacement container: %v", err)
					p.containers[i].State = StateError
					continue
				}
				p.containers[i] = ContainerInfo{
					ID:    newContainerId,
					State: StateIdle,
				}
			}
		}
		p.containerMutex.Unlock()
	}
}

// worker listens for jobs and processes them.
func (p *WorkerPool) worker(id int) {
	for job := range p.jobs {
		containerId, err := p.getAvailableContainer()
		if err != nil {
			job.Result <- Result{Error: fmt.Errorf("no available containers: %v", err)}
			continue
		}

		fmt.Printf("Worker %d processing job\n", id)
		output, err := Execute(containerId, job.Language, job.Code)
		p.releaseContainer(containerId)
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

// Shutdown gracefully stops all workers.
func (p *WorkerPool) Shutdown() {
	close(p.jobs)
	p.wg.Wait()

	ctx := context.Background()
	p.containerMutex.Lock()
	defer p.containerMutex.Unlock()

	for _, container := range p.containers {
		err := p.dockerClient.ContainerRemove(ctx, container.ID, container.RemoveOptions{Force: true})
		if err != nil {
			log.Printf("Failed to remove container %s: %v", container.ID, err)
		}
	}
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
