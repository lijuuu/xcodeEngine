package lang

import (
	"os/exec"
)

func ExecutePythonCode(containerName, code string) (string, error) {
	cmd := exec.Command("docker", "exec", containerName, "python3", "-c", code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}
