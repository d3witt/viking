package machine

import (
	"context"
	"errors"
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
)

func NewRmCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:        "rm",
		Usage:       "Remove a machine(s)",
		Description: `Remove machines from config. The --leave flag removes the machine from the Docker Swarm. By default, errors stop the removal process. Use --force to ignore errors and continue removal.`,
		Args:        true,
		ArgsUsage:   "[IP]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "leave",
				Usage: "Leave the machine from the swarm and remove the node.",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Force remove the machine.",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Automatically confirm the remove operation without prompting.",
			},
		},
		Action: func(ctx *cli.Context) error {
			machine := ctx.Args().First()
			leave := ctx.Bool("leave")
			force := ctx.Bool("force")
			yes := ctx.Bool("yes")

			return runRemove(ctx.Context, vikingCli, machine, leave, force, yes)
		},
	}
}

func runRemove(ctx context.Context, vikingCli *command.Cli, machine string, leave, force, yes bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	if !yes {
		message := fmt.Sprintf("You want to remove machine %s. Are you sure?", machine)
		if machine == "" {
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

	if machine == "" || len(conf.Machines) == 1 {
		return removeMachines(ctx, vikingCli, leave)
	}

	return removeMachine(ctx, vikingCli, machine, leave, force)
}

func removeMachines(ctx context.Context, vikingCli *command.Cli, leave bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	if leave {
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

func removeMachine(ctx context.Context, vikingCli *command.Cli, machine string, leave, force bool) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	m, err := conf.GetMachine(machine)
	if err != nil {
		return err
	}

	if leave {
		swarm, err := vikingCli.DialSwarm(ctx)
		if err != nil {
			return err
		}
		defer swarm.Close()

		node := swarm.GetClientByAddr(m.IP.String())
		if node == nil {
			if !force {
				return errors.New("target machine docker client is unavailable")
			}
		} else {
			if err := swarm.LeaveNode(ctx, node, force); err != nil {
				return err
			}
		}

		if err := swarm.RemoveNodesByAddr(ctx, m.IP.String(), force); err != nil {
			return err
		}
	}

	if err := conf.RemoveMachine(machine); err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, machine)

	return nil
}
