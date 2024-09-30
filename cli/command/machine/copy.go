package machine

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/d3witt/viking/archive"
	"github.com/d3witt/viking/cli/command"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewCopyCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "copy",
		Aliases:   []string{"cp"},
		Usage:     "Copy files/folders between local and remote machine",
		ArgsUsage: "SRC_PATH DEST_PATH",
		Description: `
Copy files/folders between a machine and the local filesystem.
Use ':' to specify a path on the remote machine.

Examples:
		viking machine cp /path/to/file.txt :remote/path
		viking machine cp :remote/path/file.txt /local/path`,
		Action: func(ctx *cli.Context) error {
			args := ctx.Args()
			if args.Len() != 2 {
				return cli.ShowCommandHelp(ctx, "copy")
			}
			return runCopy(vikingCli, args.Get(0), args.Get(1))
		},
	}
}

func runCopy(vikingCli *command.Cli, from, to string) error {
	fromMachine, fromPath := parseMachinePath(from)
	toMachine, toPath := parseMachinePath(to)

	if fromMachine == "" && toMachine == "" {
		return fmt.Errorf("at least one path must be on the remote machine (prefixed with ':')")
	}

	if fromMachine != "" && toMachine != "" {
		return fmt.Errorf("cannot copy between two remote machines")
	}

	client, err := vikingCli.DialMachine()
	if err != nil {
		return err
	}
	defer client.Close()

	if fromMachine != "" {
		return copyFromRemote(vikingCli, client, fromPath, toPath)
	}
	return copyToRemote(vikingCli, client, fromPath, toPath)
}

func copyToRemote(vikingCli *command.Cli, client *ssh.Client, from, to string) error {
	data, err := archive.Tar(from)
	if err != nil {
		return err
	}

	bar := copyProgressBar(
		vikingCli.Out,
		-1,
		fmt.Sprintf("Sending to %s", client.RemoteAddr()),
	)

	reader := io.TeeReader(data, bar)

	err = archive.UntarRemote(client, to, reader)
	bar.Finish()

	return err
}

func copyFromRemote(vikingCli *command.Cli, client *ssh.Client, from, to string) error {
	data, err := archive.TarRemote(client, from)
	if err != nil {
		return err
	}

	bar := copyProgressBar(
		vikingCli.Out,
		-1,
		fmt.Sprintf("Receiving from %s", client.RemoteAddr()),
	)

	reader := io.TeeReader(data, bar)

	err = archive.Untar(reader, to)
	bar.Finish()

	return err
}

func parseMachinePath(fullPath string) (machine, path string) {
	if strings.HasPrefix(fullPath, ":") {
		return "remote", strings.TrimPrefix(fullPath, ":")
	}
	return "", fullPath
}

func copyProgressBar(out io.Writer, maxBytes int64, message string) *progressbar.ProgressBar {
	return progressbar.NewOptions64(
		maxBytes,
		progressbar.OptionSetDescription(message),
		progressbar.OptionSetWriter(out),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(10),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)
}
