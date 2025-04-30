package executor

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/fatih/color"
	logrus "github.com/sirupsen/logrus"
)

// WorkerPool manages a pool of workers for code execution
type WorkerPool struct {
	jobs         chan Job
	containerMgr *ContainerManager
	logger       *logrus.Logger
	maxWorkers   int
	maxJobCount  int
	wg           sync.WaitGroup
	shutdownChan chan struct{}
}

// NewWorkerPool initializes a new worker pool
func NewWorkerPool(maxWorkers, maxJobCount int,memorylimit,cpunanolimit int64) (*WorkerPool, error) {
	containerMgr, err := NewContainerManager(maxWorkers,memorylimit,cpunanolimit)
	if err != nil {
		return nil, err
	}

	pool := &WorkerPool{
		jobs:         make(chan Job, maxJobCount),
		containerMgr: containerMgr,
		logger:       containerMgr.logger,
		maxWorkers:   maxWorkers,
		maxJobCount:  maxJobCount,
		shutdownChan: make(chan struct{}),
	}

	log.Print("Initializing pool")
	if err := containerMgr.InitializePool(); err != nil {
		return nil, err
	}

	pool.wg.Add(1)
	go containerMgr.MonitorContainers(&pool.wg)

	for i := 0; i < maxWorkers; i++ { // Fixed range loop to use index
		pool.wg.Add(1)
		go pool.worker(i + 1)
	}

	pool.logger.WithFields(logrus.Fields{
		"maxWorkers": maxWorkers,
	}).Info(color.GreenString("Initialized WorkerPool with %d workers", maxWorkers))
	return pool, nil
}

// worker processes jobs from the queue
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	p.logger.WithFields(logrus.Fields{
		"workerID": id,
	}).Info(color.GreenString("Worker %d started", id))

	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				p.logger.WithFields(logrus.Fields{
					"workerID": id,
				}).Info(color.GreenString("Worker %d shutting down due to closed channel", id))
				return
			}
			p.executeJob(id, job)
		case <-p.shutdownChan:
			p.logger.WithFields(logrus.Fields{
				"workerID": id,
			}).Info(color.GreenString("Worker %d received shutdown signal", id))
			return
		}
	}
}

// executeJob handles the execution of a single job
func (p *WorkerPool) executeJob(workerID int, job Job) {
	containerID, err := p.containerMgr.GetAvailableContainer()
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"workerID":    workerID,
			"error":       err,
			"containerID": "N/A",
		}).Error(color.RedString("Worker %d couldn't get container: %v", workerID, err))
		job.Result <- Result{Error: err}
		return
	}

	p.logger.WithFields(logrus.Fields{
		"workerID":    workerID,
		"containerID": containerID[:12],
	}).Info(color.GreenString("Worker %d executing in container %s", workerID, containerID[:12]))
	p.containerMgr.SetContainerState(containerID, StateBusy)

	start := time.Now()
	output, success, err := p.executeCode(containerID, job.Language, job.Code)
	duration := time.Since(start)

	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"containerID": containerID[:12],
			"language":    job.Language,
			"duration":    duration,
		}).Warn(color.YellowString("Worker %d job failed", workerID))
	} else {
		p.containerMgr.SetContainerState(containerID, StateIdle)
		p.logger.WithFields(logrus.Fields{
			"workerID":    workerID,
			"containerID": containerID[:12],
			"language":    job.Language,
			"duration":    duration,
			// "output":     output,
		}).Info(color.GreenString("Worker %d job completed in container %s (%dms)", workerID, containerID[:12], duration.Milliseconds()))
	}

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
		p.logger.WithFields(logrus.Fields{
			"containerID": containerID[:12],
			"language":    language,
		}).Error(color.RedString("Unsupported language %s in container %s", language, containerID[:12]))
		return "", false, fmt.Errorf("unsupported language: %s", language)
	}

	healthCheckCtx, healthCheckCancel := context.WithCancel(context.Background())
	defer healthCheckCancel()

	ctx, cancel := context.WithTimeout(healthCheckCtx, config.Timeout)
	defer cancel()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				p.logger.WithFields(logrus.Fields{
					"containerID": containerID[:12],
				}).Debug(color.GreenString("ctx done for container %s", containerID[:12]))
				return
			case <-healthCheckCtx.Done():
				p.logger.WithFields(logrus.Fields{
					"containerID": containerID[:12],
				}).Debug(color.GreenString("healthCheckCtx done for container %s", containerID[:12]))
				return
			case <-ticker.C:
				if p.containerMgr.CheckResourceOutsurge(containerID) {
					go p.containerMgr.RemoveContainer(containerID)
					cancel()
					return
					// healthCheckCancel()
					// cancel()
				}
			}
		}
	}()

	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", append([]string{"exec", containerID}, config.Args(code)...)...)
	cmd.Stdout = &output
	cmd.Stderr = &output

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"containerID": containerID[:12],
			"language":    language,
			"duration":    duration,
		}).Error(color.RedString("Execution error in container: %v",err))
		return output.String(), false, fmt.Errorf("execution error: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"containerID": containerID[:12],
		"language":    language,
		"duration":    duration,
	}).Debug(color.GreenString("Execution completed in container %s", containerID[:12]))
	return output.String(), true, nil
}

// ExecuteJob submits a job to the worker pool
func (p *WorkerPool) ExecuteJob(language, code string) Result {
	p.logger.WithFields(logrus.Fields{
		"language": language,
	}).Info(color.GreenString("Submitting job for %s", language))

	result := make(chan Result, 1)
	select {
	case p.jobs <- Job{Language: language, Code: code, Result: result}:
		return <-result
	default:
		p.logger.WithFields(logrus.Fields{
			"language":    language,
			"maxJobCount": p.maxJobCount,
		}).Warn(color.YellowString("Job queue full, rejecting %s job (max: %d)", language, p.maxJobCount))
		return Result{Error: fmt.Errorf("job queue full, max capacity: %d", p.maxJobCount)}
	}
}

// Shutdown gracefully stops the worker pool
func (p *WorkerPool) Shutdown() {
	p.logger.Info(color.GreenString("Shutting down worker pool..."))
	close(p.shutdownChan)
	close(p.jobs)
	p.containerMgr.Shutdown()
	p.wg.Wait()
	p.logger.Info(color.GreenString("Worker pool shutdown complete"))
}
