package main

import (
	"fmt"
	"log"
	"os"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/cli/command/cfg"
	"github.com/d3witt/viking/cli/command/key"
	"github.com/d3witt/viking/cli/command/machine"
	"github.com/d3witt/viking/config"
	"github.com/d3witt/viking/streams"
	"github.com/urfave/cli/v2"
)

func main() {
	c, err := config.ParseDefaultConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	vikingCli := &command.Cli{
		Config: &c,
		In:     streams.StdIn,
		Out:    streams.StdOut,
		Err:    streams.StdErr,
	}

	app := &cli.App{
		Name:    "viking",
		Usage:   "Manage your SSH keys and remote machines",
		Version: "v1.0",
		Commands: []*cli.Command{
			machine.NewExecuteCmd(vikingCli),
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
