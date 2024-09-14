package machine

import (
	"strings"

	"github.com/d3witt/viking/cli/command"
	"golang.org/x/crypto/ssh"
)

func getRemoteClients(vikingCli *command.Cli, machine string) ([]*ssh.Client, error) {
	// app needed for copy command
	if strings.EqualFold(machine, "app") || machine == "" {
		return vikingCli.DialMachines()
	}

	client, err := vikingCli.DialMachine(machine)
	if err != nil {
		return nil, err
	}

	return []*ssh.Client{client}, nil
}
