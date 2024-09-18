package machine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/docker/docker/api/types"
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

	missing, extraNodes, _, manager, err := getSwarmStatus(ctx, dockerClients)
	if err != nil {
		return err
	}

	if manager == nil {
		fmt.Fprintln(vikingCli.Out, "No existing swarm found. Initializing new swarm...")
		if err := initializeSwarm(ctx, vikingCli, dockerClients); err != nil {
			return err
		}
	} else {
		if len(missing) > 0 {
			fmt.Fprintln(vikingCli.Out, "Joining missing nodes to the swarm...")

			if err := joinNodes(ctx, vikingCli, manager, missing); err != nil {
				return err
			}
		}
		if len(extraNodes) > 0 {
			if err := removeExtraNodes(ctx, vikingCli, manager, extraNodes); err != nil {
				return err
			}
		}
	}

	// Refresh swarm status after changes
	_, _, swarmNodes, manager, err := getSwarmStatus(ctx, dockerClients)
	if err != nil {
		return err
	}

	// Rebalance managers
	if err := rebalanceManagers(ctx, vikingCli, manager, swarmNodes); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Checking Viking network...")
	if err := ensureVikingNetwork(ctx, manager); err != nil {
		return err
	}

	return nil
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

func closeDockerClients(clients []*dockerhelper.Client) {
	for _, client := range clients {
		if client != nil {
			client.Close()
		}
	}
}

func getSwarmStatus(ctx context.Context, clients []*dockerhelper.Client) (
	missing []*dockerhelper.Client,
	extraNodes []swarm.Node,
	swarmNodes []swarm.Node,
	manager *dockerhelper.Client,
	err error,
) {
	desiredAddrs := make(map[string]*dockerhelper.Client)
	for _, cl := range clients {
		addr := cl.SSH.RemoteAddr().String()

		if strings.Contains(addr, ":") {
			addr = strings.Split(addr, ":")[0]
		}
		desiredAddrs[addr] = cl
	}

	var managerClient *dockerhelper.Client
	for _, cl := range clients {
		info, err := cl.Info(ctx)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("%s: could not get Docker info: %w", cl.SSH.RemoteAddr().String(), err)
		}
		if info.Swarm.ControlAvailable {
			managerClient = cl
			break
		}
	}

	if managerClient == nil {
		swarmNodes = []swarm.Node{}
	} else {
		swarmNodes, err = managerClient.NodeList(ctx, types.NodeListOptions{})
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to list swarm nodes: %w", err)
		}
	}

	swarmNodeAddrs := make(map[string]swarm.Node)
	for _, node := range swarmNodes {
		addr := node.Status.Addr
		swarmNodeAddrs[addr] = node
	}

	for addr, cl := range desiredAddrs {
		if _, exists := swarmNodeAddrs[addr]; !exists {
			missing = append(missing, cl)
		}
	}

	for addr, node := range swarmNodeAddrs {
		if _, exists := desiredAddrs[addr]; !exists {
			extraNodes = append(extraNodes, node)
		}
	}

	return missing, extraNodes, swarmNodes, managerClient, nil
}

func initializeSwarm(ctx context.Context, vikingCli *command.Cli, clients []*dockerhelper.Client) error {
	if len(clients) == 0 {
		return errors.New("no clients available to initialize the swarm")
	}

	leader := clients[0]
	err := retry(func() error {
		_, err := leader.SwarmInit(ctx, swarm.InitRequest{
			ListenAddr:    "0.0.0.0:2377",
			AdvertiseAddr: leader.SSH.RemoteAddr().String(),
			Spec: swarm.Spec{
				Annotations: swarm.Annotations{
					Labels: map[string]string{
						dockerhelper.SwarmLabel: "true",
					},
				},
			},
		})
		return err
	}, 3, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to initialize swarm on leader %s: %w", leader.SSH.RemoteAddr().String(), err)
	}

	// Join other nodes
	if err := joinNodes(ctx, vikingCli, leader, clients[1:]); err != nil {
		return err
	}

	return nil
}

func joinNodes(ctx context.Context, vikingCli *command.Cli, leader *dockerhelper.Client, clients []*dockerhelper.Client) error {
	if len(clients) == 0 {
		return nil
	}

	sw, err := leader.SwarmInspect(ctx)
	if err != nil {
		return fmt.Errorf("failed to inspect swarm: %w", err)
	}

	info, err := leader.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get leader info: %w", err)
	}

	workerJoinToken := sw.JoinTokens.Worker
	managerAddr := net.JoinHostPort(info.Swarm.NodeAddr, "2377")

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, client := range clients {
		wg.Add(1)
		go func(client *dockerhelper.Client) {
			defer wg.Done()
			if err := joinSwarmNode(ctx, client, managerAddr, workerJoinToken); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("could not join node %s to swarm: %v", client.SSH.RemoteAddr().String(), err))
				mu.Unlock()
			}
		}(client)
	}

	wg.Wait()

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(vikingCli.Err, err)
		}
		return errors.New("failed to join nodes to the swarm")
	}

	return nil
}

