package machine

import (
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
)

func NewRmCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "Remove a machine",
		Args:      true,
		ArgsUsage: "NAME",
		Action: func(ctx *cli.Context) error {
			machine := ctx.Args().First()
			return runRemove(vikingCli, machine)
		},
	}
}

func runRemove(vikingCli *command.Cli, machine string) error {
	if err := vikingCli.Config.RemoveMachine(machine); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Machine removed from this computer.")

	return nil
}
