# Isolated Code Execution(ICE)

This service allows you to execute code snippets in Python and Go through a RESTful API. The service runs Docker containers for executing the provided code safely.

## Features

- Executes Python and Go code via HTTP requests.
- Returns execution output, execution time, and memory usage.
- Runs in isolated Docker containers for safety.

## API Endpoint

### POST /execute

#### Request Body

```json
{
    "language": "python|go",
    "code": "your code here"
}
```

#### Response

```json
{
    "output": "output of the execution",
    "error": "error message if any",
    "execution_time": "time taken in seconds",
    "memory_usage": "memory used in bytes",
    "execution_time_unit": "seconds",
    "memory_usage_unit": "bytes"
}
```

## Docker Setup

### Build the Docker Images

#### For Python Executor

1. Build the Python Docker image:

   ```bash
   docker build -t python-executor -f python.Dockerfile .
   ```

#### For Go Executor

1. Build the Go Docker image:

   ```bash
   docker build -t go-executor -f go.Dockerfile .
   ```

## Running the Service

1. Build the Go application:

   ```bash
   go build -o server main.go
   ```

2. Run the server:

   ```bash
   ./server
   ```

3. You can now send requests to `http://localhost:8080/execute` using tools like `curl` or Postman.

## Example Usage

To execute Python code:

```bash
curl -X POST http://localhost:8080/execute -d '{"language": "python", "code": "print(\"Hello from Python!\")"}' -H "Content-Type: application/json"
```

To execute Go code:

```bash
curl -X POST http://localhost:8080/execute -d '{"language": "go", "code": "package main; import \"fmt\"; func main() { fmt.Println(\"Hello from Go!\") }"}' -H "Content-Type: application/json"
```

---

### TODO

- Add a message broker for handling code execution requests.
- Implement Kubernetes orchestration for scaling and managing containerized executions.
