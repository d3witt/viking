package machine

import (
	"context"
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
		Usage:     "Execute command on machine(s)",
		ArgsUsage: "CMD ARGS...",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "machine",
				Aliases: []string{"m"},
				Usage:   "Machine to execute command on",
			},
			&cli.BoolFlag{
				Name:    "tty",
				Aliases: []string{"t"},
				Usage:   "Allocate a pseudo-TTY",
			},
		},
		Action: func(ctx *cli.Context) error {
			machine := ctx.String("machine")
			cmd := strings.Join(ctx.Args().Slice(), " ")
			tty := ctx.Bool("tty")

			return runExecute(ctx.Context, vikingCli, machine, cmd, tty)
		},
	}
}

func runExecute(ctx context.Context, vikingCli *command.Cli, machine, cmd string, tty bool) error {
	clients, err := getExecClients(ctx, vikingCli, machine)
	if err != nil {
		return err
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	if tty {
		return executeTTY(vikingCli, clients[0], cmd)
	}

	return executeSequential(vikingCli, clients, cmd)
}

func getExecClients(ctx context.Context, vikingCli *command.Cli, machine string) ([]*ssh.Client, error) {
	if machine == "" {
		return vikingCli.DialMachines(ctx)
	}
	client, err := vikingCli.DialMachine(machine)
	if err != nil {
		return nil, err
	}
	return []*ssh.Client{client}, nil
}

func executeSequential(vikingCli *command.Cli, clients []*ssh.Client, cmd string) error {
	multi := len(clients) > 1

	for i, client := range clients {
		addr := client.RemoteAddr().String()
		prefix := ""
		if multi {
			prefix = fmt.Sprintf("%s: ", addr)
		}

		fmt.Fprintf(vikingCli.Out, "%sExecuting command: %s\n", prefix, cmd)

		if err := executeCmd(vikingCli, client, cmd); err != nil {
			fmt.Fprintf(vikingCli.Err, "%sError: %v\n", prefix, err)
		}

		if multi && i < len(clients)-1 {
			fmt.Fprintln(vikingCli.Out)
		}
	}

	return nil
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
