package key

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"github.com/workdate-dev/viking/cli/command"
	"github.com/workdate-dev/viking/config"
	"golang.org/x/crypto/ssh"
)

func NewGenerateCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Generate a new SSH key",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "name",
				Usage: "Key name",
			},
		},
		Action: func(ctx *cli.Context) error {
			name := ctx.String("name")

			return runGenerate(vikingCli, name)
		},
	}
}

func runGenerate(vikingCli *command.Cli, name string) error {
	if name == "" {
		name = command.GenerateRandomName()
	}

	private, public, err := generateSSHKeyPair()
	if err != nil {
		return fmt.Errorf("Failed to generate ssh key: %w", err)
	}

	_, err = vikingCli.Config.GetKeyByName(name)
	if err == nil {
		return errors.New("Key with this name arleady exist")
	}

	if err = vikingCli.Config.AddKey(
		config.Key{
			Name:      name,
			Private:   string(private),
			Public:    string(public),
			CreatedAt: time.Now(),
		},
	); err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "Key %s added.\n", name)

	return nil
}

func generateSSHKeyPair() (privateKey, publicKey string, err error) {
	privateRSAKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Generate the private key PEM block.
	privatePEMBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateRSAKey)}

	// Encode the private key to PEM format.
	privateKeyBytes := pem.EncodeToMemory(privatePEMBlock)
	privateKey = string(privateKeyBytes)

	// Generate the public key for the private key.
	publicRSAKey, err := ssh.NewPublicKey(&privateRSAKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	// Encode the public key to the authorized_keys format.
	publicKeyBytes := ssh.MarshalAuthorizedKey(publicRSAKey)
	publicKey = string(publicKeyBytes)

	return privateKey, publicKey, nil
}
