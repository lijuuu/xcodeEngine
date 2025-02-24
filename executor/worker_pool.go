package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	logrus "github.com/sirupsen/logrus"
)

// WorkerPool manages a pool of workers for code execution
type WorkerPool struct {
	jobs          chan Job
	containerMgr  *ContainerManager
	logger        *logrus.Logger
	maxWorkers    int
	maxJobCount   int
	wg            sync.WaitGroup
	shutdownChan  chan struct{}
}

// NewWorkerPool initializes a new worker pool
func NewWorkerPool(maxWorkers, maxJobCount int) (*WorkerPool, error) {
	containerMgr, err := NewContainerManager(maxWorkers)
	if err != nil {
		return nil, err
	}

	pool := &WorkerPool{
		jobs:          make(chan Job, maxJobCount),
		containerMgr:  containerMgr,
		logger:        containerMgr.logger,
		maxWorkers:    maxWorkers,
		maxJobCount:   maxJobCount,
		shutdownChan:  make(chan struct{}),
	}

	if err := containerMgr.InitializePool(); err != nil {
		return nil, err
	}

	pool.wg.Add(1)
	go containerMgr.MonitorContainers(&pool.wg)

	for i := 0; i < maxWorkers; i++ {
		pool.wg.Add(1)
		go pool.worker(i + 1)
	}

	return pool, nil
}

// worker processes jobs from the queue
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	p.logger.Printf("Worker %d started", id)

	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				p.logger.Printf("Worker %d shutting down due to closed channel", id)
				return
			}
			p.executeJob(id, job)
		case <-p.shutdownChan:
			p.logger.Printf("Worker %d received shutdown signal", id)
			return
		}
	}
}

// executeJob handles the execution of a single job
func (p *WorkerPool) executeJob(workerID int, job Job) {
	containerID, err := p.containerMgr.GetAvailableContainer()
	if err != nil {
		p.logger.Printf("Worker %d couldn't get container: %v", workerID, err)
		job.Result <- Result{Error: err}
		return
	}

	p.logger.Printf("Worker %d executing in container %s", workerID, containerID[:12])
	p.containerMgr.SetContainerState(containerID, StateBusy)

	start := time.Now()
	output, success, err := p.executeCode(containerID, job.Language, job.Code)
	duration := time.Since(start)

	p.containerMgr.SetContainerState(containerID, StateIdle)
	job.Result <- Result{
		Output:        output,
		Success:       success,
		Error:         err,
		ExecutionTime: fmt.Sprintf("%dms", duration.Milliseconds()),
	}
}

// executeCode runs code in a container
func (p *WorkerPool) executeCode(containerID, language, code string) (string, bool, error) {
	config, ok := GetLanguageConfig(language)
	if !ok {
		return "", false, fmt.Errorf("unsupported language: %s", language)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", append([]string{"exec", containerID}, config.Args(code)...)...)
	cmd.Stdout = &output
	cmd.Stderr = &output

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		p.logger.WithFields(logrus.Fields{
			"container": containerID[:12],
			"language":  language,
			"duration":  duration,
		}).Warn("Execution timeout, removing and replacing container")
		p.containerMgr.RemoveContainer(containerID)
		p.containerMgr.StartContainer()
		return "", false, fmt.Errorf("timeout after %v", config.Timeout)
	}

	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"container": containerID[:12],
			"language":  language,
			"duration":  duration,
			"error":     err,
		}).Error("Execution failed")
		return output.String(), false, fmt.Errorf("execution error: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"container": containerID[:12],
		"language":  language,
		"duration":  duration,
	}).Debug("Execution completed")
	return output.String(), true, nil
}

// ExecuteJob submits a job to the worker pool
func (p *WorkerPool) ExecuteJob(language, code string) Result {
	result := make(chan Result, 1)
	select {
	case p.jobs <- Job{Language: language, Code: code, Result: result}:
		return <-result
	default:
		return Result{Error: fmt.Errorf("job queue full, max capacity: %d", p.maxJobCount)}
	}
}

// Shutdown gracefully stops the worker pool
func (p *WorkerPool) Shutdown() {
	p.logger.Println("Shutting down worker pool...")
	close(p.shutdownChan)
	close(p.jobs)
	p.containerMgr.Shutdown()
	p.wg.Wait()
	p.logger.Println("Worker pool shutdown complete")
}