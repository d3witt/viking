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
		Action: func(ctx *cli.Context) error {
			_, err := appconf.NewDefaultConfig()
			if err != nil {
				return err
			}

			fmt.Println("viking.toml created")

			return nil
		},
	}
}
