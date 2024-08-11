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
	execs, err := vikingCli.MachineExecuters(machine)
	defer func() {
		for _, exec := range execs {
			exec.Close()
		}
	}()

	if err != nil {
		return err
	}

	if tty {
		if len(execs) != 1 {
			return fmt.Errorf("cannot allocate a pseudo-TTY to multiple hosts")
		}

		return executeTTY(vikingCli, execs[0], cmd)
	}

	var wg sync.WaitGroup
	wg.Add(len(execs))

	for _, exec := range execs {
		go func(exec sshexec.Executor) {
			defer wg.Done()

			out := vikingCli.Out
			errOut := vikingCli.Err
			if len(execs) > 1 {
				prefix := fmt.Sprintf("%s: ", exec.Addr())
				out = out.WithPrefix(prefix)
				errOut = errOut.WithPrefix(prefix + "error: ")
			}

			if err := execute(out, exec, cmd); err != nil {
				fmt.Fprintln(errOut, err.Error())
			}
		}(exec)
	}

	wg.Wait()
	return nil
}

func execute(out io.Writer, exec sshexec.Executor, cmd string) error {
	sshCmd := sshexec.Command(exec, cmd)

	output, err := sshCmd.CombinedOutput()
	if handleSSHError(err) != nil {
		return err
	}

	fmt.Fprint(out, string(output))
	return nil
}

func executeTTY(vikingCli *command.Cli, exec sshexec.Executor, cmd string) error {
	sshCmd := sshexec.Command(exec, cmd)

	w, h, err := vikingCli.In.Size()
	if err != nil {
		return err
	}

	if err := vikingCli.Out.MakeRaw(); err != nil {
		return err
	}
	defer vikingCli.Out.Restore()

	err = sshCmd.RunInteractive(vikingCli.In, vikingCli.Out, vikingCli.Err, w, h)
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
