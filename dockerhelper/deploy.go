package dockerhelper

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

func Deploy(ctx context.Context, sw *Swarm, name, image string) error {
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

	if err := DistributeImage(ctx, reader, sw.Clients, image); err != nil {
		return err
	}

	manager := sw.findManager(ctx, nil)
	if manager == nil {
		return fmt.Errorf("no manager node found")
	}

	services, err := manager.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	serviceSpec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: name,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image: image,
			},
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
		_, err := manager.ServiceUpdate(ctx, existingService.ID, existingService.Version, serviceSpec, types.ServiceUpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
	} else {
		slog.InfoContext(ctx, "Creating new service", "name", name)
		_, err := manager.ServiceCreate(ctx, serviceSpec, types.ServiceCreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	}

	return nil
}
