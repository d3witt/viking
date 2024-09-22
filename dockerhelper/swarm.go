package dockerhelper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"

	"github.com/d3witt/viking/parallel"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"golang.org/x/crypto/ssh"
)

var ErrNoManagerFound = errors.New("no manager node found or available")

type Swarm struct {
	Clients []*Client
}

func NewSwarm(clients []*Client) *Swarm {
	return &Swarm{
		Clients: clients,
	}
}

func DialSwarmSSH(ctx context.Context, clients []*ssh.Client) (*Swarm, error) {
	dockerClients := make([]*Client, len(clients))

	err := parallel.RunFirstErr(ctx, len(clients), func(i int) error {
		dockerClient, err := DialSSH(clients[i])
		if err != nil {
			return fmt.Errorf("%s: could not dial Docker: %w", clients[i].RemoteAddr().String(), err)
		}
		dockerClients[i] = dockerClient
		return nil
	})
	if err != nil {
		return nil, err
	}

	return NewSwarm(dockerClients), nil
}

func (s *Swarm) Close() {
	for _, cl := range s.Clients {
		cl.Close()
	}
}

type SwarmStatus struct {
	Missing  []*Client
	Workers  []*Client
	Managers []*Client
}

func (s *Swarm) GetClientByAddr(ip string) *Client {
	for _, client := range s.Clients {
		host, _, _ := net.SplitHostPort(client.SSH.RemoteAddr().String())
		if host == ip {
			return client
		}
	}

	return nil
}

func (s *Swarm) Exists(ctx context.Context) bool {
	slog.InfoContext(ctx, "Checking if swarm exists")

	for _, cl := range s.Clients {
		info, err := cl.Info(ctx)
		if err != nil {
			slog.WarnContext(ctx, "Failed to get info", "addr", cl.SSH.RemoteAddr(), "err", err)
			continue
		}

		if info.Swarm.LocalNodeState != swarm.LocalNodeStateInactive {
			return true
		}
	}

	return false
}

// Returns the swarm status or an error if multiple clusters are detected or other issues arise.
func (s *Swarm) Status(ctx context.Context) (*SwarmStatus, error) {
	slog.InfoContext(ctx, "Retrieving swarm status")

	var swarmStatus SwarmStatus
	clusterIDs := make(map[string]struct{})

	for _, cl := range s.Clients {
		info, err := cl.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get info for client %s: %w", cl.SSH.RemoteAddr(), err)
		}

		if info.Swarm.Cluster != nil {
			clusterIDs[info.Swarm.Cluster.ID] = struct{}{}
		}

		switch info.Swarm.LocalNodeState {
		case swarm.LocalNodeStateActive:
			if info.Swarm.ControlAvailable {
				swarmStatus.Managers = append(swarmStatus.Managers, cl)
			} else {
				swarmStatus.Workers = append(swarmStatus.Workers, cl)
			}
		case swarm.LocalNodeStateInactive:
			swarmStatus.Missing = append(swarmStatus.Missing, cl)
		default:
			return nil, fmt.Errorf("unknown swarm state '%s' for client %s", info.Swarm.LocalNodeState, cl.SSH.RemoteAddr())
		}
	}

	if len(clusterIDs) > 1 {
		return nil, fmt.Errorf("multiple swarm clusters detected: %v", clusterIDs)
	}

	return &swarmStatus, nil
}

