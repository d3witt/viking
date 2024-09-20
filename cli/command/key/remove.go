package key

import (
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
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
	if err := vikingCli.Config.RemoveKey(name); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, name)

	return nil
}
