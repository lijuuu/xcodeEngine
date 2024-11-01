
# Sandboxed Code Execution (SCE)

SCE is a service that allows you to execute code snippets in Python, Go, and Node.js through a RESTful API. The service runs Docker containers for executing the provided code safely, ensuring isolation and security.

## Features

- Executes code snippets in Python, Go, and Node.js via HTTP requests.
- Runs in isolated Docker containers for safety.
- Ratelimiting added

## API Endpoint

### POST /execute

#### Request Body

```json
{
    "language": "python|go|nodejs",
    "code": "your code here",
    "method":"docker"
}
```

#### Response

```json
{
    "output": "output of the execution",
    "error": "error message if any",
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

#### For Node.js Executor

1. Build the Node.js Docker image:

   ```bash
   docker build -t nodejs-executor -f nodejs.Dockerfile .
   ```

### Running the Service with Docker Compose

1. Make sure you have Docker and Docker Compose installed.
2. Run the following command to build and start the services:

   ```bash
   docker-compose up --build
   ```

3. You can now send requests to `http://localhost:8080/execute` using tools like `curl` or Postman.

## Example Usage

### To execute Python code:

```bash
curl -X POST http://localhost:8080/execute -d '{"language": "python", "code": "print(\"Hello from Python!\")"}' -H "Content-Type: application/json"
```

### To execute Go code:

```bash
curl -X POST http://localhost:8080/execute -d '{"language": "go", "code": "package main; import \"fmt\"; func main() { fmt.Println(\"Hello from Go!\") }"}' -H "Content-Type: application/json"
```

### To execute Node.js code:

```bash
curl -X POST http://localhost:8080/execute -d '{"language": "nodejs", "code": "console.log(\"Hello from Node.js!\");"}' -H "Content-Type: application/json"
```

## API Request Collection

You can find predefined requests in the `apirequest.json` file. Here are some examples included in the collection:

### Golang Execution Request

```json
{
  "language": "go",
  "code": "package main\nimport \"fmt\"\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n    a := 5\n    b := 3\n    sum := a + b\n    fmt.Printf(\"The sum of %d and %d is %d\\n\", a, b, sum)\n}",
  "method":"docker"
}
```

### Python Execution Request

```json
{
  "language": "python",
  "code": "a = 5\nb = 3\nprint(\"Hello, World!\")\nprint(f\"The sum of {a} and {b} is {a + b}\")",
  "method":"docker"
}
```

### Node.js Execution Request

```json
{
  "language": "nodejs",
  "code": "console.log('Hello, World!');\nconst a = 5;\nconst b = 3;\nconst sum = a + b;\nconsole.log(`The sum of ${a} and ${b} is ${sum}`);",
  "method":"docker"
}
```

---
