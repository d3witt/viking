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

	var private, passphrase string
	if m.Key != "" {
		key, err := c.Config.GetKeyByName(m.Key)
		if err != nil {
			return nil, err
		}

		private = key.Private
		passphrase = key.Passphrase
	}

	execs := make([]sshexec.Executor, len(m.Host))
	for i, host := range m.Host {
		execs[i] = sshexec.NewExecutor(host.String(), m.User, private, passphrase)
	}

	return execs, nil
}
