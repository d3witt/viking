package machine

import (
	"context"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
)

func NewPurgeCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "purge",
		Usage: "Leave Docker Swarm",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Automatically confirm the purge operation without prompting.",
			},
		},
		Action: func(ctx *cli.Context) error {
			yes := ctx.Bool("yes")

			return runPurge(ctx.Context, vikingCli, yes)
		},
	}
}

func runPurge(ctx context.Context, vikingCli *command.Cli, yes bool) error {
	if !yes {
		message := "You want to purge machine. Are you sure?"
		confirmed, err := command.PromptForConfirmation(vikingCli.In, vikingCli.Out, message)
		if err != nil {
			return err
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

	if err := dockerhelper.LeaveSwarm(ctx, dockerClient); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Swarm left.")
	return nil
}
