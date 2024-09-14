package service

import (
	"context"
	"strconv"

	"github.com/d3witt/viking/cli/command"
	"github.com/docker/docker/api/types"
	"github.com/urfave/cli/v2"
)

func NewListCommand(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "List all services",
		Aliases: []string{"ls"},
		Action: func(ctx *cli.Context) error {
			return listServices(ctx.Context, vikingCli)
		},
	}
}

func listServices(ctx context.Context, vikingCli *command.Cli) error {
	cl, err := vikingCli.DialManagerNode(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cl.Close()
		cl.SSH.Close()
	}()

	services, err := cl.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return err
	}

	data := [][]string{
		{"ID", "NAME", "REPLICAS", "IMAGE"},
	}
	for _, service := range services {
		data = append(data, []string{
			service.ID,
			service.Spec.Name,
			strconv.FormatUint(uint64(*service.Spec.Mode.Replicated.Replicas), 10),
			service.Spec.TaskTemplate.ContainerSpec.Image,
		})
	}

	command.PrintTable(vikingCli.Out, data)
	return nil
}
