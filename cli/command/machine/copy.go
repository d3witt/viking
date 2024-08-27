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
	"github.com/d3witt/viking/sshexec"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
)

func NewCopyCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "copy",
		Aliases:   []string{"cp"},
		Usage:     "Copy files/folders between local and remote machine",
		Args:      true,
		ArgsUsage: "MACHINE:SRC_PATH DEST_PATH | SRC_PATH MACHINE:DEST_PATH",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() != 2 {
				return fmt.Errorf("expected 2 arguments, got %d", ctx.NArg())
			}

			return runCopy(vikingCli, ctx.Args().Get(0), ctx.Args().Get(1))
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

	execs, err := vikingCli.MachineExecuters(machine)
	defer func() {
		for _, exec := range execs {
			exec.Close()
		}
	}()

	if err != nil {
		return err
	}

	if fromMachine != "" {
		return copyFromRemote(vikingCli, execs, fromPath, toPath)
	}

	return copyToRemote(vikingCli, execs, fromPath, toPath)
}

func copyToRemote(vikingCli *command.Cli, execs []sshexec.Executor, from, to string) error {
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

	wg.Add(len(execs))

	bar := copyProgressBar(
		vikingCli.Out,
		written*int64(len(execs)),
		"Sending",
	)

	for _, exec := range execs {
		go func(exec sshexec.Executor) {
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

			if err := archive.UntarRemote(exec, to, reader); err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", exec.Addr(), err))
				mu.Unlock()
				return
			}
		}(exec)
	}

	wg.Wait()

	printCopyStatus(vikingCli.Out, len(execs), errorMessages)

	return nil
}

func copyFromRemote(vikingCli *command.Cli, execs []sshexec.Executor, from, to string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errorMessages []string

	wg.Add(len(execs))

	bar := copyProgressBar(
		vikingCli.Out,
		-1,
		"Receiving",
	)

	for _, exec := range execs {
		go func(exec sshexec.Executor) {
			defer wg.Done()

			dest := to
			if len(execs) > 1 {
				dest = path.Join(to, exec.Addr())
			}

			data, err := archive.TarRemote(exec, from)
			if err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", exec.Addr(), err))
				mu.Unlock()
				return
			}

			reader := io.TeeReader(data, bar)

			buf := new(bytes.Buffer)
			_, err = buf.ReadFrom(reader)
			if err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", exec.Addr(), err))
				mu.Unlock()
				return
			}

			if err := archive.Untar(buf, dest); err != nil {
				mu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("Error untar to %s: %v", dest, err))
				mu.Unlock()
				return
			}
		}(exec)
	}

	wg.Wait()
	bar.Finish()

	printCopyStatus(vikingCli.Out, len(execs), errorMessages)

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
