package dockerhelper

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

func GetService(ctx context.Context, remote *client.Client, serviceName string) (*swarm.Service, error) {
	services, err := remote.ServiceList(ctx, types.ServiceListOptions{
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

func ListTasks(ctx context.Context, remote *client.Client, serviceID string) ([]swarm.Task, error) {
	tasks, err := remote.TaskList(ctx, types.TaskListOptions{
		Filters: filters.NewArgs(filters.Arg("service", serviceID)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	return tasks, nil
}
