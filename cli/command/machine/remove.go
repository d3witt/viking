package machine

import (
	"errors"
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/workdate-dev/viking/cli/command"
)

func NewRmCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "Remove a machine",
		Args:      true,
		ArgsUsage: "MACHINE",
		Action: func(ctx *cli.Context) error {
			machine := ctx.Args().First()
			return runRemove(vikingCli, machine)
		},
	}
}

func runRemove(vikingCli *command.Cli, machine string) error {
	if machine == "" {
		return errors.New("Name cannot be empty")
	}

	if err := vikingCli.Config.RemoveMachine(machine); err != nil {
		return fmt.Errorf("Failed to remove machine: %w", err)
	}

	fmt.Fprintln(vikingCli.Out, "Machine removed from this computer.")

	return nil
}
