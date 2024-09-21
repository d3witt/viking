package machine

import (
	"context"
	"fmt"
	"strconv"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config/appconf"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/d3witt/viking/parallel"
	"github.com/urfave/cli/v2"
)

type machineStatus struct {
	IP          string
	Port        string
	Key         string
	Reachable   string
	Docker      string
	SwarmStatus string
	Role        string
}

func NewListCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List all machines",
		Action: func(ctx *cli.Context) error {
			return runList(ctx.Context, vikingCli)
		},
	}
}

func runList(ctx context.Context, vikingCli *command.Cli) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return fmt.Errorf("failed to get app config: %w", err)
	}

	machines := conf.ListMachines()
	statuses := make([]machineStatus, len(machines))

	parallel.ForEach(ctx, len(machines), func(i int) {
		machine := machines[i]
		statuses[i] = checkMachineStatus(ctx, vikingCli, machine)
	})

	printStatusTable(vikingCli, statuses)
	return nil
}

func checkMachineStatus(ctx context.Context, vikingCli *command.Cli, machine appconf.Machine) machineStatus {
	status := machineStatus{
		IP:        machine.IP.String(),
		Port:      strconv.Itoa(machine.Port),
		Key:       machine.Key,
		Reachable: "No",
		Docker:    "Not installed",
	}

	client, err := vikingCli.DialMachine(machine.IP.String())
	if err != nil {
		return status
	}
	defer client.Close()

	status.Reachable = "Yes"

	if dockerhelper.IsDockerInstalled(client) {
		status.Docker = "Installed"

		dockerClient, err := dockerhelper.DialSSH(client)
		if err != nil {
			status.SwarmStatus = "Error getting Docker info"
			return status
		}
		defer dockerClient.Close()

		info, err := dockerClient.Info(ctx)
		if err != nil {
			status.SwarmStatus = "Error getting Docker info"
			return status
		}

		status.SwarmStatus = string(info.Swarm.LocalNodeState)

		if info.Swarm.ControlAvailable {
			status.Role = "Manager"
		} else {
			status.Role = "Worker"
		}
	}

	return status
}

func printStatusTable(vikingCli *command.Cli, statuses []machineStatus) {
	data := [][]string{
		{"IP", "Port", "Key", "Reachable", "Docker", "Swarm Status", "Role"},
	}

	for _, status := range statuses {
		data = append(data, []string{
			status.IP,
			status.Port,
			status.Key,
			status.Reachable,
			status.Docker,
			status.SwarmStatus,
			status.Role,
		})
	}

	command.PrintTable(vikingCli.Out, data)
}
