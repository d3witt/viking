package machine

import (
	"context"
	"fmt"
	"sync"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config/appconf"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/docker/docker/api/types/swarm"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

type machineStatus struct {
	IP              string
	Reachable       bool
	DockerInstalled bool
	SwarmStatus     string
	Role            string
}

func NewStatusCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:   "status",
		Usage:  "Check the status of machines",
		Action: runStatus(vikingCli),
	}
}

func runStatus(vikingCli *command.Cli) func(*cli.Context) error {
	return func(cliCtx *cli.Context) error {
		conf, err := vikingCli.AppConfig()
		if err != nil {
			return fmt.Errorf("failed to get app config: %w", err)
		}

		machines := conf.ListMachines()
		statuses := checkMachineStatuses(cliCtx.Context, vikingCli, machines)

		printStatusTable(vikingCli, statuses)
		return nil
	}
}

func checkMachineStatuses(ctx context.Context, vikingCli *command.Cli, machines []appconf.Machine) []machineStatus {
	statuses := make([]machineStatus, len(machines))
	var wg sync.WaitGroup
	wg.Add(len(machines))

	for i, machine := range machines {
		go func(i int, m appconf.Machine) {
			defer wg.Done()
			statuses[i] = checkMachineStatus(ctx, vikingCli, m)
		}(i, machine)
	}

	wg.Wait()
	return statuses
}

func checkMachineStatus(ctx context.Context, vikingCli *command.Cli, machine appconf.Machine) machineStatus {
	status := machineStatus{
		IP:              machine.IP.String(),
		Reachable:       false,
		DockerInstalled: false,
		SwarmStatus:     "Not in swarm",
		Role:            "",
	}

	client, err := vikingCli.DialMachine(machine.IP.String())
	if err != nil {
		return status
	}
	defer client.Close()

	status.Reachable = true

	if dockerhelper.IsDockerInstalled(client) {
		status.DockerInstalled = true
		status.SwarmStatus, status.Role = checkSwarmStatus(ctx, client)
	}

	return status
}

func checkSwarmStatus(ctx context.Context, client *ssh.Client) (string, string) {
	dockerClient, err := dockerhelper.DialSSH(client)
	if err != nil {
		return "Error checking swarm status", ""
	}
	defer dockerClient.Close()

	info, err := dockerClient.Info(ctx)
	if err != nil {
		return "Error getting Docker info", ""
	}

	if info.Swarm.LocalNodeState == swarm.LocalNodeStateActive {
		role := "Worker"
		if info.Swarm.ControlAvailable {
			role = "Manager"
		}

		return "In swarm", role
	}

	return "Not in swarm", ""
}

func printStatusTable(vikingCli *command.Cli, statuses []machineStatus) {
	data := [][]string{
		{"IP", "Online", "Docker Ready", "Swarm Status", "Role"},
	}

	for _, status := range statuses {
		data = append(data, []string{
			status.IP,
			boolToYesNo(status.Reachable),
			boolToYesNo(status.DockerInstalled),
			status.SwarmStatus,
			status.Role,
		})
	}

	command.PrintTable(vikingCli.Out, data)
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
