package command

import (
	"log/slog"

	"github.com/d3witt/viking/config"
	"github.com/d3witt/viking/sshexec"
	"github.com/d3witt/viking/streams"
)

type Cli struct {
	Config    *config.Config
	Out, Err  *streams.Out
	In        *streams.In
	CmdLogger *slog.Logger
}

func (c *Cli) MachineExecuters(machine string) ([]sshexec.Executor, error) {
	m, err := c.Config.GetMachineByName(machine)
	if err != nil {
		return nil, err
	}

	execs := make([]sshexec.Executor, len(m.Hosts))
	for i, host := range m.Hosts {
		exec, err := c.HostExecutor(host)
		if err != nil {
			return nil, err
		}

		execs[i] = exec
	}

	return execs, nil
}

func (c *Cli) HostExecutor(host config.Host) (sshexec.Executor, error) {
	var private, passphrase string
	if host.Key != "" {
		key, err := c.Config.GetKeyByName(host.Key)
		if err != nil {
			return nil, err
		}

		private = key.Private
		passphrase = key.Passphrase
	}

	return sshexec.NewExecutor(host.IP.String(), host.Port, host.User, private, passphrase), nil
}
