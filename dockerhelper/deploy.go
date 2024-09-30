package dockerhelper

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

func Deploy(
	ctx context.Context,
	remote *client.Client,
	name, image string,
	replicas uint64,
	ports, networks []string,
	env, label map[string]string,
) error {
	local, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer local.Close()

	reader, err := local.ImageSave(ctx, []string{image})
	if err != nil {
		return fmt.Errorf("failed to save image on source: %w", err)
	}
	defer reader.Close()

	if err := DistributeImage(ctx, local, remote, image); err != nil {
		return err
	}

	services, err := remote.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	serviceSpec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   name,
			Labels: label,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image: image,
				Env:   mapToSlice(env),
			},
			Networks: parseNetworks(networks),
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
		EndpointSpec: &swarm.EndpointSpec{
			Ports: parsePorts(ports),
		},
	}

	var existingService *swarm.Service
	for _, service := range services {
		if service.Spec.Name == name {
			existingService = &service
			break
		}
	}

	if existingService != nil {
		slog.InfoContext(ctx, "Updating existing service", "name", name)
		_, err := remote.ServiceUpdate(ctx, existingService.ID, existingService.Version, serviceSpec, types.ServiceUpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
	} else {
		slog.InfoContext(ctx, "Creating new service", "name", name)
		_, err := remote.ServiceCreate(ctx, serviceSpec, types.ServiceCreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	}

	return nil
}

func mapToSlice(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func parsePorts(ports []string) []swarm.PortConfig {
	result := make([]swarm.PortConfig, 0, len(ports))
	for _, p := range ports {
		var port swarm.PortConfig
		fmt.Sscanf(p, "%d:%d", &port.PublishedPort, &port.TargetPort)
		port.Protocol = swarm.PortConfigProtocolTCP // Assuming TCP by default
		result = append(result, port)
	}
	return result
}

func parseNetworks(networks []string) []swarm.NetworkAttachmentConfig {
	result := make([]swarm.NetworkAttachmentConfig, 0, len(networks))
	for _, n := range networks {
		result = append(result, swarm.NetworkAttachmentConfig{Target: n})
	}
	return result
}
