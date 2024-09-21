package dockerhelper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"time"

	"github.com/d3witt/viking/parallel"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"golang.org/x/crypto/ssh"
)

var ErrNoManagerFound = errors.New("no manager node found or available")

type Swarm struct {
	Clients  []*Client
	Timeout  time.Duration
	Interval time.Duration
}

func NewSwarm(clients []*Client) *Swarm {
	return &Swarm{
		Clients:  clients,
		Timeout:  2 * time.Minute,
		Interval: 1 * time.Second,
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

func (s *Swarm) RemoveNodesByAddr(ctx context.Context, addr string, force bool) error {
	manager := s.manager(ctx, []string{})
	if manager == nil {
		return ErrNoManagerFound
	}

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

			WaitFor(ctx, s.Timeout, s.Interval, func(ctx context.Context) (bool, error) {
				nodes, err := manager.NodeList(ctx, types.NodeListOptions{})
				if err != nil {
					return false, err
				}

				for _, n := range nodes {
					if n.ID == node.ID {
						return false, nil
					}
				}

				return true, nil
			})
		}
	}

	return nil
}

// LeaveNode removes a node from the swarm gracefully.
// If the node is a manager, it ensures that the swarm maintains the desired number of managers.
func (s *Swarm) LeaveNode(ctx context.Context, node *Client, force bool) error {
	slog.InfoContext(ctx, "Leaving swarm", "node", node.SSH.RemoteAddr())

	info, err := node.Info(ctx)
	if err != nil {
		return fmt.Errorf("get node info: %w", err)
	}

	manager := s.manager(ctx, []string{info.Swarm.NodeID})
	if manager == nil {
		return ErrNoManagerFound
	}
	managerInfo, err := manager.Info(ctx)
	if err != nil {
		return fmt.Errorf("get manager info: %w", err)
	}

	if info.Swarm.LocalNodeState == swarm.LocalNodeStateInactive {
		return nil
	}

	if info.Swarm.ControlAvailable {
		nodes, err := activeNodes(ctx, manager)
		if err != nil {
			return fmt.Errorf("failed to list active nodes: %w", err)
		}

		desiredManagers := desiredManagersCount(len(nodes) - 1)
		currentManagers := managerInfo.Swarm.Managers

		if currentManagers == 1 && len(nodes)-1 < 3 {
			return fmt.Errorf("cannot remove the last manager node in a single-node swarm, please add more nodes before removing this one")
		}

		// Adjust currentManagers to account for the node being removed
		currentManagers--

		for _, n := range nodes {
			if n.ID == managerInfo.Swarm.NodeID || n.ID == info.Swarm.NodeID {
				continue
			}

			if currentManagers < desiredManagers && n.Spec.Role == swarm.NodeRoleWorker {
				if err := s.promoteNode(ctx, manager, n.ID); err != nil {
					slog.ErrorContext(ctx, "Failed to promote node to manager. Continue with next node", "node", n.ID, "error", err)
				} else {
					currentManagers++
				}
			} else if currentManagers > desiredManagers && n.Spec.Role == swarm.NodeRoleManager {
				if err := s.demoteNode(ctx, manager, n.ID); err != nil {
					slog.ErrorContext(ctx, "Failed to demote node to worker. Continue with next node", "node", n.ID, "error", err)
				} else {
					currentManagers--
				}
			}

			if currentManagers == desiredManagers {
				break
			}
		}

		if info.Swarm.ControlAvailable {
			if err := s.demoteNode(ctx, manager, info.Swarm.NodeID); err != nil {
				return fmt.Errorf("demote node %s: %w", node.SSH.RemoteAddr(), err)
			}
		}
	}

	slog.InfoContext(ctx, "Leaving swarm", "node", node.SSH.RemoteAddr())
	if err := node.SwarmLeave(ctx, force); err != nil {
		return err
	}

	return WaitFor(ctx, s.Timeout, s.Interval, func(ctx context.Context) (bool, error) {
		nodes, err := manager.NodeList(ctx, types.NodeListOptions{})
		if err != nil {
			return false, err
		}

		for _, node := range nodes {
			if node.ID == info.Swarm.NodeID {
				return node.Status.State == swarm.NodeStateDown, nil
			}
		}

		return true, nil
	})
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
	manager := s.manager(ctx, nil)
	if manager == nil {
		return ErrNoManagerFound
	}

	return s.joinNodesWithManager(ctx, manager, clients)
}

// ManagerNode returns a manager node or nil if none are found.
func (s *Swarm) manager(ctx context.Context, excludeNodeIds []string) *Client {
	slog.Info("Finding manager node")
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

	WaitFor(ctx, s.Timeout, s.Interval, func(context.Context) (bool, error) {
		updatedNode, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
		if err != nil {
			return false, err
		}

		return updatedNode.Spec.Role == swarm.NodeRoleManager && updatedNode.Status.State == swarm.NodeStateReady, nil
	})

	return nil
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

	WaitFor(ctx, s.Timeout, s.Interval, func(context.Context) (bool, error) {
		updatedNode, _, err := manager.NodeInspectWithRaw(ctx, nodeID)
		if err != nil {
			return false, err
		}

		return updatedNode.Spec.Role == swarm.NodeRoleWorker && updatedNode.Status.State == swarm.NodeStateReady, nil
	})

	return nil
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
