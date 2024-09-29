package command

import (
	"context"
	"log/slog"
	"sync"

	"github.com/d3witt/viking/parallel"
	"github.com/d3witt/viking/sshexec"
	"golang.org/x/crypto/ssh"
)

func (c *Cli) DialMachines(ctx context.Context) ([]*ssh.Client, error) {
	conf, err := c.AppConfig()
	if err != nil {
		return nil, err
	}

	machines := conf.ListMachines()

	if len(machines) == 0 {
		return nil, nil
	}

	clients := make([]*ssh.Client, len(machines))
	if err := parallel.RunFirstErr(ctx, len(machines), func(i int) error {
		m := machines[i]
		private, passphrase, err := c.GetSSHKeyDetails(m.Key)
		if err != nil {
			return err
		}

		client, dialErr := sshexec.SSHClient(m.IP.String(), m.Port, m.User, private, passphrase)
		clients[i] = client

		return dialErr
	}); err != nil {
		CloseSSHClients(clients)
		return nil, err
	}

	return clients, nil
}

func (c *Cli) DialAvailableMachines(ctx context.Context) []*ssh.Client {
	conf, err := c.AppConfig()
	if err != nil {
		return nil
	}

	machines := conf.ListMachines()

	if len(machines) == 0 {
		return nil
	}

	var mu sync.Mutex
	clients := make([]*ssh.Client, 0, len(machines))
	parallel.Run(ctx, len(machines), func(i int) {
		m := machines[i]
		private, passphrase, err := c.GetSSHKeyDetails(m.Key)
		if err != nil {
			slog.ErrorContext(ctx, "Error getting SSH key details", "machine", m.IP.String(), "error", err)
			return
		}

		client, dialErr := sshexec.SSHClient(m.IP.String(), m.Port, m.User, private, passphrase)
		if dialErr != nil {
			slog.ErrorContext(ctx, "Error dialing machine", "machine", m.IP.String(), "error", dialErr)
			return
		}

		mu.Lock()
		clients = append(clients, client)
		mu.Unlock()
	})

	return clients
}

func (c *Cli) DialMachine(machine string) (*ssh.Client, error) {
	conf, err := c.AppConfig()
	if err != nil {
		return nil, err
	}

	m, err := conf.GetMachine(machine)
	if err != nil {
		return nil, err
	}

	private, passphrase, err := c.GetSSHKeyDetails(m.Key)
	if err != nil {
		return nil, err
	}

	return sshexec.SSHClient(m.IP.String(), m.Port, m.User, private, passphrase)
}

func (c *Cli) GetSSHKeyDetails(key string) (private, passphrase string, err error) {
	if key == "" {
		return "", "", nil
	}

	k, err := c.Config.GetKeyByName(key)
	if err != nil {
		return "", "", err
	}

	return k.Private, k.Passphrase, nil
}

func CloseSSHClients(clients []*ssh.Client) {
	for _, client := range clients {
		if client == nil {
			continue
		}

		client.Close()
	}
}
