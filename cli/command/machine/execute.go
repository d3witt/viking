package machine

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/sshexec"
	"github.com/urfave/cli/v2"
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
	m, err := vikingCli.Config.GetMachineByName(machine)
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

	if tty {
		if len(m.Host) != 1 {
			return fmt.Errorf("cannot allocate a pseudo-TTY to multiple hosts")
		}

		return executeTTY(vikingCli, m.Host[0].String(), m.User, private, passphrase, cmd)
	}

	var wg sync.WaitGroup
	wg.Add(len(m.Host))

	for _, host := range m.Host {
		go func(ip string) {
			defer wg.Done()

			out := vikingCli.Out
			errOut := vikingCli.Err
			if len(m.Host) > 1 {
				prefix := fmt.Sprintf("%s: ", ip)
				out = out.WithPrefix(prefix)
				errOut = errOut.WithPrefix(prefix + "error: ")
			}

			if err := execute(out, ip, m.User, private, passphrase, cmd); err != nil {
				fmt.Fprintln(errOut, err.Error())
			}
		}(host.String())
	}

	wg.Wait()
	return nil
}

func execute(out io.Writer, ip, user, private, passphrase, cmd string) error {
	client, err := sshexec.SshClient(ip, user, private, passphrase)
	if err != nil {
		return err
	}
	defer client.Close()

	sshCmd := sshexec.Command(sshexec.NewExecutor(client), cmd)
	sshCmd.NoLogs = true

	output, err := sshCmd.CombinedOutput()
	if handleSSHError(err) != nil {
		return err
	}

	fmt.Fprint(out, string(output))
	return nil
}

func executeTTY(vikingCli *command.Cli, ip, user, private, passphrase, cmd string) error {
	client, err := sshexec.SshClient(ip, user, private, passphrase)
	if err != nil {
		return err
	}
	defer client.Close()

	sshCmd := sshexec.Command(sshexec.NewExecutor(client), cmd)
	sshCmd.NoLogs = true

	w, h, err := vikingCli.In.Size()
	if err != nil {
		return err
	}

	if err := vikingCli.Out.MakeRaw(); err != nil {
		return err
	}
	defer vikingCli.Out.Restore()

	err = sshCmd.StartInteractive(cmd, vikingCli.In, vikingCli.Out, vikingCli.Err, w, h)
	if handleSSHError(err) != nil {
		return err
	}

	return nil
}

func handleSSHError(err error) error {
	if _, ok := err.(*sshexec.ExitError); ok {
		return nil
	}
	return err
}
