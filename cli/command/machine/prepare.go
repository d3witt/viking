package machine

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewPrepareCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "prepare",
		Usage: "Install Docker, set up Docker Swarm, and launch Traefik",
		Action: func(ctx *cli.Context) error {
			return runPrepare(ctx.Context, vikingCli)
		},
	}
}

func runPrepare(ctx context.Context, vikingCli *command.Cli) error {
	clients, err := vikingCli.DialMachines()
	if err != nil {
		return err
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	fmt.Fprintln(vikingCli.Out, "Check that Docker is installed...")
	if err := checkDockerInstalled(vikingCli, clients); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Check that Docker Swarm is configured...")
	if err := checkSwarm(ctx, vikingCli, clients); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Viking cluster configured successfully.")
	return nil
}

func checkDockerInstalled(vikingCli *command.Cli, clients []*ssh.Client) error {
	for _, client := range clients {
		if !dockerhelper.IsDockerInstalled(client) {
			fmt.Fprintf(vikingCli.Out, "Installing Docker on %s...\n", client.RemoteAddr().String())
			if err := dockerhelper.InstallDocker(client); err != nil {
				return fmt.Errorf("could not install Docker on host %s: %w", client.RemoteAddr().String(), err)
			}
		}
	}

	return nil
}

func checkSwarm(ctx context.Context, vikingCli *command.Cli, clients []*ssh.Client) error {
	dockerClients := make([]*dockerhelper.Client, len(clients))
	defer func() {
		for _, client := range dockerClients {
			if client != nil {
				client.Close()
			}
		}
	}()

	for i, sshClient := range clients {
		dockerClient, err := dockerhelper.DialSSH(sshClient)
		if err != nil {
			return fmt.Errorf("%s: could not dial Docker: %w", sshClient.RemoteAddr().String(), err)
		}

		dockerClients[i] = dockerClient
	}

	managers, workers, missing, err := swarmStatus(ctx, dockerClients)
	if err != nil {
		return err
	}

	if len(managers) == 0 && len(workers) > 0 {
		return errors.New("Managers nodes are missing")
	} else if len(managers) == 0 && len(workers) == 0 {
		fmt.Fprintln(vikingCli.Out, "Swarm not found. Initializing new swarm...")
		if err = initSwarm(ctx, dockerClients); err != nil {
			return err
		}
	} else if len(missing) > 0 {
		if err := joinMissingHosts(ctx, vikingCli, managers[0], missing); err != nil {
			return err
		}
	}

	fmt.Fprintln(vikingCli.Out, "Checking Viking network...")
	if err := checkVikingNetwork(ctx, managers[0]); err != nil {
		return err
	}

	return nil
}

// findSwarm returns the swarm status if all clients are part of the same swarm.
// If the clients are not part of a swarm, it returns nil.
func swarmStatus(ctx context.Context, clients []*dockerhelper.Client) (
	managers []*dockerhelper.Client,
	workers []*dockerhelper.Client,
	missing []*dockerhelper.Client,
	err error,
) {
	managers = make([]*dockerhelper.Client, 0, len(clients))
	workers = make([]*dockerhelper.Client, 0, len(clients))
	missing = make([]*dockerhelper.Client, 0, len(clients))

	for _, cl := range clients {
		info, err := cl.Info(ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: could not get Docker info: %w", cl.SSH.RemoteAddr().String(), err)
		}

		switch info.Swarm.LocalNodeState {
		case swarm.LocalNodeStateInactive:
			missing = append(missing, cl)
			continue
		case swarm.LocalNodeStateActive:
			if info.Swarm.ControlAvailable {
				managers = append(managers, cl)
			} else {
				workers = append(workers, cl)
			}
		default:
			return nil, nil, nil, fmt.Errorf("%s: unexpected local node state: %s", cl.SSH.RemoteAddr().String(), info.Swarm.LocalNodeState)
		}
	}

	return
}

func initSwarm(ctx context.Context, clients []*dockerhelper.Client) error {
	if len(clients) == 0 {
		return fmt.Errorf("no hosts provided")
	}

	managersCount := len(clients)/2 + 1
	if managersCount > 7 {
		managersCount = 7
	}

	if _, err := clients[0].SwarmInit(ctx, swarm.InitRequest{
		ListenAddr: "0.0.0.0:2377",
		Spec: swarm.Spec{
			Annotations: swarm.Annotations{
				Labels: map[string]string{
					dockerhelper.SwarmLabel: "true",
				},
			},
		},
	}); err != nil {
		return err
	}

	sw, err := clients[0].SwarmInspect(ctx)
	if err != nil {
		return err
	}

	info, err := clients[0].Info(ctx)
	if err != nil {
		return err
	}

	managers := clients[1:managersCount]
	workers := clients[managersCount:]

	for _, manager := range managers {
		if err := manager.SwarmJoin(ctx, swarm.JoinRequest{
			ListenAddr: "0.0.0.0:2377",
			JoinToken:  sw.JoinTokens.Manager,
			RemoteAddrs: []string{
				net.JoinHostPort(info.Swarm.NodeAddr, "2377"),
			},
		}); err != nil {
			return fmt.Errorf("could not join manager %s to swarm: %v", manager.SSH.RemoteAddr().String(), err)
		}
	}

	for _, worker := range workers {
		if err := worker.SwarmJoin(ctx, swarm.JoinRequest{
			ListenAddr: "0.0.0.0:2377",
			JoinToken:  sw.JoinTokens.Worker,
			RemoteAddrs: []string{
				net.JoinHostPort(info.Swarm.NodeAddr, "2377"),
			},
		}); err != nil {
			return fmt.Errorf("could not join worker %s to swarm: %v", worker.SSH.RemoteAddr().String(), err)
		}
	}

	return nil
}

func joinMissingHosts(ctx context.Context, vikingCli *command.Cli, manager *dockerhelper.Client, missing []*dockerhelper.Client) error {
	info, err := manager.Info(ctx)
	if err != nil {
		return err
	}

	sw, err := manager.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	for _, cl := range missing {
		fmt.Fprintf(vikingCli.Out, "Joining %s to the swarm\n", cl.SSH.RemoteAddr().String())
		if err := cl.SwarmJoin(ctx, swarm.JoinRequest{
			ListenAddr: "0.0.0.0:2377",
			JoinToken:  sw.JoinTokens.Worker,
			RemoteAddrs: []string{
				net.JoinHostPort(info.Swarm.NodeAddr, "2377"),
			},
		}); err != nil {
			return fmt.Errorf("could not join worker %s to swarm: %v", cl.SSH.RemoteAddr().String(), err)
		}
	}

	return nil
}

func checkVikingNetwork(ctx context.Context, client *dockerhelper.Client) error {
	networks, err := client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.Name == dockerhelper.VikingNetworkName {
			return nil
		}
	}

	_, err = client.NetworkCreate(ctx, dockerhelper.VikingNetworkName, network.CreateOptions{
		Driver: "overlay",
	})
	return err
}
