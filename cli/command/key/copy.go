package key

import (
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.design/x/clipboard"
)

func NewCopyCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "copy",
		Usage:     "Copy public key (or private with --private) to clipboard.",
		Args:      true,
		ArgsUsage: "NAME",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "private",
				Usage: "Copy private key",
			},
		},
		Action: func(ctx *cli.Context) error {
			name := ctx.Args().First()
			private := ctx.Bool("private")

			return runCopy(vikingCli, name, private)
		},
	}
}

func runCopy(vikingCli *command.Cli, name string, private bool) error {
	if name == "" {
		return errors.New("Name cannot be empty")
	}

	key, err := vikingCli.Config.GetKeyByName(name)
	if err != nil {
		return fmt.Errorf("Failed to retrieve key: %w", err)
	}

	err = clipboard.Init()
	if err != nil {
		return fmt.Errorf("Failed to copy key to clipboard: %w", err)
	}

	if private {
		clipboard.Write(clipboard.FmtText, []byte(key.Private))
		fmt.Fprintln(vikingCli.Out, "Private key copied to your clipboard.")
		return nil
	}

	clipboard.Write(clipboard.FmtText, []byte(key.Public))
	fmt.Fprintln(vikingCli.Out, "Public key copied to your clipboard.")

	return nil
}
