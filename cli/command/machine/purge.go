package machine

import (
	"context"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
)

func NewPurgeCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "purge",
		Usage: "Purge machines",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Automatically confirm the purge operation without prompting.",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Force removal of machines from the Docker Swarm, bypassing safety checks.",
			},
		},
		Args:      true,
		ArgsUsage: "[IP]...",
		Action: func(ctx *cli.Context) error {
			force := ctx.Bool("force")
			yes := ctx.Bool("yes")
			machines := ctx.Args().Slice()

			return runPurge(ctx.Context, vikingCli, machines, force, yes)
		},
	}
}

func runPurge(ctx context.Context, vikingCli *command.Cli, machines []string, force, yes bool) error {
	message := fmt.Sprintf("You want to purge machine %s. Are you sure?", machines)
	if len(machines) == 0 {
		message = "You want to purge all machines. Are you sure?"
	}

	if !yes {
		confirmed, err := command.PromptForConfirmation(vikingCli.In, vikingCli.Out, message)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}

	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	if len(machines) == 0 || len(machines) == len(conf.Machines) {
		return purgeAllMachines(ctx, vikingCli)
	}

	return purgeMachines(ctx, vikingCli, machines, force)
}

func purgeAllMachines(ctx context.Context, vikingCli *command.Cli) error {
	sshClients := vikingCli.DialAvailableMachines(ctx)
	if sshClients == nil {
		fmt.Fprintln(vikingCli.Out, "No available machines to purge.")
		return nil
	}
	defer command.CloseSSHClients(sshClients)

	swarm, err := vikingCli.SwarmAvailable(ctx, sshClients)
	if err != nil {
		return err
	}
	defer swarm.Close()

	if !swarm.Exists(ctx) {
		fmt.Fprintln(vikingCli.Out, "Swarm is not initialized.")
		return nil
	}

	swarm.LeaveSwarm(ctx)
	fmt.Fprintln(vikingCli.Out, "Swarm left.")
	return nil
}

func purgeMachines(ctx context.Context, vikingCli *command.Cli, machines []string, force bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	sshClients := vikingCli.DialAvailableMachines(ctx)
	defer command.CloseSSHClients(sshClients)

	swarm, err := vikingCli.SwarmAvailable(ctx, sshClients)
	if err != nil {
		fmt.Fprintf(vikingCli.Out, "Swarm is not available: %s\n", err)
		return nil
	}

	for _, machine := range machines {
		m, err := conf.GetMachine(machine)
		if err != nil {
			fmt.Fprintf(vikingCli.Out, "%s: %s\n", machine, err)
			continue
		}

		node := swarm.GetClientByAddr(m.IP.String())
		if node == nil {
			fmt.Fprintf(vikingCli.Out, "%s: not in swarm\n", machine)
			continue
		}

		swarm.LeaveNode(ctx, node, force)
	}

	return nil
}
