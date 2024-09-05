package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewRunCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "deploy",
		Usage: "Deploy a service to a machine",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Name of the service",
			},
			&cli.Uint64Flag{
				Name:    "replicas",
				Aliases: []string{"r"},
				Usage:   "Number of replicas",
				Value:   1,
			},
			&cli.StringFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "Publish a container's port to the host. Format: hostPort:containerPort",
			},
			&cli.StringSliceFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Usage:   "Environment variables",
			},
			&cli.StringSliceFlag{
				Name:    "bind",
				Aliases: []string{"b"},
				Usage:   "Bind folder on host to container",
			},
			&cli.StringSliceFlag{
				Name:    "network",
				Aliases: []string{"net"},
				Usage:   "Connect the service to a network",
				Value:   cli.NewStringSlice(vikingNetworkName),
			},
			&cli.StringFlag{
				Name:  "health-cmd",
				Usage: "Command to run to check health",
			},
			&cli.StringFlag{
				Name:  "health-interval",
				Usage: "Time between running the check",
				Value: "5s",
			},
			&cli.StringFlag{
				Name:  "health-timeout",
				Usage: "Maximum time to allow one check to run",
				Value: "3s",
			},
			&cli.IntFlag{
				Name:  "health-retries",
				Usage: "Consecutive failures needed to report unhealthy",
				Value: 3,
			},
			&cli.StringFlag{
				Name:  "rollback-delay",
				Usage: "Delay between task rollbacks",
				Value: "0s",
			},
			&cli.StringFlag{
				Name:  "stop-grace-period",
				Usage: "Time to wait before force killing a container",
				Value: "10s",
			},
			&cli.StringFlag{
				Name:  "stop-signal",
				Usage: "Signal to stop the container",
				Value: "SIGTERM",
			},
			&cli.StringSliceFlag{
				Name:    "label",
				Aliases: []string{"l"},
				Usage:   "Service labels",
			},
		},
		Args: true,
		Action: func(c *cli.Context) error {
			machine := c.Args().First()

			healthInterval, err := parseTime(c.String("health-interval"))
			if err != nil {
				return fmt.Errorf("invalid health-interval: %w", err)
			}
			healthTimeout, err := parseTime(c.String("health-timeout"))
			if err != nil {
				return fmt.Errorf("invalid health-timeout: %w", err)
			}
			rollbackDelay, err := parseTime(c.String("rollback-delay"))
			if err != nil {
				return fmt.Errorf("invalid rollback-delay: %w", err)
			}
			stopGracePeriod, err := parseTime(c.String("stop-grace-period"))
			if err != nil {
				return fmt.Errorf("invalid stop-grace-period: %w", err)
			}

			labels, err := parseLabels(c.StringSlice("label"))
			if err != nil {
				return fmt.Errorf("invalid label: %w", err)
			}

			image := c.Args().Get(1)
			cmd := c.Args().Slice()[2:]

			options := serviceOptions{
				Image:    image,
				Cmd:      cmd,
				Labels:   labels,
				Name:     c.String("name"),
				Replicas: c.Uint64("replicas"),
				Port:     c.String("port"),
				Env:      c.StringSlice("env"),
				Bind:     c.StringSlice("bind"),
				Network:  c.StringSlice("network"),
				Health: healthOptions{
					Cmd:      c.String("health-cmd"),
					Interval: healthInterval,
					Timeout:  healthTimeout,
					Retries:  c.Int("health-retries"),
				},
				Rollback: rollbackOptions{
					Delay: rollbackDelay,
				},
				StopGracePeriod: stopGracePeriod,
				StopSignal:      c.String("stop-signal"),
			}

			return runDeploy(c.Context, vikingCli, machine, options)
		},
	}
}

func parseLabels(val []string) (map[string]string, error) {
	labels := make(map[string]string)
	for _, label := range val {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			return labels, errors.New("invalid label format")
		}

		labels[parts[0]] = parts[1]
	}

	return labels, nil
}

func parseTime(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	return time.ParseDuration(s)
}

