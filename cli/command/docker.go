package command

import (
	"context"

	"github.com/d3witt/viking/dockerhelper"
)

func (c *Cli) DialManagerNode(ctx context.Context, machine string) (cl *dockerhelper.Client, err error) {
	clients, err := c.DialMachine(machine)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, client := range clients {
			if cl == nil || cl.SSH != client {
				client.Close()
			}
		}
	}()

	dockerClients := make([]*dockerhelper.Client, 0, len(clients))
	for _, sshClient := range clients {
		dockerClient, _ := dockerhelper.DialSSH(sshClient)
		// TODO: log error if verbose mode

		dockerClients = append(dockerClients, dockerClient)
	}

	return dockerhelper.ManagerNode(ctx, dockerClients)
}
