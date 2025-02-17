package lang

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func ExecuteGoCode(containerName, code string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Modified build command to properly suppress all build output
	cmdStr := fmt.Sprintf(`cd /app/temp && 
		echo '%s' > main.go && 
		go build -o prog main.go && 
		./prog && 
		rm -f main.go prog`,
		strings.ReplaceAll(code, "'", "'\\''"))

	cmd := exec.CommandContext(ctx, "docker", "exec", containerName, "sh", "-c", cmdStr)

	output := &bytes.Buffer{}
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("execution timed out")
		}
		return output.String(), fmt.Errorf("execution error: %w", err)
	}

	return output.String(), nil
}
