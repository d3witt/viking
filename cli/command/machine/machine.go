package machine

import (
	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
)

func NewCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "machine",
		Usage: "Manage your machines",
		Subcommands: []*cli.Command{
			NewAddCmd(vikingCli),
			NewListCmd(vikingCli),
			NewRmCmd(vikingCli),
			NewExecuteCmd(vikingCli),
			NewCopyCmd(vikingCli),
			NewPrepareCmd(vikingCli),
			NewStatusCmd(vikingCli),
		},
	}
}
