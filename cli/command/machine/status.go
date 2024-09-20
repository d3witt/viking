package machine

import (
	"context"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config/appconf"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/d3witt/viking/parallel"
	"github.com/urfave/cli/v2"
)

type machineStatus struct {
	IP          string
	Reachable   string
	Docker      string
	SwarmStatus string
	Role        string
}

func NewStatusCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Check the status of machines",
		Action: func(ctx *cli.Context) error {
			return runStatus(ctx.Context, vikingCli)
		},
	}
}

func runStatus(ctx context.Context, vikingCli *command.Cli) error {
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
		{"IP", "Reachable", "Docker", "Swarm Status", "Role"},
	}

	for _, status := range statuses {
		data = append(data, []string{
			status.IP,
			status.Reachable,
			status.Docker,
			status.SwarmStatus,
			status.Role,
		})
	}

	command.PrintTable(vikingCli.Out, data)
}