func (s *Swarm) Init(ctx context.Context) error {
	if len(s.Clients) == 0 {
		return errors.New("no clients available to initialize the swarm")
	}

	leader := s.Clients[0]
	host, _, err := net.SplitHostPort(leader.SSH.RemoteAddr().String())
	if err != nil {
		return fmt.Errorf("failed to extract hostname for leader: %w", err)
	}

	slog.InfoContext(ctx, "Initializing swarm", "leader", host)
	_, err = leader.SwarmInit(ctx, swarm.InitRequest{
		ListenAddr:    "0.0.0.0:2377",
		AdvertiseAddr: host,
		Spec: swarm.Spec{
			Annotations: swarm.Annotations{
				Labels: map[string]string{
					SwarmLabel: "true",
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize swarm on leader %s: %w", host, err)
	}

	return s.joinNodesWithManager(ctx, leader, s.Clients[1:])
}

func (s *Swarm) removeNodesByAddr(ctx context.Context, manager *Client, addr string, force bool) error {
	nodes, err := manager.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes {
		if node.Status.Addr == addr {
			slog.InfoContext(ctx, "Removing node", "node", node.ID)

			if err := manager.NodeRemove(ctx, node.ID, types.NodeRemoveOptions{Force: force}); err != nil {
				slog.ErrorContext(ctx, "Failed to remove node", "node", node.ID, "error", err)
			}
		}
	}

	return nil
}

// LeaveNode removes a node from the swarm.
// If the node is a worker, it simply leaves the swarm.
// If the node is a manager, it adjusts the swarm to maintain the desired number of managers.
// If no manager is available, it tries to make the node leave the swarm.
func (s *Swarm) LeaveNode(ctx context.Context, node *Client, force bool) error {
	slog.InfoContext(ctx, "Leaving swarm", "node", node.SSH.RemoteAddr())

	info, err := node.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get node info: %w", err)
	}

	if info.Swarm.LocalNodeState == swarm.LocalNodeStateInactive {
		return nil
	}

	manager := s.findManager(ctx, nil)

	if info.Swarm.ControlAvailable && manager != nil {
		if err := s.adjustManagerCount(ctx, manager, info); err != nil {
			return err
		}

		manager = s.findManager(ctx, []string{info.Swarm.NodeID})
		if manager == nil {
			return ErrNoManagerFound
		}

		if err := s.demoteNode(ctx, manager, info.Swarm.NodeID); err != nil {
			return fmt.Errorf("failed to demote node %s: %w", node.SSH.RemoteAddr(), err)
		}
	}

	if err := node.SwarmLeave(ctx, force); err != nil {
		return fmt.Errorf("failed to leave swarm: %w", err)
	}

	if manager != nil {
		slog.InfoContext(ctx, "Waiting for node to be down", "node", info.Swarm.NodeID)
		condition := func(node swarm.Node) bool {
			return node.Status.State == swarm.NodeStateDown
		}
		if err := s.waitForNodeCondition(ctx, manager, info.Swarm.NodeID, condition); err != nil {
			return fmt.Errorf("failed to wait for node to be down: %w", err)
		}

		if err := s.removeNodesByAddr(ctx, manager, info.Swarm.NodeAddr, force); err != nil {
			return err
		}
	}

	return nil
}

// LeaveSwarm removes multiple nodes from the swarm concurrently.
func (s *Swarm) LeaveSwarm(ctx context.Context) {
	slog.InfoContext(ctx, "Leaving swarm")

	parallel.ForEach(ctx, len(s.Clients), func(i int) {
		client := s.Clients[i]
		info, err := client.Info(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to get node info", "client", client.SSH.RemoteAddr(), "error", err)
			return
		}

		if info.Swarm.LocalNodeState == swarm.LocalNodeStateInactive {
			return
		}

		if err := client.SwarmLeave(ctx, true); err != nil {
			slog.ErrorContext(ctx, "Failed to leave swarm", "client", client.SSH.RemoteAddr(), "error", err)
		}
	})
}

func (s *Swarm) JoinNodes(ctx context.Context, clients []*Client) error {
	manager := s.findManager(ctx, nil)
	if manager == nil {
		return ErrNoManagerFound
	}

	return s.joinNodesWithManager(ctx, manager, clients)
}

// ManagerNode returns a manager node or nil if none are found.
func (s *Swarm) findManager(ctx context.Context, excludeNodeIds []string) *Client {
	for _, cl := range s.Clients {
		info, err := cl.Info(ctx)
		if err != nil {
			slog.Error("Error getting node info", "err", err)
			continue
		}
		if info.Swarm.ControlAvailable && !slices.Contains(excludeNodeIds, info.Swarm.NodeID) {
			return cl
		}
	}
	return nil
}

func (s *Swarm) adjustManagerCount(ctx context.Context, manager *Client, nodeToRemove system.Info) error {
	nodes, err := activeNodes(ctx, manager)
	if err != nil {
		return fmt.Errorf("failed to list active nodes: %w", err)
	}

	desiredManagers := desiredManagersCount(len(nodes) - 1)
	managerInfo, err := manager.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get manager info: %w", err)
	}
	currentManagers := managerInfo.Swarm.Managers - 1 // Subtract the node being removed

	if managerInfo.Swarm.Managers == 1 && managerInfo.Swarm.Nodes < 3 {
		return errors.New("not enough nodes to remove manager, please add more nodes to the swarm or run `viking machine rm` to remove the swarm")
	}

	for _, n := range nodes {
		if n.ID == managerInfo.Swarm.NodeID || n.ID == nodeToRemove.Swarm.NodeID {
			continue
		}

		switch {
		case currentManagers < desiredManagers && n.Spec.Role == swarm.NodeRoleWorker:
			if err := s.promoteNode(ctx, manager, n.ID); err != nil {
				slog.ErrorContext(ctx, "Failed to promote node to manager", "node", n.ID, "error", err)
			} else {
				currentManagers++
			}
		case currentManagers > desiredManagers && n.Spec.Role == swarm.NodeRoleManager:
			if err := s.demoteNode(ctx, manager, n.ID); err != nil {
				slog.ErrorContext(ctx, "Failed to demote node to worker", "node", n.ID, "error", err)
			} else {
				currentManagers--
			}
		}

		if currentManagers == desiredManagers {
			break
		}
	}

	return nil
}

// JoinNodes handles promotion of workers to managers to maintain the desired number of managers.
func (s *Swarm) joinNodesWithManager(ctx context.Context, manager *Client, clients []*Client) error {
	if len(clients) == 0 {
		return nil
	}

	sw, err := manager.SwarmInspect(ctx)
	if err != nil {
		return fmt.Errorf("failed to inspect swarm: %w", err)
	}

	info, err := manager.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get manager info: %w", err)
	}

	managerAddr := net.JoinHostPort(info.Swarm.NodeAddr, "2377")

	// If run in parallel, the manager node will be promoted to manager before the worker nodes are joined.
	// This will cause the worker nodes to fail to join the swarm.
	for _, client := range clients {
		if err := joinSwarmNode(ctx, client, managerAddr, sw.JoinTokens.Worker); err != nil {
			slog.ErrorContext(ctx, "Could not join node to swarm", "node", client.SSH.RemoteAddr(), "error", err)
		}
	}

	info, err = manager.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get manager info: %w", err)
	}

	nodes, err := activeNodes(ctx, manager)
	if err != nil {
		return fmt.Errorf("failed to list active nodes: %w", err)
	}

	desiredManagers := desiredManagersCount(len(nodes))
	if desiredManagers > info.Swarm.Managers {
		slog.InfoContext(ctx, "Adding more managers to the swarm")
		managersToAdd := desiredManagers - info.Swarm.Managers

		for _, node := range nodes {
			if managersToAdd == 0 {
				break
			}

			if node.Spec.Role == swarm.NodeRoleWorker {
				if err := s.promoteNode(ctx, manager, node.ID); err == nil {
					managersToAdd--
				} else {
					slog.ErrorContext(ctx, "Failed to promote node to manager. Continue with next node", "node", node.ID, "error", err)
				}
			}
		}
	}

	return nil
}

