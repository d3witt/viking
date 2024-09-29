package machine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/d3witt/viking/parallel"
	"github.com/docker/docker/api/types/network"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewApplyCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "apply",
		Usage: "Apply the viking.toml configuration to the Swarm",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Automatically confirm the sync operation without prompting.",
			},
		},
		Action: func(ctx *cli.Context) error {
			yes := ctx.Bool("yes")

			return runApply(ctx.Context, vikingCli, yes)
		},
	}
}

func runApply(ctx context.Context, vikingCli *command.Cli, yes bool) error {
	clients, err := vikingCli.DialMachines(ctx)
	if err != nil {
		return fmt.Errorf("failed to dial machines: %w", err)
	}
	defer command.CloseSSHClients(clients)

	if err := checkDockerInstalled(ctx, vikingCli, clients); err != nil {
		return err
	}

	swarm, err := vikingCli.Swarm(ctx, clients)
	if err != nil {
		return err
	}
	defer swarm.Close()

	if !swarm.Exists(ctx) {
		fmt.Fprintf(vikingCli.Out, "Swarm is not initialized.")

		if !yes {
			confirmed, err := command.PromptForConfirmation(vikingCli.In, vikingCli.Out, "Do you want to initialize the Swarm?")
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
		}

		if err := swarm.Init(ctx); err != nil {
			return err
		}

		fmt.Fprintln(vikingCli.Out, "Swarm initialized.")
	}

	if err := swarm.Validate(ctx); err != nil {
		return fmt.Errorf("swarm validation failed: %w", err)
	}

	missing, err := swarm.GetMissingClients(ctx)
	if err != nil {
		return fmt.Errorf("failed to get missing clients: %w", err)
	}
	extra, err := swarm.GetExtraNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get extra clients: %w", err)
	}

	if len(missing) == 0 && len(extra) == 0 {
		fmt.Fprintln(vikingCli.Out, "Machines are already in sync.")
		return nil
	}

	message := "The following actions will be performed:\n"
	if len(missing) > 0 {
		hosts := getHosts(missing)
		message += fmt.Sprintf("  - Add the following machines to the Swarm: %s\n", strings.Join(hosts, ", "))
	}

	if len(extra) > 0 {
		message += fmt.Sprintf("  - Remove the following nodes from the Swarm: %s\n", strings.Join(extra, ", "))
	}

	if !yes {
		confirmed, err := command.PromptForConfirmation(vikingCli.In, vikingCli.Out, "Do you want to continue?")
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}

	if len(missing) > 0 {
		if err := swarm.JoinNodes(ctx, missing); err != nil {
			return fmt.Errorf("failed to add nodes to the Swarm: %w", err)
		}
	}

	if len(extra) > 0 {
		for _, node := range extra {
			if err := swarm.RemoveNodesByAddr(ctx, node, true); err != nil {
				return fmt.Errorf("failed to remove node %s from the Swarm: %w", node, err)
			}
		}
	}

	fmt.Fprintln(vikingCli.Out, "Machines are ready.")
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

func getHosts(clients []*dockerhelper.Client) []string {
	hosts := make([]string, len(clients))
	for i, client := range clients {
		hosts[i] = client.RemoteHost()
	}
	return hosts
}
