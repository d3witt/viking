package dockerhelper

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
)

func ServiceLogs(ctx context.Context, swarm *Swarm, service string, options container.LogsOptions) (io.ReadCloser, error) {
	manager := swarm.findManager(ctx, nil)
	if manager == nil {
		return nil, fmt.Errorf("no manager node found")
	}

	return manager.ServiceLogs(ctx, service, options)
}

func RemoveService(ctx context.Context, swarm *Swarm, service string) error {
	manager := swarm.findManager(ctx, nil)
	if manager == nil {
		return fmt.Errorf("no manager node found")
	}

	return manager.ServiceRemove(ctx, service)
}

func GetService(ctx context.Context, swarm *Swarm, serviceName string) (*swarm.Service, error) {
	manager := swarm.findManager(ctx, nil)
	if manager == nil {
		return nil, fmt.Errorf("no manager node found")
	}

	services, err := manager.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("name", serviceName)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	if len(services) == 0 {
		return nil, nil
	}

	return &services[0], nil
}
