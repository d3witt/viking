package appconf

import (
	"errors"
	"net"
)

type Machine struct {
	IP   net.IP `toml:"-"`
	Port int    `toml:"port,omitempty"`
	User string `toml:"user,omitempty"`
	Key  string `toml:"key,omitempty"`
}

var (
	ErrMachineNotFound           = errors.New("machine not found")
	ErrMachineAlreadyExists      = errors.New("machine already exists")
	ErrMachineNameOrHostRequired = errors.New("machine name or host is required")
)

func (c *Config) ListMachines() []Machine {
	machines := make([]Machine, 0, len(c.Machines))

	for ip, machine := range c.Machines {
		machine.IP = net.ParseIP(ip)

		if machine.IP == nil {
			continue
		}

		setMachineDefaults(&machine)

		machines = append(machines, machine)
	}

	return machines
}

func (c *Config) GetMachine(ip string) (Machine, error) {
	if machine, ok := c.Machines[ip]; ok {
		machine.IP = net.ParseIP(ip)

		if machine.IP == nil {
			return Machine{}, ErrMachineNotFound
		}

		setMachineDefaults(&machine)

		return machine, nil
	}

	return Machine{}, ErrMachineNotFound
}

func (c *Config) AddMachine(machine ...Machine) error {
	for _, m := range machine {
		_, err := c.GetMachine(m.IP.String())
		if err == nil {
			return ErrMachineAlreadyExists
		}

		c.Machines[m.IP.String()] = m
	}

	return c.Save()
}

func (c *Config) RemoveMachine(ip string) error {
	_, err := c.GetMachine(ip)
	if err != nil {
		return err
	}

	delete(c.Machines, ip)

	return c.Save()
}

func (c *Config) ClearMachines() error {
	c.Machines = map[string]Machine{}

	return c.Save()
}

func setMachineDefaults(machine *Machine) {
	if machine.Port == 0 {
		machine.Port = 22
	}

	if machine.User == "" {
		machine.User = "root"
	}
}