func runDeploy(ctx context.Context, vikingCli *command.Cli, machine string, options serviceOptions) error {
	clients, err := vikingCli.DialMachine(machine)
	if err != nil {
		return err
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	if err := ensureDockerInstalled(vikingCli, clients); err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "Deploying service to %s...\n", machine)
	manager, err := ensureSwarm(ctx, vikingCli, clients)
	if err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "Creating service %s...\n", options.Name)
	return runService(ctx, vikingCli, manager, options)
}

const (
	swarmLabel        = "viking-swarm"
	vikingNetworkName = "viking-network"
)

func ensureDockerInstalled(vikingCli *command.Cli, clients []*ssh.Client) error {
	for _, client := range clients {
		if !dockerhelper.IsDockerInstalled(client) {
			fmt.Fprintf(vikingCli.Out, "Installing Docker on %s...\n", client.RemoteAddr().String())
			if err := dockerhelper.InstallDocker(client); err != nil {
				return fmt.Errorf("could not install Docker on host %s: %w", client.RemoteAddr().String(), err)
			}
		}
	}

	return nil
}

func ensureSwarm(ctx context.Context, vikingCli *command.Cli, clients []*ssh.Client) (*dockerhelper.Client, error) {
	dockerClients := make([]*dockerhelper.Client, len(clients))
	for i, sshClient := range clients {
		dockerClient, err := dockerhelper.DialSSH(sshClient)
		if err != nil {
			return nil, fmt.Errorf("could not create Docker client: %w", err)
		}

		dockerClients[i] = dockerClient
	}
	defer func() {
		for _, client := range dockerClients {
			client.Close()
		}
	}()

	status, nodes, err := getSwarmStatus(ctx, dockerClients)
	if err != nil {
		return nil, err
	}

	if status == nil {
		status, nodes, err = initSwarm(ctx, dockerClients)
		if err != nil {
			return nil, err
		}
	}

	if err := joinMissingHosts(ctx, vikingCli, dockerClients, status, nodes); err != nil {
		return nil, err
	}

	for _, dockerClient := range dockerClients {
		if err := ensureVikingNetwork(ctx, dockerClient); err != nil {
			return nil, fmt.Errorf("could not ensure Viking network on host %s: %v", dockerClient.SSH.RemoteAddr().String(), err)
		}
	}

	manager := findManager(nodes)
	if manager == nil {
		return nil, errors.New("could not find manager node")
	}

	managerClient := findClientByNodeAddr(dockerClients, manager.Status.Addr)
	if managerClient == nil {
		return nil, errors.New("could not find manager client")
	}

	return managerClient, nil
}

func getSwarmStatus(ctx context.Context, clients []*dockerhelper.Client) (*swarm.Swarm, []swarm.Node, error) {
	var status *swarm.Swarm
	var swarmAddr string
	nodes := make([]swarm.Node, 0)

	for _, dockerClient := range clients {
		s, err := dockerClient.SwarmInspect(ctx)
		if err != nil {
			if client.IsErrConnectionFailed(err) {
				return nil, nil, err
			}
			continue
		}

		if status != nil && s.ID != status.ID {
			return nil, nil, fmt.Errorf("%s and %s are part of different swarms", swarmAddr, dockerClient.SSH.RemoteAddr().String())
		}

		if status != nil {
			continue
		}

		status = &s
		swarmAddr = dockerClient.SSH.RemoteAddr().String()

		nodes, err = dockerClient.NodeList(ctx, types.NodeListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("could not list nodes: %v", err)
		}
	}

	return status, nodes, nil
}

