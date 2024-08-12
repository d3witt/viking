package machine

import (
	"fmt"
	"net/mail"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config"
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

	fmt.Fprintln(vikingCli.Out, "Verifying Docker install...")
	if installed := dockerhelper.IsDockerInstalled(exec); !installed {
		fmt.Fprintln(vikingCli.Out, "Docker is not installed. Installing Docker...")
		if err := dockerhelper.InstallDocker(exec); err != nil {
			return err
		}
	}

	fmt.Fprintln(vikingCli.Out, "Verifying Docker Swarm mode is active...")
	if initialized := dockerhelper.IsSwarmInitialized(exec); !initialized {
		fmt.Fprintln(vikingCli.Out, "Docker Swarm mode is not active. Initializing Docker Swarm mode...")

		if err := dockerhelper.InitDockerSwarm(exec); err != nil {
			return err
		}
	}

	return nil
}

func validEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func getCertEmail(cli *command.Cli) (string, error) {
	profile := cli.Config.GetDefaultProfile()

	if profile.Email != "" {
		return profile.Email, nil
	}

	email, err := command.Prompt(cli.In, cli.Out, "Enter an email that will be used for SSL certificate", "")
	if err != nil {
		return "", err
	}

	if !validEmail(email) {
		fmt.Printf("error!")
		return "", fmt.Errorf("Please, enter a valid email address")
	}

	if err := cli.Config.SetDefaultProfile(config.Profile{
		Email: email,
	}); err != nil {
		return "", err
	}

	return email, nil
}
