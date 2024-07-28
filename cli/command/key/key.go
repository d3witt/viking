package key

import (
	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
)

func NewCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "key",
		Usage: "Manage SSH keys",
		Subcommands: []*cli.Command{
			NewAddCmd(vikingCli),
			NewListCmd(vikingCli),
			NewRmCmd(vikingCli),
			NewGenerateCmd(vikingCli),
			NewCopyCmd(vikingCli),
		},
	}
}
