package service

import (
	"context"
	"io"

	"github.com/d3witt/viking/cli/command"
	"github.com/docker/docker/api/types/container"
	"github.com/urfave/cli/v2"
)

func NewLogsCommand(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "logs",
		Usage: "Logs of a service",
		Args:  true,
		Action: func(ctx *cli.Context) error {
			machine := ctx.Args().First()
			service := ctx.Args().Get(1)

			return runLogs(ctx.Context, vikingCli, machine, service)
		},
	}
}

func runLogs(ctx context.Context, vikingCli *command.Cli, machine, service string) error {
	cl, err := vikingCli.DialManagerNode(ctx, machine)
	if err != nil {
		return err
	}
	defer func() {
		cl.Close()
		cl.SSH.Close()
	}()

	read, err := cl.ServiceLogs(ctx, service, container.LogsOptions{
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
