package key

import (
	"errors"
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/workdate-dev/viking/cli/command"
)

func NewRmCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "Remove a key",
		Args:      true,
		ArgsUsage: "NAME",
		Action: func(ctx *cli.Context) error {
			name := ctx.Args().First()
			return runRemove(vikingCli, name)
		},
	}
}

func runRemove(vikingCli *command.Cli, name string) error {
	if name == "" {
		return errors.New("Name cannot be empty")
	}

	if err := vikingCli.Config.RemoveKey(name); err != nil {
		return fmt.Errorf("Failed to remove key: %w", err)
	}

	fmt.Fprintln(vikingCli.Out, "Key removed from this computer.")

	return nil
}
