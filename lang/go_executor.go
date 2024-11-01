package lang

import (
	"fmt"
	"os"
	"os/exec"
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

	
	runCmd := exec.Command("docker", "exec", containerName, "go", "run", "/app/main.go")
	output, err := runCmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}

	return string(output), nil
}
