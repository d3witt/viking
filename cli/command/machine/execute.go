package machine

import (
	"errors"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/sshexec"
	"github.com/urfave/cli/v2"
)

func NewExecuteCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:        "exec",
		Description: "Execute shell command on machine",
		Args:        true,
		ArgsUsage:   "MACHINE \"COMMAND\"",
		Action: func(ctx *cli.Context) error {
			machine := ctx.Args().First()
			cmd := ctx.Args().Get(1)

			return runExecute(vikingCli, machine, cmd)
		},
	}
}

func runExecute(vikingCli *command.Cli, machine string, cmd string) error {
	if machine == "" {
		return errors.New("Name is required")
	}

	m, err := vikingCli.Config.GetMachine(machine)
	if err != nil {
		return fmt.Errorf("Failed to get machine: %w", err)
	}

	var private, passphrase string
	if m.Key != "" {
		key, err := vikingCli.Config.GetKeyByName(m.Key)
		if err != nil {
			return fmt.Errorf("Failed to retrieve key: %w", err)
		}

		private = key.Private
		passphrase = key.Passphrase
	}

	client, err := sshexec.SshClient(m.Host.String(), m.User, private, passphrase)
	if err != nil {
		return err
	}
	defer client.Close()

	output, err := sshexec.Command(sshexec.NewExecutor(client), cmd).CombinedOutput()
	if err != nil {
		if _, ok := err.(*sshexec.ExitError); !ok {
			return err
		}
	}

	fmt.Fprintln(vikingCli.Out, output)

	return nil
}
