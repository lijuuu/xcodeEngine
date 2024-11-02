package lang

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func ExecuteJsCode(containerName, code string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	
	cmd := exec.CommandContext(ctx, "docker", "exec", containerName, "node", "-e", code)
	output, err := cmd.CombinedOutput()

	
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("execution timed out")
	}

	if err != nil {
		return string(output), err
	}
	return string(output), nil
}
