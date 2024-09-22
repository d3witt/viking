package command

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/d3witt/viking/dockerhelper"
)

func (c *Cli) DialSwarm(ctx context.Context) (sw *dockerhelper.Swarm, err error) {
	clients, err := c.DialMachines(ctx)
	if err != nil {
		return nil, err
	}

	if len(clients) == 0 {
		return dockerhelper.NewSwarm([]*dockerhelper.Client{}), nil
	}

	dockerClients := make([]*dockerhelper.Client, 0, len(clients))
	for _, sshClient := range clients {
		dockerClient, err := dockerhelper.DialSSH(sshClient)
		if err != nil {
			slog.WarnContext(ctx, "Error dialing docker client", "err", err)
		}

		dockerClients = append(dockerClients, dockerClient)
	}

	if len(dockerClients) == 0 {
		return nil, fmt.Errorf("no docker clients available")
	}

	return dockerhelper.NewSwarm(dockerClients), nil
}

func (c *Cli) DialNode(ctx context.Context, machine string) (cl *dockerhelper.Client, err error) {
	clients, err := c.DialMachine(machine)
	if err != nil {
		return nil, err
	}

	return dockerhelper.DialSSH(clients)
}
