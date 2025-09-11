# Code Execution Engine

A high-performance, containerized code execution service that provides secure, isolated execution of code snippets across multiple programming languages. Built with Go and Docker, it uses NATS messaging for distributed communication and implements a worker pool architecture for optimal resource management.

## Architecture

The engine follows a microservices architecture with the following components:

- **NATS Message Handler**: Processes execution requests from message queues
- **Worker Pool**: Manages concurrent code execution with configurable limits
- **Container Manager**: Handles Docker container lifecycle and resource monitoring
- **Service Layer**: Provides code compilation and execution logic with language normalization

## Tech Stack

- **Runtime**: Go 1.24
- **Containerization**: Docker with isolated execution environments
- **Messaging**: NATS for asynchronous communication
- **Logging**: Zap logger with BetterStack integration
- **Monitoring**: Resource usage tracking and container health checks

## Supported Languages

- **JavaScript/Node.js** (`js`, `javascript`)
- **Python** (`python`, `py`)
- **Go** (`go`, `golang`)
- **C++** (`cpp`, `c++`)

## Internal Workflow

### 1. Message Processing
- Service subscribes to NATS subjects: `compiler.execute.request` and `problems.execute.request`
- Incoming requests are deserialized and validated
- Code is decoded from base64 and sanitized for security

### 2. Worker Pool Execution
- Jobs are queued in a buffered channel with configurable capacity
- Worker goroutines process jobs concurrently
- Each worker requests an available container from the pool

### 3. Container Management
- Pre-warmed Docker containers (`lijuthomas/worker`) are maintained in a pool
- Containers are assigned to jobs and monitored for resource usage
- Automatic container replacement when limits are exceeded or containers fail
- Resource limits: 400MB memory, 500 CPU nano-cores per container

### 4. Code Execution
- Code is written to temporary files within containers
- Language-specific compilation and execution commands are run
- Output is captured with 10-second timeout per execution
- Resource monitoring prevents container abuse

### 5. Response Handling
- Execution results are serialized and published back via NATS
- Includes output, error messages, execution time, and success status

## NATS Integration

The service operates as a NATS subscriber, processing two types of requests:

### Compiler Requests (`compiler.execute.request`)
Standard code compilation and execution with base64-encoded input.

### Problem Execution (`problems.execute.request`)
Extended execution for coding problems with relaxed input limits.

**Message Flow:**
```
Client → NATS → Code Engine → Docker Container → NATS → Client
```

## Configuration

Environment variables:

```bash
NATSURL=nats://localhost:4222
ENVIRONMENT=production
BETTERSTACKUPLOADURL=<logging_endpoint>
BETTERSTACKSOURCETOKEN=<logging_token>
```

## Container Requirements

- Docker daemon must be running
- Worker image `lijuthomas/worker` must be available locally
- Network isolation enabled for security

## Resource Management (default)

- **Worker Pool**: 2 workers, 3 concurrent jobs
- **Memory Limit**: 400MB per container
- **CPU Limit**: 500 nano-cores per container
- **Execution Timeout**: 10 seconds per job
- **Health Monitoring**: 1-second intervals

## Security Features

- Network isolation for execution containers
- Resource limits to prevent abuse
- Code sanitization and validation
- Automatic container cleanup and replacement

## Deployment

### Docker Build
```bash
docker build -t code-execution-engine .
```

### Local Development
```bash
go mod download
go run cmd/main.go
```

### Production
The service runs as a long-lived process, automatically managing container pools and processing NATS messages.

