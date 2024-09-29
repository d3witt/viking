package command

import (
	"context"
	"errors"
	"log/slog"

	"github.com/d3witt/viking/dockerhelper"
	"golang.org/x/crypto/ssh"
)

func (c *Cli) Swarm(ctx context.Context, clients []*ssh.Client) (sw *dockerhelper.Swarm, err error) {
	if len(clients) == 0 {
		return dockerhelper.NewSwarm([]*dockerhelper.Client{}), nil
	}

	dockerClients := make([]*dockerhelper.Client, len(clients))
	for i, sshClient := range clients {
		dockerClients[i], err = dockerhelper.DialSSH(sshClient)
		if err != nil {
			CloseDockerClients(dockerClients)
			return nil, err
		}
	}

	return dockerhelper.NewSwarm(dockerClients), nil
}

func (c *Cli) SwarmAvailable(ctx context.Context, clients []*ssh.Client) (*dockerhelper.Swarm, error) {
	var avail []*dockerhelper.Client
	for _, cl := range clients {
		dc, err := dockerhelper.DialSSH(cl)
		if err != nil {
			slog.ErrorContext(ctx, "error dialing docker client", "host", cl.RemoteAddr().String(), "error", err)
			continue
		}
		avail = append(avail, dc)
	}

	if len(avail) == 0 {
		return nil, errors.New("no clients available")
	}

	return dockerhelper.NewSwarm(avail), nil
}

func (c *Cli) DialNode(ctx context.Context, machine string) (cl *dockerhelper.Client, err error) {
	clients, err := c.DialMachine(machine)
	if err != nil {
		return nil, err
	}

	return dockerhelper.DialSSH(clients)
}

func CloseDockerClients(clients []*dockerhelper.Client) {
	for _, client := range clients {
		if client == nil {
			continue
		}

		client.Close()
	}
}
