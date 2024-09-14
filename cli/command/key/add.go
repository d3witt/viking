package key

import (
	"fmt"
	"os"
	"time"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config/userconf"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewAddCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add a new ssh key from file",
		Args:      true,
		ArgsUsage: "FILE_PATH",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Usage:   "Key name",
				Aliases: []string{"n"},
			},
			&cli.StringFlag{
				Name:    "passphrase",
				Usage:   "Key passphrase",
				Aliases: []string{"p"},
			},
		},
		Action: func(ctx *cli.Context) error {
			path := ctx.Args().First()
			name := ctx.String("name")
			passphrase := ctx.String("passphrase")

			return runAdd(vikingCli, path, name, passphrase)
		},
	}
}

func runAdd(vikingCli *command.Cli, path, name, passphrase string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var privateKey ssh.Signer

	if passphrase == "" {
		privateKey, err = ssh.ParsePrivateKey(data)
	} else {
		privateKey, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
	}

	if err != nil {
		return err
	}

	publicKey := ssh.MarshalAuthorizedKey(privateKey.PublicKey())

	if name == "" {
		name = command.GenerateRandomName()
	}

	if err := vikingCli.Config.AddKey(
		userconf.Key{
			Name:       name,
			Private:    string(data),
			Public:     string(publicKey),
			Passphrase: passphrase,
			CreatedAt:  time.Now(),
		},
	); err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "Key %s added.\n", name)

	return nil
}