func joinSwarmNode(ctx context.Context, client *dockerhelper.Client, managerAddr, joinToken string) error {
	return retry(func() error {
		return client.SwarmJoin(ctx, swarm.JoinRequest{
			ListenAddr:    "0.0.0.0:2377",
			AdvertiseAddr: client.SSH.RemoteAddr().String(),
			JoinToken:     joinToken,
			RemoteAddrs:   []string{managerAddr},
		})
	}, 3, 5*time.Second)
}

func removeExtraNodes(ctx context.Context, vikingCli *command.Cli, manager *dockerhelper.Client, extraNodes []swarm.Node) error {
	fmt.Fprintln(vikingCli.Out, "Removing extra nodes from the swarm...")
	for _, node := range extraNodes {
		// If the node is a manager, demote it before removal
		if node.Spec.Role == swarm.NodeRoleManager {
			// Skip if the node is the leader
			if node.ManagerStatus != nil && node.ManagerStatus.Leader {
				fmt.Fprintf(vikingCli.Err, "Cannot remove leader node %s\n", node.Description.Hostname)
				continue
			}
			fmt.Fprintf(vikingCli.Out, "Demoting manager node %s before removal\n", node.Description.Hostname)
			if err := demoteNode(ctx, manager, node.ID); err != nil {
				fmt.Fprintf(vikingCli.Err, "Failed to demote node %s: %v\n", node.Description.Hostname, err)
				continue
			}
		}

		err := manager.NodeRemove(ctx, node.ID, types.NodeRemoveOptions{Force: true})
		if err != nil {
			fmt.Fprintf(vikingCli.Err, "Failed to remove node %s: %v\n", node.Description.Hostname, err)
		} else {
			fmt.Fprintf(vikingCli.Out, "Removed node %s from the swarm\n", node.Description.Hostname)
		}
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

func rebalanceManagers(ctx context.Context, vikingCli *command.Cli, manager *dockerhelper.Client, nodes []swarm.Node) error {
	totalNodes := len(nodes)
	desiredManagers := totalNodes/2 + 1
	if desiredManagers > 7 {
		desiredManagers = 7
	}

	var currentManagers, currentWorkers []swarm.Node
	for _, node := range nodes {
		switch node.Spec.Role {
		case swarm.NodeRoleManager:
			currentManagers = append(currentManagers, node)
		case swarm.NodeRoleWorker:
			currentWorkers = append(currentWorkers, node)
		}
	}

	// Promote workers to managers if needed
	if len(currentManagers) < desiredManagers {
		promoteCount := desiredManagers - len(currentManagers)
		for i := 0; i < promoteCount && i < len(currentWorkers); i++ {
			nodeID := currentWorkers[i].ID
			if err := promoteNode(ctx, manager, nodeID); err != nil {
				fmt.Fprintf(vikingCli.Err, "Failed to promote node %s: %v\n", nodeID, err)
			} else {
				fmt.Fprintf(vikingCli.Out, "Promoted node %s to manager\n", nodeID)
			}
		}
	}

	// Demote managers to workers if needed
	if len(currentManagers) > desiredManagers {
		demoteCount := len(currentManagers) - desiredManagers
		for i := 0; i < demoteCount && i < len(currentManagers); i++ {
			node := currentManagers[i]
			if node.ManagerStatus != nil && node.ManagerStatus.Leader {
				continue // Do not demote the leader
			}
			nodeID := node.ID
			if err := demoteNode(ctx, manager, nodeID); err != nil {
				fmt.Fprintf(vikingCli.Err, "Failed to demote node %s: %v\n", nodeID, err)
			} else {
				fmt.Fprintf(vikingCli.Out, "Demoted node %s to worker\n", nodeID)
			}
		}
	}

	return nil
}

func promoteNode(ctx context.Context, manager *dockerhelper.Client, nodeID string) error {
	node, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return err
	}
	node.Spec.Role = swarm.NodeRoleManager
	return manager.NodeUpdate(ctx, nodeID, node.Version, node.Spec)
}

func demoteNode(ctx context.Context, manager *dockerhelper.Client, nodeID string) error {
	node, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return err
	}
	node.Spec.Role = swarm.NodeRoleWorker
	return manager.NodeUpdate(ctx, nodeID, node.Version, node.Spec)
}

func retry(operation func() error, attempts int, delay time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = operation(); err == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return err
}
