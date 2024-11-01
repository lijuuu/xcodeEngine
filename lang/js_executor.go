package lang

import (
	"os/exec"
)

func ExecuteJsCode(containerName, code string) (string, error) {
	cmd := exec.Command("docker", "exec", containerName, "node", "-e", code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}
