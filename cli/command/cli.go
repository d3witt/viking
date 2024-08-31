package command

import (
	"log/slog"

	"github.com/d3witt/viking/config"
	"github.com/d3witt/viking/sshexec"
	"github.com/d3witt/viking/streams"
	"golang.org/x/crypto/ssh"
)

type Cli struct {
	Config    *config.Config
	Out, Err  *streams.Out
	In        *streams.In
	CmdLogger *slog.Logger
}

func (c *Cli) DialMachine(machine string) ([]*ssh.Client, error) {
	m, err := c.Config.GetMachineByName(machine)
	if err != nil {
		return nil, err
	}

	execs := make([]*ssh.Client, len(m.Hosts))
	for i, host := range m.Hosts {
		exec, err := c.DialHost(host)
		if err != nil {
			return nil, err
		}

		execs[i] = exec
	}

	return execs, nil
}

func (c *Cli) DialHost(host config.Host) (*ssh.Client, error) {
	var private, passphrase string
	if host.Key != "" {
		key, err := c.Config.GetKeyByName(host.Key)
		if err != nil {
			return nil, err
		}

		private = key.Private
		passphrase = key.Passphrase
	}

	return sshexec.SSHClient(host.IP.String(), host.Port, host.User, private, passphrase)
}
