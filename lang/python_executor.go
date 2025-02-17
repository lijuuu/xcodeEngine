package lang

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

func ExecutePythonCode(containerName, code string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "exec", containerName, "python3", "-c", code)
	output := &bytes.Buffer{}
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("execution timed out after 5 seconds")
		}
		return output.String(), fmt.Errorf("execution error: %w", err)
	}

	return output.String(), nil
}
