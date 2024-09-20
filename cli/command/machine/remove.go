package machine

import (
	"fmt"
	"strings"

	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
)

func NewRmCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "Remove a machine(s)",
		Args:      true,
		ArgsUsage: "[IP]",
		Action: func(ctx *cli.Context) error {
			machine := ctx.Args().First()
			return runRemove(vikingCli, machine)
		},
	}
}

func runRemove(vikingCli *command.Cli, machine string) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	if machine == "" {
		machiens := conf.ListMachines()
		hosts := []string{}
		for _, m := range machiens {
			hosts = append(hosts, m.IP.String())
		}

		if len(hosts) == 0 {
			return nil
		}

		if err := conf.ClearMachines(); err != nil {
			return err
		}

		fmt.Fprintln(vikingCli.Out, strings.Join(hosts, ", "))
		return nil
	}

	if err := conf.RemoveMachine(machine); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, machine)

	return nil
}
