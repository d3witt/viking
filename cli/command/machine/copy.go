package machine

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
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
		ArgsUsage: "[remote|IP]:SRC_PATH DEST_PATH | SRC_PATH [remote|IP]:DEST_PATH",
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
		return fmt.Errorf("at least one path must contain machine name")
	}

	if fromMachine != "" && toMachine != "" {
		return fmt.Errorf("cannot copy between two remote machines")
	}

	clients, err := dialCopyMachines(vikingCli, fromMachine, toMachine)
	if err != nil {
		return err
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	if fromMachine != "" {
		return copyFromRemote(vikingCli, fromPath, toPath, clients...)
	}
	return copyToRemote(vikingCli, fromPath, toPath, clients...)
}

func dialCopyMachines(vikingCli *command.Cli, fromMachine, toMachine string) ([]*ssh.Client, error) {
	machine := fromMachine + toMachine
	if strings.EqualFold(machine, "remote") {
		return vikingCli.DialMachines()
	} else {
		client, err := vikingCli.DialMachine(machine)
		if err != nil {
			return nil, err
		}
		return []*ssh.Client{client}, nil
	}
}

func copyToRemote(vikingCli *command.Cli, from, to string, clients ...*ssh.Client) error {
	data, err := archive.Tar(from)
	if err != nil {
		return err
	}

	// Create a temporary file to store the tar archive
	tmpFile, err := os.CreateTemp("", "archive-*.tar")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	// Write the tar archive to the temporary file
	written, err := io.Copy(tmpFile, data)
	if err != nil {
		return err
	}

	// Close the temporary file to flush the data
	if err := tmpFile.Close(); err != nil {
		return err
	}

	var errorMessages []string

	for _, client := range clients {
		// Open the temporary file for reading
		tmpFile, err := os.Open(tmpFile.Name())
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Error opening temporary file: %v", err))
			continue
		}

		bar := copyProgressBar(
			vikingCli.Out,
			written,
			fmt.Sprintf("Sending to %s", client.RemoteAddr()),
		)

		// Create a multi-reader to read from the file and update the progress bar
		reader := io.TeeReader(tmpFile, bar)

		if err := archive.UntarRemote(client, to, reader); err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", client.RemoteAddr().String(), err))
		}

		tmpFile.Close()
		bar.Finish()
	}

	printCopyStatus(vikingCli.Out, len(clients), errorMessages)

	return nil
}

func copyFromRemote(vikingCli *command.Cli, from, to string, clients ...*ssh.Client) error {
	var errorMessages []string

	for _, client := range clients {
		dest := to
		if len(clients) > 1 {
			dest = path.Join(to, client.RemoteAddr().String())
		}

		data, err := archive.TarRemote(client, from)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", client.RemoteAddr().String(), err))
			continue
		}

		bar := copyProgressBar(
			vikingCli.Out,
			-1,
			fmt.Sprintf("Receiving from %s", client.RemoteAddr()),
		)

		reader := io.TeeReader(data, bar)

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(reader)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", client.RemoteAddr().String(), err))
			continue
		}

		if err := archive.Untar(buf, dest); err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Error untar to %s: %v", dest, err))
		}

		bar.Finish()
	}

	printCopyStatus(vikingCli.Out, len(clients), errorMessages)

	return nil
}

func parseMachinePath(fullPath string) (machine, path string) {
	if strings.Contains(fullPath, ":") {
		parts := strings.SplitN(fullPath, ":", 2)
		return parts[0], parts[1]
	}

	return "", fullPath
}

func printCopyStatus(out io.Writer, total int, errorMessages []string) {
	errCount := len(errorMessages)

	fmt.Fprintf(out, "Success: %d, Errors: %d\n", total-errCount, errCount)

	if len(errorMessages) > 0 {
		fmt.Fprintln(out, "Error details:")
		for _, message := range errorMessages {
			fmt.Fprintln(out, message)
		}
	}
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
