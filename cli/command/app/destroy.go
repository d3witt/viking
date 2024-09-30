package app

import (
	"context"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
)

func NewDestroyCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "destroy",
		Usage: "Destroy the app and remove it from the Swarm",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Skip confirmation prompt",
			},
		},
		Action: func(ctx *cli.Context) error {
			yes := ctx.Bool("yes")

			return runDestroy(ctx.Context, vikingCli, yes)
		},
	}
}

func runDestroy(ctx context.Context, vikingCli *command.Cli, yes bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return fmt.Errorf("failed to get app config: %w", err)
	}

	if !yes {
		confirmed, err := command.PromptForConfirmation(vikingCli.In, vikingCli.Out, fmt.Sprintf("Are you sure you want to destroy the app '%s'?", conf.Name))
		if err != nil {
			return fmt.Errorf("failed to prompt for confirmation: %w", err)
		}
		if !confirmed {
			return nil
		}
	}

	sshClient, err := vikingCli.DialMachine()
	if err != nil {
		return err
	}
	defer sshClient.Close()

	dockerClient, err := dockerhelper.DialSSH(sshClient)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	if err := dockerClient.ServiceRemove(ctx, conf.Name); err != nil {
		return fmt.Errorf("failed to remove service %s: %w", conf.Name, err)
	}

	fmt.Fprintf(vikingCli.Out, "App %s destroyed and removed from the Swarm.\n", conf.Name)

	return nil
}
