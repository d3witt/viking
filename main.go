package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/cli/command/app"
	"github.com/d3witt/viking/cli/command/cfg"
	"github.com/d3witt/viking/cli/command/key"
	"github.com/d3witt/viking/cli/command/machine"
	"github.com/d3witt/viking/config/userconf"
	"github.com/d3witt/viking/streams"
	"github.com/urfave/cli/v2"
)

var version = "dev" // set by build script

func main() {
	c, err := userconf.ParseDefaultConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	cmdLogger := slog.New(command.NewCmdLogHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	vikingCli := &command.Cli{
		Config:    c,
		In:        streams.StdIn,
		Out:       streams.StdOut,
		Err:       streams.StdErr,
		CmdLogger: cmdLogger,
	}

	app := &cli.App{
		Name:    "viking",
		Usage:   "Manage your SSH keys and remote machines",
		Version: version,
		Commands: []*cli.Command{
			app.NewInitCmd(vikingCli),
			// Often used commands
			machine.NewExecuteCmd(vikingCli),
			machine.NewCopyCmd(vikingCli),
			machine.NewApplyCmd(vikingCli),

			// Other commands
			key.NewCmd(vikingCli),
			machine.NewCmd(vikingCli),
			cfg.NewConfigCmd(vikingCli),
		},
		Suggest:   true,
		Reader:    vikingCli.In,
		Writer:    vikingCli.Out,
		ErrWriter: vikingCli.Err,
		ExitErrHandler: func(ctx *cli.Context, err error) {
			if err != nil {
				fmt.Fprintf(vikingCli.Err, "Error: %v\n", err)
				os.Exit(0)
			}
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
