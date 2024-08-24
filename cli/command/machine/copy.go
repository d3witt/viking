package machine

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/d3witt/viking/archive"
	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/sshexec"
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

	// Create a temporary file to store the tar achive
	tmpFile, err := os.CreateTemp("", "archive-*.tar")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	// Write the tar archive to the temporary file
	if _, err := io.Copy(tmpFile, data); err != nil {
		return err
	}

	// Close the temporary file to flush the data
	if err := tmpFile.Close(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(execs))

	for _, exec := range execs {
		go func(exec sshexec.Executor) {
			defer wg.Done()

			out := vikingCli.Out
			errOut := vikingCli.Err
			if len(execs) > 1 {
				prefix := fmt.Sprintf("%s: ", exec.Addr())
				out = out.WithPrefix(prefix)
				errOut = errOut.WithPrefix(prefix + "error: ")
			}

			// Open the temporary file for reading
			tmpFile, err := os.Open(tmpFile.Name())
			if err != nil {
				fmt.Fprintln(errOut, err)
				return
			}
			defer tmpFile.Close()

			if err := archive.UntarRemote(exec, to, tmpFile); err != nil {
				fmt.Fprintln(errOut, err)
				return
			}

			fmt.Fprintf(out, "Successfully copied to %s\n", exec.Addr()+":"+to)
		}(exec)
	}

	wg.Wait()

	return nil
}

func copyFromRemote(vikingCli *command.Cli, execs []sshexec.Executor, from, to string) error {
	var wg sync.WaitGroup
	wg.Add(len(execs))

	for _, exec := range execs {
		go func(exec sshexec.Executor) {
			defer wg.Done()

			dest := to

			out := vikingCli.Out
			errOut := vikingCli.Err
			if len(execs) > 1 {
				dest = path.Join(to, exec.Addr())

				prefix := fmt.Sprintf("%s: ", exec.Addr())
				out = out.WithPrefix(prefix)
				errOut = errOut.WithPrefix(prefix + "error: ")
			}

			data, err := archive.TarRemote(exec, from)
			if err != nil {
				fmt.Fprintln(errOut, err)
				return
			}

			buf := new(bytes.Buffer)
			buf.ReadFrom(data)

			if err := archive.Untar(buf, dest); err != nil {
				fmt.Fprintln(errOut, err)

				return
			}

			fmt.Fprintf(out, "Successfully copied to %s\n", dest)
		}(exec)
	}

	wg.Wait()
	return nil
}
