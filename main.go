package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
	"github.com/workdate-dev/viking/cli/command"
	"github.com/workdate-dev/viking/cli/command/key"
	"github.com/workdate-dev/viking/config"
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

	cfg, err := config.ParseDefaultConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	vikingCli := &command.Cli{
		Config: &cfg,
		In:     os.Stdin,
		Out:    os.Stdout,
		Err:    os.Stderr,
	}

	app := &cli.App{
		Name:    "viking",
		Usage:   "Run multiple apps on single machine.",
		Version: "v1.0",
		Commands: []*cli.Command{
			key.NewCmd(vikingCli),
		},
		Suggest:   true,
		Reader:    vikingCli.In,
		Writer:    vikingCli.Out,
		ErrWriter: vikingCli.Err,
		ExitErrHandler: func(ctx *cli.Context, err error) {
			if err != nil {
				fmt.Fprintln(ctx.App.Writer, err)
				os.Exit(0)
			}
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