func (s *Swarm) promoteNode(ctx context.Context, manager *Client, nodeID string) error {
	slog.InfoContext(ctx, "Promoting node to manager", "id", nodeID)

	node, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to inspect node %s: %w", nodeID, err)
	}

	node.Spec.Role = swarm.NodeRoleManager

	if err := manager.NodeUpdate(ctx, nodeID, node.Version, node.Spec); err != nil {
		return fmt.Errorf("failed to update node %s: %w", nodeID, err)
	}

	slog.InfoContext(ctx, "Waiting for node to become a manager", "node", nodeID)
	condition := func(node swarm.Node) bool {
		return node.Spec.Role == swarm.NodeRoleManager && node.Status.State == swarm.NodeStateReady &&
			node.ManagerStatus != nil && node.ManagerStatus.Reachability == swarm.ReachabilityReachable
	}
	return s.waitForNodeCondition(ctx, manager, nodeID, condition)
}

func (s *Swarm) demoteNode(ctx context.Context, manager *Client, nodeID string) error {
	slog.InfoContext(ctx, "Demoting node to worker", "id", nodeID)

	node, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to inspect node %s: %w", nodeID, err)
	}

	node.Spec.Role = swarm.NodeRoleWorker

	if err := manager.NodeUpdate(ctx, nodeID, node.Version, node.Spec); err != nil {
		return fmt.Errorf("failed to update node %s: %w", nodeID, err)
	}

	slog.InfoContext(ctx, "Waiting for node to be demoted", "id", nodeID)
	condition := func(node swarm.Node) bool {
		return node.Spec.Role == swarm.NodeRoleWorker && node.Status.State == swarm.NodeStateReady && node.ManagerStatus == nil
	}
	return s.waitForNodeCondition(ctx, manager, nodeID, condition)
}

