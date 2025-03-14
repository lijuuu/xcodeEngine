package main

import (
	"fmt"
	"os"
	"os/exec"
)

func runCommand(command string, args ...string) {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: dockerkill <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "everything":
		runCommand("docker", "system", "prune", "-a", "--volumes", "-f")
	case "images":
		runCommand("docker", "rmi", "$(docker images -q)", "--force")
	case "containers":
		runCommand("docker", "kill", "$(docker ps -q)")
	case "networks":
		runCommand("docker", "network", "prune", "-f")
	case "volumes":
		runCommand("docker", "volume", "prune", "-f")
	case "list":
		if len(os.Args) < 3 {
			fmt.Println("Usage: dockerkill list <images|containers|networks|volumes>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "images":
			runCommand("docker", "images")
		case "containers":
			runCommand("docker", "ps", "-a")
		case "networks":
			runCommand("docker", "network", "ls")
		case "volumes":
			runCommand("docker", "volume", "ls")
		default:
			fmt.Println("Unknown list command.")
			os.Exit(1)
		}
	case "prune":
		runCommand("docker", "system", "prune", "-a")
	default:
		fmt.Println("Unknown command.")
		os.Exit(1)
	}
}