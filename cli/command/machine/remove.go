package machine

import (
	"context"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
)

func NewRmCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Aliases:   []string{"rm"},
		Usage:     "Remove machines from the configuration and leave them from the Swarm",
		Args:      true,
		ArgsUsage: "[IP]...",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "config-only",
				Usage: "Keep the machines in the Swarm after removal.",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Force removal of the machine from the Swarm.",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Automatically confirm the remove operation without prompting.",
			},
		},
		Action: func(ctx *cli.Context) error {
			machines := ctx.Args().Slice()
			configOnly := ctx.Bool("config-only")
			force := ctx.Bool("force")
			yes := ctx.Bool("yes")

			return runRemove(ctx.Context, vikingCli, machines, configOnly, force, yes)
		},
	}
}

func runRemove(ctx context.Context, vikingCli *command.Cli, machines []string, configOnly, force, yes bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	if !yes {
		message := fmt.Sprintf("You want to remove machine %s. Are you sure?", machines)
		if len(machines) == 0 {
			message = "You want to remove all machines. Are you sure?"
		}

		confirmed, err := command.PromptForConfirmation(vikingCli.In, vikingCli.Out, message)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}

	if len(machines) > 0 {
		for _, machine := range machines {
			if _, err := conf.GetMachine(machine); err != nil {
				return fmt.Errorf("get machine %s: %v", machine, err)
			}
		}
	}

	all := len(machines) == 0 || len(machines) == len(conf.Machines)
	if all {
		return removeAllMachines(ctx, vikingCli, configOnly)
	}

	return removeMachines(ctx, vikingCli, machines, configOnly, force)
}

func removeAllMachines(ctx context.Context, vikingCli *command.Cli, configOnly bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	if !configOnly {
		swarm, err := vikingCli.DialSwarm(ctx)
		if err != nil {
			return err
		}
		defer swarm.Close()

		swarm.LeaveSwarm(ctx)
	}

	if err := conf.ClearMachines(); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, "Machines removed")
	return nil
}

func removeMachines(ctx context.Context, vikingCli *command.Cli, machines []string, configOnly, force bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	var swarm *dockerhelper.Swarm
	if !configOnly {
		swarm, err = vikingCli.DialSwarm(ctx)
		if err != nil {
			return err
		}
		defer swarm.Close()
	}

	for _, machine := range machines {
		m, err := conf.GetMachine(machine)
		if err != nil {
			return fmt.Errorf("get machine %s: %v", machine, err)
		}

		if !configOnly {
			node := swarm.GetClientByAddr(m.IP.String())
			if node == nil {
				if !force {
					return fmt.Errorf("docker client for machine %s is unavailable", machine)
				}
			} else {
				if err := swarm.LeaveNode(ctx, node, force); err != nil {
					return fmt.Errorf("leave node %s: %v", machine, err)
				}
			}
		}

		if err := conf.RemoveMachine(machine); err != nil {
			return fmt.Errorf("remove machine %s from config: %v", machine, err)
		}

		fmt.Fprintln(vikingCli.Out, machine)
	}

	return nil
}
