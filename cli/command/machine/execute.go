package machine

import (
	"fmt"
	"strings"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/sshexec"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func NewExecuteCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "Execute shell command on machine",
		Args:      true,
		ArgsUsage: "MACHINE \"COMMAND\"",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "tty",
				Aliases: []string{"t"},
				Usage:   "Allocate a pseudo-TTY",
			},
		},
		Action: func(ctx *cli.Context) error {
			machine := ctx.Args().First()
			cmd := strings.Join(ctx.Args().Tail(), " ")
			tty := ctx.Bool("tty")

			return runExecute(vikingCli, machine, cmd, tty)
		},
	}
}

func runExecute(vikingCli *command.Cli, machine string, cmd string, tty bool) error {
	m, err := vikingCli.Config.GetMachine(machine)
	if err != nil {
		return err
	}

	var private, passphrase string
	if m.Key != "" {
		key, err := vikingCli.Config.GetKeyByName(m.Key)
		if err != nil {
			return err
		}

		private = key.Private
		passphrase = key.Passphrase
	}

	client, err := sshexec.SshClient(m.Host.String(), m.User, private, passphrase)
	if err != nil {
		return err
	}
	defer client.Close()

	sshCmd := sshexec.Command(sshexec.NewExecutor(client), cmd)
	sshCmd.NoLogs = true

	if tty {
		w, h, err := vikingCli.TerminalSize()
		if err != nil {
			return err
		}

		termState, err := term.GetState(vikingCli.OutFd)
		if err != nil {
			return fmt.Errorf("failed to get terminal state: %w", err)
		}
		defer term.Restore(vikingCli.OutFd, termState)

		term.MakeRaw(vikingCli.OutFd)
		if err := sshCmd.StartInteractive(cmd, vikingCli.In, vikingCli.Out, vikingCli.Err, w, h); handleSSHError(err) != nil {
			return err
		}

		return nil
	}

	output, err := sshCmd.CombinedOutput()
	if handleSSHError(err) != nil {
		return err
	}

	fmt.Fprint(vikingCli.Out, string(output))

	return nil
}

func handleSSHError(err error) error {
	if _, ok := err.(*sshexec.ExitError); ok {
		return nil
	}

	return err
}
