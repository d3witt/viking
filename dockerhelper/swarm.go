package dockerhelper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/d3witt/viking/parallel"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
)

var ErrNoManagerFound = errors.New("no manager node found or available")

// ManagerNode retrieves the first available manager node from the provided clients.
// It concurrently checks each client to determine if it's a manager.
// Returns the manager client or an error if none are found.
func ManagerNode(ctx context.Context, clients []*Client) (*Client, error) {
	if len(clients) == 0 {
		return nil, ErrNoManagerFound
	}

	slog.InfoContext(ctx, "Searching for manager node...")

	type result struct {
		client *Client
		err    error
	}

	resultCh := make(chan result, len(clients))
	var wg sync.WaitGroup

	for i := 0; i < len(clients); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			info, err := clients[i].Info(ctx)
			if err != nil {
				resultCh <- result{err: err}
				return
			}

			if info.Swarm.LocalNodeState == swarm.LocalNodeStateActive && info.Swarm.ControlAvailable {
				resultCh <- result{client: clients[i]}
			} else {
				resultCh <- result{err: ErrNoManagerFound}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for res := range resultCh {
		if res.err == nil {
			return res.client, nil
		}
	}

	return nil, ErrNoManagerFound
}

type SwarmStatusInfo struct {
	Missing  []*Client
	Workers  []*Client
	Managers []*Client
}

// SwarmStatus retrieves the status of the swarm across the provided clients.
// It categorizes nodes into Missing, Workers, and Managers.
// Returns the swarm status or an error if multiple clusters are detected or other issues arise.
func SwarmStatus(ctx context.Context, clients []*Client) (*SwarmStatusInfo, error) {
	slog.InfoContext(ctx, "Retrieving swarm status")

	var swarmStatus SwarmStatusInfo
	clusterIDs := make(map[string]struct{})

	for _, cl := range clients {
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

// PromoteNode promotes a worker node to a manager within the swarm.
func PromoteNode(ctx context.Context, manager *Client, nodeID string) error {
	slog.InfoContext(ctx, "Promoting node to manager", "id", nodeID)

	node, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to inspect node %s: %w", nodeID, err)
	}

	node.Spec.Role = swarm.NodeRoleManager

	if err := manager.NodeUpdate(ctx, nodeID, node.Version, node.Spec); err != nil {
		return fmt.Errorf("failed to update node %s: %w", nodeID, err)
	}

	return nil
}

// DemoteNode demotes a manager node to a worker within the swarm.
func DemoteNode(ctx context.Context, manager *Client, nodeID string) error {
	slog.InfoContext(ctx, "Demoting node to worker", "id", nodeID)

	node, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to inspect node %s: %w", nodeID, err)
	}

	node.Spec.Role = swarm.NodeRoleWorker

	if err := manager.NodeUpdate(ctx, nodeID, node.Version, node.Spec); err != nil {
		return fmt.Errorf("failed to update node %s: %w", nodeID, err)
	}

	return nil
}

// DesiredManagersCount calculates the desired number of manager nodes based on the total number of nodes.
// Ensures fault tolerance by maintaining a quorum of managers.
func DesiredManagersCount(total int) int {
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

// JoinNodes adds multiple nodes to the swarm using the provided manager.
// It handles promotion of workers to managers to maintain the desired number of managers.
func JoinNodes(ctx context.Context, manager *Client, clients []*Client) error {
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

	parallel.ForEach(ctx, len(clients), func(i int) {
		if err := joinSwarmNode(ctx, clients[i], managerAddr, sw.JoinTokens.Worker); err != nil {
			slog.ErrorContext(ctx, "Could not join node to swarm", "node", clients[i].SSH.RemoteAddr(), "error", err)
		}
	})

	info, err = manager.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get manager info: %w", err)
	}

	nodes, err := activeNodes(ctx, manager)
	if err != nil {
		return fmt.Errorf("failed to list active nodes: %w", err)
	}

	desiredManagers := DesiredManagersCount(len(nodes))
	if desiredManagers > info.Swarm.Managers {
		slog.InfoContext(ctx, "Adding more managers to the swarm")
		managersToAdd := desiredManagers - info.Swarm.Managers

		for _, node := range nodes {
			if managersToAdd == 0 {
				break
			}

			if node.Spec.Role == swarm.NodeRoleWorker {
				if err := PromoteNode(ctx, manager, node.ID); err == nil {
					managersToAdd--
				} else {
					slog.ErrorContext(ctx, "Failed to promote node to manager. Continue with next node", "node", node.ID, "error", err)
				}
			}
		}
	}

	return nil
}

// joinSwarmNode handles the actual swarm join operation for a single node.
func joinSwarmNode(ctx context.Context, client *Client, managerAddr, joinToken string) error {
	host, _, err := net.SplitHostPort(managerAddr)
	if err != nil {
		return fmt.Errorf("invalid manager address %s: %w", managerAddr, err)
	}

	joinRequest := swarm.JoinRequest{
		ListenAddr:    "0.0.0.0:2377",
		AdvertiseAddr: host,
		JoinToken:     joinToken,
		RemoteAddrs:   []string{managerAddr},
	}

	slog.InfoContext(ctx, "Joining swarm", "machine", client.SSH.RemoteAddr().String(), "manager", managerAddr)
	return client.SwarmJoin(ctx, joinRequest)
}

// LeaveNode removes a node from the swarm gracefully.
// If the node is a manager, it ensures that the swarm maintains the desired number of managers.
func LeaveNode(ctx context.Context, manager *Client, node *Client, force bool) error {
	slog.InfoContext(ctx, "Leaving swarm", "node", node.SSH.RemoteAddr())

	managerInfo, err := manager.Info(ctx)
	if err != nil {
		return fmt.Errorf("get manager info: %w", err)
	}

	info, err := node.Info(ctx)
	if err != nil {
		return fmt.Errorf("get node info: %w", err)
	}

	if info.Swarm.LocalNodeState == swarm.LocalNodeStateInactive {
		return nil
	}

	if info.Swarm.ControlAvailable {
		nodes, err := activeNodes(ctx, manager)
		if err != nil {
			return fmt.Errorf("failed to list active nodes: %w", err)
		}

		desiredManagers := DesiredManagersCount(len(nodes) - 1)
		currentManagers := managerInfo.Swarm.Managers

		if managerInfo.Swarm.Managers == 1 && managerInfo.Swarm.Nodes < 3 {
			return fmt.Errorf("cannot remove the last manager node in a single-node swarm, please add more nodes before removing this one")
		}

		for _, node := range nodes {
			if currentManagers-1 < desiredManagers && node.Spec.Role == swarm.NodeRoleWorker {
				if err := PromoteNode(ctx, manager, node.ID); err != nil {
					slog.ErrorContext(ctx, "Failed to promote node to manager. Continue with next node", "node", node.ID, "error", err)
				} else {
					currentManagers++
				}
			} else if currentManagers-1 > desiredManagers && node.Spec.Role == swarm.NodeRoleManager && node.ID != info.Swarm.NodeID && node.ID != managerInfo.Swarm.NodeID {
				if err := DemoteNode(ctx, manager, node.ID); err != nil {
					slog.ErrorContext(ctx, "Failed to demote node to worker. Continue with next node", "node", node.ID, "error", err)
				} else {
					currentManagers--
				}
			}

			if currentManagers-1 == desiredManagers {
				break
			}
		}

		if err := DemoteNode(ctx, manager, info.Swarm.NodeID); err != nil {
			return fmt.Errorf("demote node %s: %w", node.SSH.RemoteAddr(), err)
		}
	}

	if err := node.SwarmLeave(ctx, force); err != nil {
		return fmt.Errorf("failed to leave swarm: %w", err)
	}

	return manager.NodeRemove(ctx, info.Swarm.NodeID, types.NodeRemoveOptions{
		Force: true,
	})
}

// LeaveSwarm removes multiple nodes from the swarm concurrently.
func LeaveSwarm(ctx context.Context, clients []*Client) {
	slog.InfoContext(ctx, "Leaving swarm")

	parallel.ForEach(ctx, len(clients), func(i int) {
		client := clients[i]
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
