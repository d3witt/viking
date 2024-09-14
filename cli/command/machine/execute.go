package machine

import (
	"fmt"
	"strings"
	"sync"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/sshexec"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewExecuteCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "Execute shell command on machine(s)",
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
	if machine == "" {
		clients, err := vikingCli.DialMachines()
		defer func() {
			for _, client := range clients {
				client.Close()
			}
		}()

		if err != nil {
			return err
		}

		return executeCommand(vikingCli, cmd, tty, clients...)
	}

	client, err := vikingCli.DialMachine(machine)
	if err != nil {
		return err
	}
	defer client.Close()

	return executeCommand(vikingCli, cmd, tty, client)
}

func executeCommand(vikingCli *command.Cli, cmd string, tty bool, clients ...*ssh.Client) error {
	if tty {
		if len(clients) != 1 {
			return fmt.Errorf("cannot allocate a pseudo-TTY to multiple hosts")
		}

		return executeTTY(vikingCli, clients[0], cmd)
	}

	var wg sync.WaitGroup
	wg.Add(len(clients))

	for _, client := range clients {
		go func(client *ssh.Client) {
			defer wg.Done()

			out := vikingCli.Out
			errOut := vikingCli.Err
			if len(clients) > 1 {
				prefix := fmt.Sprintf("%s: ", client.RemoteAddr().String())
				out = out.WithPrefix(prefix)
				errOut = errOut.WithPrefix(prefix + "error: ")
			}

			sshCmd := sshexec.Command(client, cmd)

			output, err := sshCmd.CombinedOutput()
			if handleSSHError(err) != nil {
				fmt.Fprint(errOut, string(output))
			}

			fmt.Fprint(out, string(output))
		}(client)
	}

	wg.Wait()
	return nil
}

func executeTTY(vikingCli *command.Cli, client *ssh.Client, cmd string) error {
	sshCmd := sshexec.Command(client, cmd)

	w, h, err := vikingCli.In.Size()
	if err != nil {
		return err
	}

	sshCmd.Stdin = vikingCli.In
	sshCmd.Stdout = vikingCli.Out
	sshCmd.Stderr = vikingCli.Err
	sshCmd.SetPty(h, w)

	if err := vikingCli.Out.MakeRaw(); err != nil {
		return err
	}
	defer vikingCli.Out.Restore()

	return handleSSHError(sshCmd.Run())
}

func handleSSHError(err error) error {
	if _, ok := err.(*sshexec.ExitError); ok {
		return nil
	}
	return err
}
