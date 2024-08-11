package machine

import (
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
)

func NewDeployCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "deploy",
		Usage:     "Deploy docker container to a machine",
		Args:      true,
		ArgsUsage: "NAME",
		Action: func(ctx *cli.Context) error {
			machine := ctx.Args().First()

			return runDeploy(vikingCli, machine)
		},
	}
}

func runDeploy(vikingCli *command.Cli, machine string) error {
	execs, err := vikingCli.MachineExecuters(machine)
	if err != nil {
		return err
	}

	if len(execs) != 1 {
		return fmt.Errorf("cannot deploy to multiple hosts")
	}

	exec := execs[0]
	defer exec.Close()

	exec.SetLogger(vikingCli.CmdLogger)

	fmt.Fprintf(vikingCli.Out, "Checking if Docker is installed on %s...\n", machine)
	if installed := dockerhelper.IsDockerInstalled(exec); !installed {
		fmt.Fprintf(vikingCli.Out, "Docker is not installed on %s. Installing Docker...\n", machine)
	}

	fmt.Fprintf(vikingCli.Out, "Checking if Docker Swarm mode is active on %s...\n", machine)
	if initialized := dockerhelper.IsSwarmInitialized(exec); !initialized {
		fmt.Fprintf(vikingCli.Out, "Docker Swarm mode is not active on %s. Initializing Docker Swarm mode...\n", machine)

		if err := dockerhelper.InitDockerSwarm(exec); err != nil {
			return err
		}
	}

	return nil
}
