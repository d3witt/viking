package machine

import (
	"fmt"

	"github.com/d3witt/viking/cli/command"
	"github.com/urfave/cli/v2"
)

func NewRmCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Aliases:   []string{"rm"},
		Usage:     "Removes machines from the viking.toml configuration file.",
		Args:      true,
		ArgsUsage: "[IP]...",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Automatically confirm the remove operation without prompting.",
			},
		},
		Action: func(ctx *cli.Context) error {
			machines := ctx.Args().Slice()
			yes := ctx.Bool("yes")

			return runRemove(vikingCli, machines, yes)
		},
	}
}

func runRemove(vikingCli *command.Cli, machines []string, yes bool) error {
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

	if len(machines) == 0 {
		if err := conf.ClearMachines(); err != nil {
			return err
		}

		fmt.Fprintln(vikingCli.Out, "Machines removed from configuration. Remember to run 'viking sync' to apply changes to the Swarm.")
	}

	if err := conf.RemoveMachines(machines...); err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "Machines %v removed from configuration. Remember to run 'viking sync' to apply changes to the Swarm.\n", machines)

	return nil
}
