package app

import (
	"context"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
)

func NewDeployCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "deploy",
		Usage:     "Deploy the app to the Swarm",
		Args:      true,
		ArgsUsage: "[IMAGE]",
		Action: func(ctx *cli.Context) error {
			image := ctx.Args().First()

			return runDeploy(ctx.Context, vikingCli, image)
		},
	}
}

func runDeploy(ctx context.Context, vikingCli *command.Cli, image string) error {
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

	replicas := conf.Replicas
	if replicas == 0 {
		replicas = 1
	}

	networks := conf.Networks
	if len(networks) == 0 {
		networks = []string{dockerhelper.VikingNetworkName}
	}

	if err := dockerhelper.Deploy(ctx, swarm, conf.Name, image, replicas, conf.Ports, networks, conf.Env, conf.Label); err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "App %s deployed to the Swarm.\n", conf.Name)

	return nil
}
