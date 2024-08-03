package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/cli/command/cfg"
	"github.com/d3witt/viking/cli/command/key"
	"github.com/d3witt/viking/cli/command/machine"
	"github.com/d3witt/viking/config"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
)

func main() {
	w := os.Stderr

	slog.SetDefault(slog.New(
		tint.NewHandler(colorable.NewColorable(w), &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
			NoColor:    !isatty.IsTerminal(w.Fd()),
		}),
	))

	c, err := config.ParseDefaultConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	vikingCli := &command.Cli{
		Config: &c,
		In:     os.Stdin,
		InFd:   int(os.Stdin.Fd()),
		Out:    os.Stdout,
		OutFd:  int(os.Stdout.Fd()),
		Err:    os.Stderr,
	}

	app := &cli.App{
		Name:    "viking",
		Usage:   "Manage your SSH keys and remote machines",
		Version: "v1.0",
		Commands: []*cli.Command{
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
