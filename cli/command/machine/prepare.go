package machine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/d3witt/viking/parallel"
	"github.com/docker/docker/api/types/network"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewPrepareCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "prepare",
		Usage: "Install Docker, set up Docker Swarm or join new machiens to the Swarm",
		Action: func(ctx *cli.Context) error {
			return runPrepare(ctx.Context, vikingCli)
		},
	}
}

func runPrepare(ctx context.Context, vikingCli *command.Cli) error {
	clients, err := vikingCli.DialMachines()
	if err != nil {
		return fmt.Errorf("failed to dial machines: %w", err)
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	slog.InfoContext(ctx, "Checking if Docker is installed on all machines")
	if err := checkDockerInstalled(ctx, clients); err != nil {
		return err
	}

	slog.InfoContext(ctx, "Ensuring Docker Swarm is configured")
	if err := ensureSwarm(ctx, vikingCli, clients); err != nil {
		return err
	}

	slog.InfoContext(ctx, "Machines are ready")
	return nil
}

func checkDockerInstalled(ctx context.Context, clients []*ssh.Client) error {
	err := parallel.RunFirstErr(context.Background(), len(clients), func(i int) error {
		client := clients[i]
		if !dockerhelper.IsDockerInstalled(client) {
			slog.InfoContext(ctx, "Installing Docker", "machine", client.RemoteAddr().String())
			if err := dockerhelper.InstallDocker(client); err != nil {
				slog.ErrorContext(ctx, "Failed to install Docker", "machine", client.RemoteAddr().String(), "error", err)
				return fmt.Errorf("could not install Docker on host %s: %w", client.RemoteAddr().String(), err)
			}
		}
		return nil
	})
	if err != nil {
		return errors.New("failed to install Docker")
	}

	return nil
}

func ensureSwarm(ctx context.Context, vikingCli *command.Cli, sshClients []*ssh.Client) error {
	dockerClients, err := createDockerClients(ctx, vikingCli, sshClients)
	if err != nil {
		return err
	}
	defer closeDockerClients(dockerClients)

	status, err := dockerhelper.SwarmStatus(ctx, dockerClients)
	if err != nil {
		return err
	}

	if len(status.Managers) == 0 && len(status.Workers) > 0 {
		return errors.New("no managers found in the swarm")
	}

	if len(status.Managers) == 0 && len(status.Workers) == 0 {
		fmt.Fprintln(vikingCli.Out, "No existing swarm found. Initializing new swarm...")
		if err := dockerhelper.InitSwarm(ctx, dockerClients); err != nil {
			return err
		}
	} else {
		if len(status.Missing) > 0 {
			fmt.Fprintln(vikingCli.Out, "Joining missing nodes to the swarm...")

			if err := dockerhelper.JoinNodes(ctx, status.Managers[0], status.Missing); err != nil {
				return err
			}
		}
	}

	status, err = dockerhelper.SwarmStatus(ctx, dockerClients)
	if err != nil {
		return err
	}

	if len(status.Managers) == 0 {
		return errors.New("No managers available to configure the Viking network.")
	}

	fmt.Fprintln(vikingCli.Out, "Checking Viking network...")
	return ensureVikingNetwork(ctx, status.Managers[0])
}

func createDockerClients(ctx context.Context, vikingCli *command.Cli, sshClients []*ssh.Client) ([]*dockerhelper.Client, error) {
	dockerClients := make([]*dockerhelper.Client, len(sshClients))

	err := parallel.RunFirstErr(ctx, len(sshClients), func(i int) error {
		dockerClient, err := dockerhelper.DialSSH(sshClients[i])
		if err != nil {
			return fmt.Errorf("%s: could not dial Docker: %w", sshClients[i].RemoteAddr().String(), err)
		}
		dockerClients[i] = dockerClient
		return nil
	})
	if err != nil {
		fmt.Fprintln(vikingCli.Err, err)
		return nil, errors.New("failed to dial Docker on all machines")
	}

	return dockerClients, nil
}

func ensureVikingNetwork(ctx context.Context, client *dockerhelper.Client) error {
	networks, err := client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, network := range networks {
		if network.Name == dockerhelper.VikingNetworkName {
			return nil
		}
	}

	_, err = client.NetworkCreate(ctx, dockerhelper.VikingNetworkName, network.CreateOptions{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w", dockerhelper.VikingNetworkName, err)
	}
	return nil
}
