package app

import (
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config/appconf"
	"github.com/urfave/cli/v2"
)

func NewInitCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize a new viking config",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Name of the app",
			},
		},
		Action: func(ctx *cli.Context) error {
			name := ctx.String("name")

			_, err := appconf.NewDefaultConfig(name)
			if err != nil {
				return err
			}

			fmt.Println("viking.toml")

			return nil
		},
	}
}
