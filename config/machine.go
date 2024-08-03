package config

import (
	"fmt"
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

var MachineNotFoundError = fmt.Errorf("machine not found")
var MachineAlreadyExistsError = fmt.Errorf("machine already exists")
var MachineNameOrHostRequiredError = fmt.Errorf("machine name or host is required")

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
		return Machine{}, MachineNameOrHostRequiredError
	}

	if m, err := c.GetMachineByName(machine); err == nil {
		return m, nil
	}

	if m, err := c.GetMachineByHost(net.ParseIP(machine)); err == nil {
		return m, nil
	}

	return Machine{}, MachineNotFoundError
}

func (c *Config) GetMachineByName(name string) (Machine, error) {
	if machine, ok := c.Machines[name]; ok {
		machine.Name = name
		return machine, nil
	}

	return Machine{}, MachineNotFoundError
}

func (c *Config) GetMachineByHost(host net.IP) (Machine, error) {
	for name, machine := range c.Machines {
		if machine.Host.Equal(host) {
			machine.Name = name
			return machine, nil
		}
	}

	return Machine{}, MachineNotFoundError
}

func (c *Config) AddMachine(machine Machine) error {
	_, err := c.GetMachine(machine.Name)
	if err == nil {
		return MachineAlreadyExistsError
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
