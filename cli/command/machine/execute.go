package machine

import (
	"fmt"
	"strings"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/sshexec"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewExecuteCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "Execute command on machine",
		ArgsUsage: "CMD ARGS...",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "tty",
				Aliases: []string{"t"},
				Usage:   "Allocate a pseudo-TTY",
			},
		},
		Action: func(ctx *cli.Context) error {
			cmd := strings.Join(ctx.Args().Slice(), " ")
			tty := ctx.Bool("tty")

			return runExecute(vikingCli, cmd, tty)
		},
	}
}

func runExecute(vikingCli *command.Cli, cmd string, tty bool) error {
	sshClient, err := vikingCli.DialMachine()
	if err != nil {
		return err
	}
	defer sshClient.Close()

	if tty {
		return executeTTY(vikingCli, sshClient, cmd)
	}

	return executeCmd(vikingCli, sshClient, cmd)
}

func executeCmd(vikingCli *command.Cli, client *ssh.Client, cmd string) error {
	sshCmd := sshexec.Command(client, cmd)
	output, err := sshCmd.CombinedOutput()

	if err != nil && !isExitError(err) {
		return err
	}

	fmt.Fprint(vikingCli.Out, string(output))
	return nil
}

func executeTTY(vikingCli *command.Cli, client *ssh.Client, cmd string) error {
	sshCmd := sshexec.Command(client, cmd)

	w, h, err := vikingCli.In.Size()
	if err != nil {
		return fmt.Errorf("get terminal size: %w", err)
	}

	sshCmd.Stdin = vikingCli.In
	sshCmd.Stdout = vikingCli.Out
	sshCmd.Stderr = vikingCli.Err
	sshCmd.SetPty(h, w)

	if err := vikingCli.Out.MakeRaw(); err != nil {
		return err
	}
	defer vikingCli.Out.Restore()

	if err := sshCmd.Run(); err != nil && !isExitError(err) {
		return err
	}

	return nil
}

func isExitError(err error) bool {
	_, ok := err.(*sshexec.ExitError)
	return ok
}
