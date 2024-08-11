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
	host := e.Addr()
	cmdStr := fmt.Sprintf("docker swarm init --advertise-addr %s", host)
	if err := sshexec.Command(e, cmdStr).Run(); err != nil {
		return fmt.Errorf("failed to initialize Docker Swarm: %w", err)
	}

	return nil
}
