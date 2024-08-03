package cfg

import (
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config"
	"github.com/urfave/cli/v2"
)

func NewConfigCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Get config directory path",
		Action: func(ctx *cli.Context) error {
			path, err := config.ConfigDir()
			if err != nil {
				return err
			}

			fmt.Fprintln(vikingCli.Out, path)
			return nil
		},
	}
}
