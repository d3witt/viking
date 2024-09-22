package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/d3witt/viking/parallel"
	"github.com/d3witt/viking/sshexec"
	"golang.org/x/crypto/ssh"
)

// DialMachines dials all hosts of a machine and returns a slice of SSH clients
// for available hosts. If there are no available hosts, DialMachine returns
// an error.
func (c *Cli) DialMachines(ctx context.Context) ([]*ssh.Client, error) {
	conf, err := c.AppConfig()
	if err != nil {
		return nil, err
	}

	machines := conf.ListMachines()

	if len(machines) == 0 {
		return []*ssh.Client{}, nil
	}

	var clients []*ssh.Client
	var mu sync.Mutex

	parallel.ForEach(ctx, len(machines), func(i int) {
		m := machines[i]
		private, passphrase, err := c.GetSSHKeyDetails(m.Key)
		if err != nil {
			fmt.Fprint(c.Out, err.Error())
			return
		}

		client, dialErr := sshexec.SSHClient(m.IP.String(), m.Port, m.User, private, passphrase)
		if dialErr != nil {
			slog.WarnContext(ctx, "Error dialing SSH client", "err", dialErr)
			return
		}

		mu.Lock()
		clients = append(clients, client)
		mu.Unlock()
	})

	if len(clients) == 0 {
		return nil, errors.New("no available hosts")
	}

	return clients, nil
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