func initSwarm(ctx context.Context, clients []*dockerhelper.Client) (*swarm.Swarm, []swarm.Node, error) {
	if len(clients) == 0 {
		return nil, nil, fmt.Errorf("no hosts provided")
	}

	managersCount := len(clients)/2 + 1

	if _, err := clients[0].SwarmInit(ctx, swarm.InitRequest{
		ListenAddr: "0.0.0.0:2377",
		Spec: swarm.Spec{
			Annotations: swarm.Annotations{
				Labels: map[string]string{
					swarmLabel: "true",
				},
			},
		},
	}); err != nil {
		return nil, nil, err
	}

	status, err := clients[0].SwarmInspect(ctx)
	if err != nil {
		if client.IsErrConnectionFailed(err) {
			return nil, nil, err
		}

		return nil, nil, fmt.Errorf("failed to initialize swarm: %v", err)
	}
	leaderHost, _, err := net.SplitHostPort(clients[0].SSH.RemoteAddr().String())
	if err != nil {
		return nil, nil, fmt.Errorf("could not parse remote address: %v", err)
	}
	leaderAddr := net.JoinHostPort(leaderHost, "2377")

	other := clients[1:]
	managers := other[:managersCount-1]
	workers := other[managersCount-1:]

	for _, manager := range managers {
		if err := manager.SwarmJoin(ctx, swarm.JoinRequest{
			ListenAddr: "0.0.0.0:2377",
			JoinToken:  status.JoinTokens.Manager,
			RemoteAddrs: []string{
				leaderAddr,
			},
		}); err != nil {
			return nil, nil, fmt.Errorf("could not join manager %s to swarm: %v", manager.SSH.RemoteAddr().String(), err)
		}
	}

	for _, worker := range workers {
		if err := worker.SwarmJoin(ctx, swarm.JoinRequest{
			JoinToken: status.JoinTokens.Worker,
			RemoteAddrs: []string{
				leaderAddr,
			},
		}); err != nil {
			return nil, nil, fmt.Errorf("could not join worker %s to swarm: %v", worker.SSH.RemoteAddr().String(), err)
		}
	}

	nodes, err := clients[0].NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("could not list nodes: %v", err)
	}

	return &status, nodes, nil
}

func findManager(nodes []swarm.Node) *swarm.Node {
	idx := slices.IndexFunc(nodes, func(node swarm.Node) bool {
		return node.ManagerStatus != nil
	})
	if idx == -1 {
		return nil
	}

	return &nodes[idx]
}

func findClientByNodeAddr(clients []*dockerhelper.Client, addr string) *dockerhelper.Client {
	idx := slices.IndexFunc(clients, func(client *dockerhelper.Client) bool {
		ip, _, err := net.SplitHostPort(client.SSH.RemoteAddr().String())
		if err != nil {
			return false
		}

		return ip == addr
	})
	if idx == -1 {
		return nil
	}

	return clients[idx]
}

func joinMissingHosts(ctx context.Context, vikingCli *command.Cli, clients []*dockerhelper.Client, status *swarm.Swarm, nodes []swarm.Node) error {
	manager := findManager(nodes)
	if manager == nil {
		return fmt.Errorf("could not find swarm manager node")
	}

	for _, client := range clients {
		found := false

		ip, _, err := net.SplitHostPort(client.SSH.RemoteAddr().String())
		if err != nil {
			return fmt.Errorf("could not parse node address: %v", err)
		}

		for _, node := range nodes {
			if node.Status.Addr == ip {
				found = true
				break
			}
		}

		if found {
			continue
		}

		fmt.Fprintf(vikingCli.Out, "Joining %s to the swarm\n", client.SSH.RemoteAddr().String())
		if err := client.SwarmJoin(ctx, swarm.JoinRequest{
			JoinToken: status.JoinTokens.Worker,
			RemoteAddrs: []string{
				net.JoinHostPort(manager.ManagerStatus.Addr, "2377"),
			},
		}); err != nil {
			return fmt.Errorf("could not join worker %s to swarm: %v", client.SSH.RemoteAddr().String(), err)
		}
	}

	return nil
}

func ensureVikingNetwork(ctx context.Context, client *dockerhelper.Client) error {
	networks, err := client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.Name == vikingNetworkName {
			return nil
		}
	}

	_, err = client.NetworkCreate(ctx, vikingNetworkName, network.CreateOptions{
		Driver: "overlay",
	})
	return err
}

