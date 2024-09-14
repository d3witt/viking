package machine

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
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
		ArgsUsage: "[APP|IP]:SRC_PATH DEST_PATH | SRC_PATH [APP|IP]:DEST_PATH",
		Action: func(ctx *cli.Context) error {
			args := ctx.Args()
			if args.Len() != 2 {
				return cli.ShowCommandHelp(ctx, "copy")
			}
			return runCopy(vikingCli, args.Get(0), args.Get(1))
		},
	}
}

func parseMachinePath(fullPath string) (machine, path string) {
	if strings.Contains(fullPath, ":") {
		parts := strings.SplitN(fullPath, ":", 2)
		return parts[0], parts[1]
	}

	return "", fullPath
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

	machine := fromMachine + toMachine

	var clients []*ssh.Client
	var err error

	if strings.ToLower(machine) == "app" {
		clients, err = vikingCli.DialMachines()
		if err != nil {
			return err
		}
		defer func() {
			for _, client := range clients {
				client.Close()
			}
		}()
	} else {
		client, err := vikingCli.DialMachine(machine)
		if err != nil {
			return err
		}
		defer client.Close()

		clients = []*ssh.Client{client}
	}

	if fromMachine != "" {
		return copyFromRemote(vikingCli, clients, fromPath, toPath)
	}

	return copyToRemote(vikingCli, clients, fromPath, toPath)
}

func copyToRemote(vikingCli *command.Cli, clients []*ssh.Client, from, to string) error {
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

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errorMessages []string

	wg.Add(len(clients))

	bar := copyProgressBar(
		vikingCli.Out,
		written*int64(len(clients)),
		"Sending",
	)

	for _, client := range clients {
		go func(client *ssh.Client) {
			defer wg.Done()

			// Open the temporary file for reading
			tmpFile, err := os.Open(tmpFile.Name())
			if err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("Error opening temporary file: %v", err))
				mu.Unlock()
				return
			}
			defer tmpFile.Close()

			// Create a multi-reader to read from the file and update the progress bar
			reader := io.TeeReader(tmpFile, bar)

			if err := archive.UntarRemote(client, to, reader); err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", client.RemoteAddr().String(), err))
				mu.Unlock()
				return
			}
		}(client)
	}

	wg.Wait()

	printCopyStatus(vikingCli.Out, len(clients), errorMessages)

	return nil
}

func copyFromRemote(vikingCli *command.Cli, clients []*ssh.Client, from, to string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errorMessages []string

	wg.Add(len(clients))

	bar := copyProgressBar(
		vikingCli.Out,
		-1,
		"Receiving",
	)

	for _, client := range clients {
		go func(client *ssh.Client) {
			defer wg.Done()

			dest := to
			if len(clients) > 1 {
				dest = path.Join(to, client.RemoteAddr().String())
			}

			data, err := archive.TarRemote(client, from)
			if err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", client.RemoteAddr().String(), err))
				mu.Unlock()
				return
			}

			reader := io.TeeReader(data, bar)

			buf := new(bytes.Buffer)
			_, err = buf.ReadFrom(reader)
			if err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", client.RemoteAddr().String(), err))
				mu.Unlock()
				return
			}

			if err := archive.Untar(buf, dest); err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("Error untar to %s: %v", dest, err))
				mu.Unlock()
				return
			}
		}(client)
	}

	wg.Wait()
	bar.Finish()

	printCopyStatus(vikingCli.Out, len(clients), errorMessages)

	return nil
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
