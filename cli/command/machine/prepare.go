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
		Name:        "prepare",
		Usage:       "Install Docker, set up Docker Swarm or join new machiens to the Swarm",
		Description: "This command will install Docker on all available machines, set up a Docker Swarm if it does not exist, or join new machines to the Swarm.",
		Action: func(ctx *cli.Context) error {
			return runPrepare(ctx.Context, vikingCli)
		},
	}
}

func runPrepare(ctx context.Context, vikingCli *command.Cli) error {
	clients, err := vikingCli.DialMachines(ctx)
	if err != nil {
		return fmt.Errorf("failed to dial machines: %w", err)
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	if err := checkDockerInstalled(ctx, vikingCli, clients); err != nil {
		return err
	}

	if err := ensureSwarm(ctx, vikingCli, clients); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Machines are ready to use.")
	return nil
}

func checkDockerInstalled(ctx context.Context, vikingCli *command.Cli, clients []*ssh.Client) error {
	err := parallel.RunFirstErr(context.Background(), len(clients), func(i int) error {
		client := clients[i]
		if !dockerhelper.IsDockerInstalled(client) {
			fmt.Fprintf(vikingCli.Out, "Docker is not installed on host %s. Installing...\n", client.RemoteAddr().String())
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
	swarm, err := dockerhelper.DialSwarmSSH(ctx, sshClients)
	if err != nil {
		return err
	}

	status, err := swarm.Status(ctx)
	if err != nil {
		return err
	}

	if len(status.Managers) == 0 && len(status.Workers) > 0 {
		return errors.New("no managers found in the swarm")
	}

	if len(status.Managers) == 0 && len(status.Workers) == 0 {
		fmt.Fprintln(vikingCli.Out, "No existing swarm found. Initializing new swarm...")
		if err := swarm.Init(ctx); err != nil {
			return err
		}
	} else {
		if len(status.Missing) > 0 {
			fmt.Fprintln(vikingCli.Out, "Joining missing nodes to the swarm...")

			if err := swarm.JoinNodes(ctx, status.Missing); err != nil {
				return err
			}
		}
	}

	status, err = swarm.Status(ctx)
	if err != nil {
		return err
	}

	if len(status.Managers) == 0 {
		return errors.New("No managers available to configure the Viking network.")
	}

	return ensureVikingNetwork(ctx, status.Managers[0])
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
