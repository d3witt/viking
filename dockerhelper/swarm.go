package dockerhelper

import (
	"fmt"

	"github.com/d3witt/viking/sshexec"
)

func IsSwarmInitialized(e sshexec.Executor) bool {
	cmdStr := "docker info --format '{{.Swarm.LocalNodeState}}' | grep -qx 'active'"
	return sshexec.Command(e, cmdStr).Run() == nil
}

func InitDockerSwarm(e sshexec.Executor) error {
	cmdStr := "docker swarm init"
	if err := sshexec.Command(e, cmdStr).Run(); err != nil {
		return fmt.Errorf("failed to initialize Docker Swarm: %w", err)
	}

	return nil
}

func RunService(e sshexec.Executor, publish, name, image, cmd string) error {
	cmdStr := fmt.Sprintf("docker service create --with-registry-auth --name %s %s", name, image)
	if err := sshexec.Command(e, cmdStr).Run(); err != nil {
		return fmt.Errorf("failed to run service: %w", err)
	}

	return nil
}
