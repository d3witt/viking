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
			machine := ctx.Args().First()
			service := ctx.Args().Get(1)

			return runRemove(ctx.Context, vikingCli, machine, service)
		},
	}
}

func runRemove(ctx context.Context, vikingCli *command.Cli, machine, service string) error {
	cl, err := vikingCli.DialManagerNode(ctx, machine)
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

	fmt.Fprintf(vikingCli.Out, "Service %s removed from machine %s.\n", service, machine)
	return nil
}
