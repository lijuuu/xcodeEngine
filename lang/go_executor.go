package lang

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func ExecuteGoCode(containerName, code string) (string, error) {
	tempFile, err := os.CreateTemp("", "main*.go")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(code); err != nil {
		return "", fmt.Errorf("failed to write code to file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	
	copyCmd := exec.Command("docker", "cp", tempFile.Name(), fmt.Sprintf("%s:/app/main.go", containerName))
	if err := copyCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to copy code to container: %w", err)
	}

	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	
	runCmd := exec.CommandContext(ctx, "docker", "exec", containerName, "go", "run", "/app/main.go")
	output, err := runCmd.CombinedOutput()

	
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("execution timed out")
	}

	if err != nil {
		return string(output), err
	}

	return string(output), nil
}
