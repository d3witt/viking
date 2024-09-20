package machine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

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

	fmt.Fprintln(vikingCli.Out, "Checking if Docker is installed on all machines...")
	if err := checkDockerInstalled(vikingCli, clients); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Ensuring Docker Swarm is configured properly...")
	if err := ensureSwarm(ctx, vikingCli, clients); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Viking configured successfully.")
	return nil
}

func checkDockerInstalled(vikingCli *command.Cli, clients []*ssh.Client) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, client := range clients {
		wg.Add(1)
		go func(client *ssh.Client) {
			defer wg.Done()

			if !dockerhelper.IsDockerInstalled(client) {
				fmt.Fprintf(vikingCli.Out, "Installing Docker on %s...\n", client.RemoteAddr().String())
				if err := dockerhelper.InstallDocker(client); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("could not install Docker on host %s: %w", client.RemoteAddr().String(), err))
					mu.Unlock()
				}
			}
		}(client)
	}

	wg.Wait()

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(vikingCli.Err, err)
		}
		return errors.New("failed to install Docker on all machines")
	}

	return nil
}

func ensureSwarm(ctx context.Context, vikingCli *command.Cli, sshClients []*ssh.Client) error {
	dockerClients, err := createDockerClients(vikingCli, sshClients)
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
		if err := initializeSwarm(ctx, dockerClients); err != nil {
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

func createDockerClients(vikingCli *command.Cli, sshClients []*ssh.Client) ([]*dockerhelper.Client, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	dockerClients := make([]*dockerhelper.Client, len(sshClients))

	for i, sshClient := range sshClients {
		wg.Add(1)
		go func(i int, sshClient *ssh.Client) {
			defer wg.Done()
			dockerClient, err := dockerhelper.DialSSH(sshClient)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: could not dial Docker: %w", sshClient.RemoteAddr().String(), err))
				mu.Unlock()
				return
			}
			dockerClients[i] = dockerClient
		}(i, sshClient)
	}

	wg.Wait()

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(vikingCli.Err, err)
		}
		return nil, errors.New("failed to dial Docker on all machines")
	}

	return dockerClients, nil
}

func initializeSwarm(ctx context.Context, clients []*dockerhelper.Client) error {
	if len(clients) == 0 {
		return errors.New("no clients available to initialize the swarm")
	}

	leader := clients[0]
	host, _, err := net.SplitHostPort(leader.SSH.RemoteAddr().String())
	if err != nil {
		return fmt.Errorf("failed to extract hostname for leader: %w", err)
	}

	_, err = leader.SwarmInit(ctx, swarm.InitRequest{
		ListenAddr:    "0.0.0.0:2377",
		AdvertiseAddr: host,
		Spec: swarm.Spec{
			Annotations: swarm.Annotations{
				Labels: map[string]string{
					dockerhelper.SwarmLabel: "true",
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize swarm on leader %s: %w", host, err)
	}

	return dockerhelper.JoinNodes(ctx, leader, clients[1:])
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
