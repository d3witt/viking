package config

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/d3witt/viking/dockerhelper"
	"github.com/d3witt/viking/sshexec"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"golang.org/x/crypto/ssh"
)

type Machine struct {
	Name      string `toml:"-"`
	Host      net.IP
	User      string
	Key       string
	CreatedAt time.Time
}

func (c *Config) ListMachines() []Machine {
	machines := make([]Machine, 0, len(c.Machines))

	for name, machine := range c.Machines {
		machine.Name = name
		machines = append(machines, machine)
	}

	return machines
}

// GetMachine returns a machine by name or host.
func (c *Config) GetMachine(machine string) (Machine, error) {
	if m, err := c.GetMachineByName(machine); err == nil {
		return m, nil
	}

	if m, err := c.GetMachineByHost(net.ParseIP(machine)); err == nil {
		return m, nil
	}

	return Machine{}, fmt.Errorf("Machine not found: %s", machine)
}

func (c *Config) GetMachineByName(name string) (Machine, error) {
	if machine, ok := c.Machines[name]; ok {
		return machine, nil
	}

	return Machine{}, fmt.Errorf("Machine not found: %s", name)
}

func (c *Config) GetMachineByHost(host net.IP) (Machine, error) {
	for _, machine := range c.Machines {
		if machine.Host.Equal(host) {
			return machine, nil
		}
	}

	return Machine{}, fmt.Errorf("Machine not found: %s", host)
}

func (c *Config) AddMachine(machine Machine) error {
	if _, ok := c.Machines[machine.Name]; ok {
		return fmt.Errorf("Machine already exists: %s", machine.Name)
	}

	c.Machines[machine.Name] = machine

	return c.Save()
}

// RemoveMachine removes a machine from the config by name or host.
func (c *Config) RemoveMachine(machine string) error {
	m, err := c.GetMachine(machine)
	if err != nil {
		return err
	}

	delete(c.Machines, m.Name)

	return c.Save()
}

const vikingNetwork = "viking"

func (m *Machine) Configure(ctx context.Context, sshClient *ssh.Client) error {
	if err := dockerhelper.InstallDockerIfMissing(sshexec.NewExecutor(sshClient)); err != nil {
		return fmt.Errorf("Failed to check or install docker: %w", err)
	}

	sshAdd := fmt.Sprintf("ssh://%s@%s:22", m.User, m.Host.String())
	dockerClient, err := dockerhelper.Client(sshAdd)
	if err != nil {
		return fmt.Errorf("Failed to create docker client: %w", err)
	}

	if _, err := dockerClient.SwarmInspect(ctx); err != nil {
		if client.IsErrNotFound(err) {
			if _, err := dockerClient.SwarmInit(ctx, swarm.InitRequest{}); err != nil {
				return fmt.Errorf("Failed to initialize swarm: %w", err)
			}
		} else {
			return fmt.Errorf("Failed to inspect swarm: %w", err)
		}
	}

	if _, err := dockerClient.NetworkInspect(ctx, vikingNetwork, network.InspectOptions{}); err != nil {
		if client.IsErrNotFound(err) {
			networkOptions := network.CreateOptions{
				Driver:  "overlay",
				Scope:   "swarm",
				Ingress: true,
				Labels: map[string]string{
					"viking": "true",
				},
			}
			if _, err := dockerClient.NetworkCreate(ctx, vikingNetwork, networkOptions); err != nil {
				return fmt.Errorf("Failed to create network: %w", err)
			}
		} else {
			return fmt.Errorf("Failed to inspect network: %w", err)
		}
	}

	return nil
}
