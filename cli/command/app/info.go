package app

import (
	"context"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
)

func NewInfoCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Display information about the app",
		Action: func(ctx *cli.Context) error {
			return runInfo(ctx.Context, vikingCli)
		},
	}
}

func runInfo(ctx context.Context, vikingCli *command.Cli) error {
	var info struct {
		AppName  string
		Status   string
		Image    string
		Replicas uint64
	}

	conf, err := vikingCli.AppConfig()
	if err != nil {
		return fmt.Errorf("failed to get app config: %w", err)
	}
	info.AppName = conf.Name

	sshClients, err := vikingCli.DialMachines(ctx)
	if err != nil {
		return fmt.Errorf("failed to dial machines: %w", err)
	}
	defer command.CloseSSHClients(sshClients)

	swarm, err := vikingCli.Swarm(ctx, sshClients)
	if err != nil {
		return fmt.Errorf("failed to get swarm: %w", err)
	}
	defer swarm.Close()

	service, err := dockerhelper.GetService(ctx, swarm, conf.Name)
	if err != nil {
		return fmt.Errorf("failed to get service info: %w", err)
	}

	if service == nil {
		info.Status = "Not deployed"
	} else {
		info.Status = "Deployed"
		info.Image = service.Spec.TaskTemplate.ContainerSpec.Image
		info.Replicas = *service.Spec.Mode.Replicated.Replicas
	}

	fmt.Fprintf(vikingCli.Out, "App Name: %s\n", info.AppName)
	fmt.Fprintf(vikingCli.Out, "Status: %s\n", info.Status)
	if info.Status == "Deployed" {
		fmt.Fprintf(vikingCli.Out, "Image: %s\n", info.Image)
		fmt.Fprintf(vikingCli.Out, "Replicas: %d\n", info.Replicas)
	}

	return nil
}
