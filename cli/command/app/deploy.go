package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
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

	sshClient, err := vikingCli.DialMachine()
	if err != nil {
		return err
	}
	defer sshClient.Close()

	if err := prepare(ctx, vikingCli, sshClient); err != nil {
		return err
	}

	dockerClient, err := dockerhelper.DialSSH(sshClient)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	replicas := conf.Replicas
	if replicas == 0 {
		replicas = 1
	}

	networks := conf.Networks
	if len(networks) == 0 {
		networks = []string{dockerhelper.VikingNetworkName}
	}

	if err := dockerhelper.Deploy(ctx, dockerClient, conf.Name, image, replicas, conf.Ports, networks, conf.Env, conf.Label); err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "App %s deployed to the Swarm.\n", conf.Name)

	return nil
}

func prepare(ctx context.Context, vikingCli *command.Cli, sshClient *ssh.Client) error {
	if err := checkDockerInstalled(ctx, vikingCli, sshClient); err != nil {
		return err
	}

	dockerClient, err := dockerhelper.DialSSH(sshClient)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	inactive, err := dockerhelper.IsSwarmInactive(ctx, dockerClient)
	if err != nil {
		return fmt.Errorf("could not check Swarm status on host %s: %w", sshClient.RemoteAddr().String(), err)
	}
	if inactive {
		fmt.Fprintf(vikingCli.Out, "Swarm is not active on host %s. Initializing...\n", sshClient.RemoteAddr().String())
		host, _, err := net.SplitHostPort(sshClient.RemoteAddr().String())
		if err != nil {
			return fmt.Errorf("could not parse host address: %w", err)
		}

		if err := dockerhelper.InitSwarm(ctx, dockerClient, host); err != nil {
			slog.ErrorContext(ctx, "Failed to initialize Swarm", "machine", sshClient.RemoteAddr().String(), "error", err)
			return fmt.Errorf("could not initialize Swarm on host %s: %w", sshClient.RemoteAddr().String(), err)
		}
	}

	return dockerhelper.CreateNetworkIfNotExists(ctx, dockerClient, dockerhelper.VikingNetworkName)
}

func checkDockerInstalled(ctx context.Context, vikingCli *command.Cli, client *ssh.Client) error {
	if !dockerhelper.IsDockerInstalled(client) {
		fmt.Fprintf(vikingCli.Out, "Docker is not installed on host %s. Installing...\n", client.RemoteAddr().String())
		if err := dockerhelper.InstallDocker(client); err != nil {
			slog.ErrorContext(ctx, "Failed to install Docker", "machine", client.RemoteAddr().String(), "error", err)
			return fmt.Errorf("could not install Docker on host %s: %w", client.RemoteAddr().String(), err)
		}
	}
	return nil
}