type healthOptions struct {
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
	Cmd      string
}

type rollbackOptions struct {
	Delay time.Duration
}

type serviceOptions struct {
	Name            string
	Image           string
	Cmd             []string
	Replicas        uint64
	Labels          map[string]string
	Env             []string
	Bind            []string
	Network         []string
	Port            string
	Health          healthOptions
	Rollback        rollbackOptions
	StopGracePeriod time.Duration
	StopSignal      string
}

func runService(
	ctx context.Context,
	vikingCli *command.Cli,
	client *dockerhelper.Client,
	options serviceOptions,
) error {
	healthTest := []string{"NONE"}
	if options.Health.Cmd != "" {
		healthTest = []string{"CMD-SHELL", options.Health.Cmd}
	}

	serviceSpec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   options.Name,
			Labels: options.Labels,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image:           options.Image,
				StopSignal:      options.StopSignal,
				Env:             options.Env,
				Command:         options.Cmd,
				StopGracePeriod: &options.StopGracePeriod,
				Healthcheck: &container.HealthConfig{
					Test:     healthTest,
					Interval: options.Health.Interval,
					Timeout:  options.Health.Timeout,
					Retries:  options.Health.Retries,
				},
			},
			Networks: buildNetworks(options.Network),
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &options.Replicas,
			},
		},
		UpdateConfig: &swarm.UpdateConfig{
			Parallelism:     1,
			Delay:           10 * time.Second,
			FailureAction:   swarm.UpdateFailureActionPause,
			Monitor:         15 * time.Second,
			MaxFailureRatio: 0.15,
		},
		RollbackConfig: &swarm.UpdateConfig{
			Parallelism:     1,
			Delay:           options.Rollback.Delay,
			FailureAction:   swarm.UpdateFailureActionPause,
			Monitor:         15 * time.Second,
			MaxFailureRatio: 0.15,
		},
	}

	if options.Port != "" {
		parts := strings.Split(options.Port, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid port format: %s", options.Port)
		}

		hostPort, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid port format: %s", parts[0])
		}

		containerPort, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid port format: %s", parts[1])
		}

		serviceSpec.EndpointSpec = &swarm.EndpointSpec{
			Ports: []swarm.PortConfig{
				{
					Protocol:      swarm.PortConfigProtocolTCP,
					TargetPort:    uint32(containerPort),
					PublishedPort: uint32(hostPort),
				},
			},
		}
	}

	if len(options.Bind) > 0 {
		mounts, err := buildBindMounts(options.Bind)
		if err != nil {
			return err
		}

		serviceSpec.TaskTemplate.ContainerSpec.Mounts = mounts
	}

	existing, err := client.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters.NewArgs(
			filters.Arg("name", options.Name),
		),
	})
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		fmt.Fprintf(vikingCli.Out, "Updating service %s...\n", options.Name)
		metadata, _, err := client.ServiceInspectWithRaw(ctx, existing[0].ID, types.ServiceInspectOptions{})
		if err != nil {
			return err
		}

		_, err = client.ServiceUpdate(ctx, existing[0].ID, metadata.Version, serviceSpec, types.ServiceUpdateOptions{})
		return err
	} else {
		_, err := client.ServiceCreate(ctx, serviceSpec, types.ServiceCreateOptions{})
		return err
	}
}

func buildBindMounts(binds []string) ([]mount.Mount, error) {
	mounts := make([]mount.Mount, len(binds))
	for i, bind := range binds {
		parts := strings.Split(bind, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid bind mount: %s", bind)
		}
		src, dst := parts[0], parts[1]

		mounts[i] = mount.Mount{
			Type:   mount.TypeBind,
			Source: src,
			Target: dst,
		}
	}

	return mounts, nil
}

func buildNetworks(networks []string) []swarm.NetworkAttachmentConfig {
	networkConfigs := make([]swarm.NetworkAttachmentConfig, len(networks))
	for i, network := range networks {
		networkConfigs[i] = swarm.NetworkAttachmentConfig{Target: network}
	}
	return networkConfigs
}
