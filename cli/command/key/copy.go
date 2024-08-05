package key

import (
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
	"golang.design/x/clipboard"
)

func NewCopyCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "copy",
		Usage:     "Copy public key (or private with --private) to clipboard.",
		Args:      true,
		ArgsUsage: "NAME",
		Action: func(ctx *cli.Context) error {
			name := ctx.Args().First()

			return runCopy(vikingCli, name)
		},
	}
}

func runCopy(vikingCli *command.Cli, name string) error {
	key, err := vikingCli.Config.GetKeyByName(name)
	if err != nil {
		return err
	}

	err = clipboard.Init()
	if err != nil {
		return err
	}

	clipboard.Write(clipboard.FmtText, []byte(key.Public))
	fmt.Fprintln(vikingCli.Out, "Public key copied to your clipboard.")

	return nil
}
