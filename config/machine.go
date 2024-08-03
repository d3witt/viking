package config

import (
	"errors"
	"net"
	"time"
)

type Machine struct {
	Name      string `toml:"-"`
	Host      net.IP
	User      string
	Key       string
	CreatedAt time.Time
}

var (
	ErrMachineNotFound           = errors.New("machine not found")
	ErrMachineAlreadyExists      = errors.New("machine already exists")
	ErrMachineNameOrHostRequired = errors.New("machine name or host is required")
)

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
	if machine == "" {
		return Machine{}, ErrMachineNameOrHostRequired
	}

	if m, err := c.GetMachineByName(machine); err == nil {
		return m, nil
	}

	if m, err := c.GetMachineByHost(net.ParseIP(machine)); err == nil {
		return m, nil
	}

	return Machine{}, ErrMachineNotFound
}

func (c *Config) GetMachineByName(name string) (Machine, error) {
	if machine, ok := c.Machines[name]; ok {
		machine.Name = name
		return machine, nil
	}

	return Machine{}, ErrMachineNotFound
}

func (c *Config) GetMachineByHost(host net.IP) (Machine, error) {
	for name, machine := range c.Machines {
		if machine.Host.Equal(host) {
			machine.Name = name
			return machine, nil
		}
	}

	return Machine{}, ErrMachineNotFound
}

func (c *Config) AddMachine(machine Machine) error {
	_, err := c.GetMachine(machine.Name)
	if err == nil {
		return ErrMachineAlreadyExists
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
