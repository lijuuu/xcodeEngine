
# Sandboxed Code Execution (SCE) for xcode

SCE is a service that allows you to execute code snippets in Python, Go,C++ and Node.js through a RESTful API. The service runs Docker containers for executing the provided code safely, ensuring isolation and security.

## Features

- Executes code snippets in Python, Go,C++ and Node.js via HTTP requests.
- Runs in isolated Docker containers for safety.
- Ratelimiting added

## API Endpoint

### POST /execute

#### Request Body

```json
{
"code":"Y29uc29sZS5sb2coJ0hlbGxvLCB3b3JsZCEnKTs=",//base64 encoded code
"language":"js"
}
```

#### Response

```json
{
  "output": "Hello, world!\n",
  "status_message": "Success",
  "execution_time": "270.48344ms"
}
```

