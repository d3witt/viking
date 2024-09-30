package dockerhelper

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

func IsSwarmInactive(ctx context.Context, remote *client.Client) (bool, error) {
	info, err := remote.Info(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get info: %w", err)
	}

	return info.Swarm.LocalNodeState == swarm.LocalNodeStateInactive, nil
}

func InitSwarm(ctx context.Context, remote *client.Client, host string) error {
	slog.InfoContext(ctx, "Initializing swarm")
	_, err := remote.SwarmInit(ctx, swarm.InitRequest{
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
		return fmt.Errorf("failed to initialize swarm: %w", err)
	}

	return nil
}

func LeaveSwarm(ctx context.Context, remote *client.Client) error {
	slog.InfoContext(ctx, "Leaving swarm")

	info, err := remote.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get info: %w", err)
	}

	if info.Swarm.LocalNodeState == swarm.LocalNodeStateInactive {
		return nil
	}

	if err := remote.SwarmLeave(ctx, true); err != nil {
		return fmt.Errorf("failed to leave swarm: %w", err)
	}

	return nil
}

func NetworkExists(ctx context.Context, remote *client.Client, name string) (bool, error) {
	networks, err := remote.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list networks: %w", err)
	}

	for _, network := range networks {
		if network.Name == name {
			return true, nil
		}
	}

	return false, nil
}

func CreateNetworkIfNotExists(ctx context.Context, remote *client.Client, name string) error {
	networks, err := remote.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, network := range networks {
		if network.Name == name {
			return nil
		}
	}

	slog.InfoContext(ctx, "Creating network", "name", name)

	_, err = remote.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:     "overlay",
		Attachable: true,
	})
	return err
}
