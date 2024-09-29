package app

import (
	"context"
	"io"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/docker/docker/api/types/container"
	"github.com/urfave/cli/v2"
)

func NewLogsCommand(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "logs",
		Usage: "Logs of a app",
		Args:  true,
		Action: func(ctx *cli.Context) error {
			return runLogs(ctx.Context, vikingCli)
		},
	}
}

func runLogs(ctx context.Context, vikingCli *command.Cli) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	sshClients, err := vikingCli.DialMachines(ctx)
	if err != nil {
		return err
	}

	swarm, err := vikingCli.Swarm(ctx, sshClients)
	if err != nil {
		return err
	}

	read, err := dockerhelper.ServiceLogs(ctx, swarm, conf.Name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(vikingCli.Out, read)
	if err != nil {
		return err
	}
	err = read.Close()
	if err != nil {
		return err
	}
	return nil
}
