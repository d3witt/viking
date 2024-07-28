package key

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"github.com/workdate-dev/viking/cli/command"
	"golang.design/x/clipboard"
)

func NewCopyCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "copy",
		Usage:     "Copy private|public key to clipboard",
		Args:      true,
		ArgsUsage: "NAME",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "private",
				Usage: "Copy private key",
			},
			&cli.BoolFlag{
				Name:  "public",
				Usage: "Copy public key",
			},
		},
		Action: func(ctx *cli.Context) error {
			name := ctx.Args().First()
			private := ctx.Bool("private")
			public := ctx.Bool("public")

			if private == public {
				return errors.New("Please specify either --private or --public")
			}

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
