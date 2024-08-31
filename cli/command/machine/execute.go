package machine

import (
	"fmt"
	"io"
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
		Usage:     "Execute shell command on machine",
		ArgsUsage: "NAME \"COMMAND\"",
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
	clients, err := vikingCli.DialMachine(machine)
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	if err != nil {
		return err
	}

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

			if err := execute(out, client, cmd); err != nil {
				fmt.Fprintln(errOut, err.Error())
			}
		}(client)
	}

	wg.Wait()
	return nil
}

func execute(out io.Writer, client *ssh.Client, cmd string) error {
	sshCmd := sshexec.Command(client, cmd)

	output, err := sshCmd.CombinedOutput()
	if handleSSHError(err) != nil {
		return err
	}

	fmt.Fprint(out, string(output))
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
