package service

import (
	"context"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
)

func NewRemoveCommand(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "remove",
		Usage: "Remove a service",
		Args:  true,
		Action: func(ctx *cli.Context) error {
			service := ctx.Args().First()

			return runRemove(ctx.Context, vikingCli, service)
		},
	}
}

func runRemove(ctx context.Context, vikingCli *command.Cli, service string) error {
	cl, err := vikingCli.DialManagerNode(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cl.Close()
		cl.SSH.Close()
	}()

	err = cl.ServiceRemove(ctx, service)
	if err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "Service %s removed.\n", service)
	return nil
}
