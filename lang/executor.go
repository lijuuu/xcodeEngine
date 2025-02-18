package lang

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type execConfig struct {
	timeout time.Duration
	args    func(code string) []string
}

var configs = map[string]execConfig{
	"go": {
		timeout: 10 * time.Second,
		args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`cd /app/temp && echo '%s' > main.go && go build -o prog main.go && ./prog`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
	"js": {
		timeout: 5 * time.Second,
		args: func(code string) []string {
			return []string{"node", "-e", code}
		},
	},
	"python": {
		timeout: 5 * time.Second,
		args: func(code string) []string {
			return []string{"python3", "-c", code}
		},
	},
	"cpp": {
		timeout: 5 * time.Second,
		args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > prog.cpp && g++ -o prog prog.cpp && ./prog`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
}

func Execute(containerName, language, code string) (string, error) {
	config, ok := configs[language]
	if !ok {
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()

	args := append([]string{"exec", containerName}, config.args(code)...)
	cmd := exec.CommandContext(ctx, "docker", args...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("execution timed out after %v", config.timeout)
		}
		return output.String(), fmt.Errorf("execution error: %w", err)
	}

	return output.String(), nil
}
