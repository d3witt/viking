package service

import (
	"context"
	"fmt"
	"net"
	"slices"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewRunCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "deploy",
		Usage: "Deploy container",
		Args:  true,
		Action: func(c *cli.Context) error {
			machine := c.Args().First()
			return runDeploy(c.Context, vikingCli, machine)
		},
	}
}

func runDeploy(ctx context.Context, vikingCli *command.Cli, machine string) error {
	clients, err := vikingCli.DialMachine(machine)
	if err != nil {
		return err
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	if err := ensureDockerInstalled(vikingCli, clients); err != nil {
		return err
	}

	if err := ensureSwarm(ctx, vikingCli, clients); err != nil {
		return err
	}

	return nil
}

const (
	swarmLabel        = "viking-swarm"
	vikingNetworkName = "viking-network"
)

func ensureDockerInstalled(vikingCli *command.Cli, clients []*ssh.Client) error {
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

func ensureSwarm(ctx context.Context, vikingCli *command.Cli, clients []*ssh.Client) error {
	dockerClients := make([]*dockerhelper.Client, len(clients))
	for i, sshClient := range clients {
		dockerClient, err := dockerhelper.DialSSH(sshClient)
		if err != nil {
			return fmt.Errorf("could not create Docker client: %w", err)
		}

		dockerClients[i] = dockerClient
	}
	defer func() {
		for _, client := range dockerClients {
			client.Close()
		}
	}()

	status, nodes, err := getSwarmStatus(ctx, dockerClients)
	if err != nil {
		return err
	}

	if status == nil {
		status, nodes, err = initSwarm(ctx, dockerClients)
		if err != nil {
			return err
		}
	}

	if err := joinMissingHosts(ctx, vikingCli, dockerClients, status, nodes); err != nil {
		return err
	}

	for _, dockerClient := range dockerClients {
		if err := ensureVikingNetwork(ctx, dockerClient); err != nil {
			return fmt.Errorf("could not ensure Viking network on host %s: %v", dockerClient.SSH.RemoteAddr().String(), err)
		}
	}

	return nil
}

func getSwarmStatus(ctx context.Context, clients []*dockerhelper.Client) (*swarm.Swarm, []swarm.Node, error) {
	var status *swarm.Swarm
	var swarmAddr string
	nodes := make([]swarm.Node, 0)

	for _, dockerClient := range clients {
		s, err := dockerClient.SwarmInspect(ctx)
		if err != nil {
			if client.IsErrConnectionFailed(err) {
				return nil, nil, err
			}
			continue
		}

		if status != nil && s.ID != status.ID {
			return nil, nil, fmt.Errorf("%s and %s are part of different swarms", swarmAddr, dockerClient.SSH.RemoteAddr().String())
		}

		if status != nil {
			continue
		}

		status = &s
		swarmAddr = dockerClient.SSH.RemoteAddr().String()

		nodes, err = dockerClient.NodeList(ctx, types.NodeListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("could not list nodes: %v", err)
		}
	}

	return status, nodes, nil
}

func initSwarm(ctx context.Context, clients []*dockerhelper.Client) (*swarm.Swarm, []swarm.Node, error) {
	if len(clients) == 0 {
		return nil, nil, fmt.Errorf("no hosts provided")
	}

	managersCount := len(clients)/2 + 1

	if _, err := clients[0].SwarmInit(ctx, swarm.InitRequest{
		ListenAddr: "0.0.0.0:2377",
		Spec: swarm.Spec{
			Annotations: swarm.Annotations{
				Labels: map[string]string{
					swarmLabel: "true",
				},
			},
		},
	}); err != nil {
		return nil, nil, err
	}

	status, err := clients[0].SwarmInspect(ctx)
	if err != nil {
		if client.IsErrConnectionFailed(err) {
			return nil, nil, err
		}

		return nil, nil, fmt.Errorf("failed to initialize swarm: %v", err)
	}
	swarmHost, _, err := net.SplitHostPort(clients[0].SSH.RemoteAddr().String())
	if err != nil {
		return nil, nil, fmt.Errorf("could not parse remote address: %v", err)
	}
	swarmAddr := net.JoinHostPort(swarmHost, "2377")

	other := clients[1:]
	managers := other[:managersCount-1]
	workers := other[managersCount-1:]

	for _, manager := range managers {
		if err := manager.SwarmJoin(ctx, swarm.JoinRequest{
			ListenAddr: "0.0.0.0:2377",
			JoinToken:  status.JoinTokens.Manager,
			RemoteAddrs: []string{
				swarmAddr,
			},
		}); err != nil {
			return nil, nil, fmt.Errorf("could not join manager %s to swarm: %v", manager.SSH.RemoteAddr().String(), err)
		}
	}

	for _, worker := range workers {
		if err := worker.SwarmJoin(ctx, swarm.JoinRequest{
			JoinToken: status.JoinTokens.Worker,
			RemoteAddrs: []string{
				swarmAddr,
			},
		}); err != nil {
			return nil, nil, fmt.Errorf("could not join worker %s to swarm: %v", worker.SSH.RemoteAddr().String(), err)
		}
	}

	nodes, err := clients[0].NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("could not list nodes: %v", err)
	}

	return &status, nodes, nil
}

func joinMissingHosts(ctx context.Context, vikingCli *command.Cli, clients []*dockerhelper.Client, status *swarm.Swarm, nodes []swarm.Node) error {
	leaderIdx := slices.IndexFunc(nodes, func(node swarm.Node) bool {
		return node.ManagerStatus != nil && node.ManagerStatus.Leader
	})
	if leaderIdx == -1 {
		return fmt.Errorf("no leader found")
	}

	leader := nodes[leaderIdx]

	for _, client := range clients {
		found := false

		ip, _, err := net.SplitHostPort(client.SSH.RemoteAddr().String())
		if err != nil {
			return fmt.Errorf("could not parse node address: %v", err)
		}

		for _, node := range nodes {
			if node.Status.Addr == ip {
				found = true
				break
			}
		}

		if found {
			continue
		}

		fmt.Fprintf(vikingCli.Out, "Joining %s to the swarm\n", client.SSH.RemoteAddr().String())
		if err := client.SwarmJoin(ctx, swarm.JoinRequest{
			JoinToken: status.JoinTokens.Worker,
			RemoteAddrs: []string{
				net.JoinHostPort(leader.ManagerStatus.Addr, "2377"),
			},
		}); err != nil {
			return fmt.Errorf("could not join worker %s to swarm: %v", client.SSH.RemoteAddr().String(), err)
		}
	}

	return nil
}

func ensureVikingNetwork(ctx context.Context, client *dockerhelper.Client) error {
	networks, err := client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.Name == vikingNetworkName {
			return nil
		}
	}

	_, err = client.NetworkCreate(ctx, vikingNetworkName, network.CreateOptions{
		Driver: "overlay",
	})
	return err
}