func activeNodes(ctx context.Context, manager *Client) ([]swarm.Node, error) {
	nodes, err := manager.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	var active []swarm.Node
	for _, node := range nodes {
		if node.Status.State == swarm.NodeStateReady {
			active = append(active, node)
		}
	}

	return active, nil
}

// desiredManagersCount calculates the desired number of manager nodes based on the total number of nodes.
// Ensures fault tolerance by maintaining a quorum of managers.
func desiredManagersCount(total int) int {
	switch {
	case total == 0:
		return 0
	case total < 3:
		return 1
	case total == 3:
		return 3
	default:
		half := total/2 + 1
		if half > 7 {
			return 7
		}
		return half
	}
}

// joinSwarmNode handles the actual swarm join operation for a single node.
func joinSwarmNode(ctx context.Context, client *Client, managerAddr, joinToken string) error {
	host, _, err := net.SplitHostPort(client.SSH.RemoteAddr().String())
	if err != nil {
		return fmt.Errorf("invalid manager address %s: %w", managerAddr, err)
	}

	joinRequest := swarm.JoinRequest{
		ListenAddr:    "0.0.0.0:2377",
		AdvertiseAddr: host,
		JoinToken:     joinToken,
		RemoteAddrs:   []string{managerAddr},
	}

	slog.InfoContext(ctx, "Joining swarm", "machine", host, "manager", managerAddr)
	return client.SwarmJoin(ctx, joinRequest)
}

// waitForNodeCondition listens for node events and waits until the provided condition is met.
func (s *Swarm) waitForNodeCondition(ctx context.Context, manager *Client, nodeID string, condition func(node swarm.Node) bool) error {
	// Check if the condition is already met before listening for events
	node, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to inspect node %s: %w", nodeID, err)
	}
	if condition(node) {
		return nil
	}

	// Set up event filtering for the specific node
	eventFilters := filters.NewArgs()
	eventFilters.Add("type", "node")
	eventFilters.Add("id", nodeID)
	options := events.ListOptions{
		Filters: eventFilters,
	}

	// Start listening for events
	eventsCh, errorsCh := manager.Events(ctx, options)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for node condition")
		case err := <-errorsCh:
			return fmt.Errorf("error while receiving events: %w", err)
		case event := <-eventsCh:
			if event.Action == "update" {
				// Re-inspect the node to check if the condition is now met
				node, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
				if err != nil {
					return fmt.Errorf("failed to inspect node %s: %w", nodeID, err)
				}
				if condition(node) {
					return nil
				}
			}
		}
	}
}
