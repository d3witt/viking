package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/docker/docker/api/types/container"
	"github.com/urfave/cli/v2"
)

func NewLogsCommand(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "logs",
		Usage: "Fetch the logs",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "tail",
				Aliases: []string{"n"},
				Usage:   "Number of lines to show from the end of the logs",
				Value:   100,
			},
			&cli.StringFlag{
				Name:  "since",
				Usage: "Show logs since timestamp (e.g. 2013-01-02T13:23:37Z) or relative (e.g. 42m for 42 minutes)",
				Value: "",
			},
			&cli.BoolFlag{
				Name:    "follow",
				Aliases: []string{"f"},
				Usage:   "Follow log output",
				Value:   false,
			},
		},
		Action: func(ctx *cli.Context) error {
			return runLogs(ctx.Context, vikingCli, ctx.Int("tail"), ctx.String("since"), ctx.Bool("follow"))
		},
	}
}

func runLogs(ctx context.Context, vikingCli *command.Cli, tail int, since string, follow bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	sshClient, err := vikingCli.DialMachine()
	if err != nil {
		return fmt.Errorf("dial machine: %v", err)
	}
	defer sshClient.Close()

	dockerClient, err := dockerhelper.DialSSH(sshClient)
	if err != nil {
		return fmt.Errorf("dial Docker: %v", err)
	}
	defer dockerClient.Close()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       fmt.Sprintf("%d", tail),
	}

	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			options.Since = t.Format(time.RFC3339Nano)
		} else if duration, err := time.ParseDuration(since); err == nil {
			options.Since = time.Now().Add(-duration).Format(time.RFC3339Nano)
		} else {
			return fmt.Errorf("invalid since format: %s", since)
		}
	}

	reader, err := dockerClient.ServiceLogs(ctx, conf.Name, options)
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.Copy(vikingCli.Out, reader)
	return err
}
