package machine

import (
	"sort"

	"github.com/d3witt/viking/cli/command"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"
)

func NewListCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "List machines",
		Action: func(ctx *cli.Context) error {
			return listMachines(vikingCli)
		},
	}
}

func listMachines(vikingCli *command.Cli) error {
	machines := vikingCli.Config.ListMachines()

	sort.Slice(machines, func(i, j int) bool {
		return machines[i].CreatedAt.After(machines[j].CreatedAt)
	})

	data := [][]string{
		{
			"NAME",
			"HOST",
			"CREATED",
		},
	}

	for _, machine := range machines {
		data = append(data, []string{
			machine.Name,
			machine.Host.String(),
			humanize.Time(machine.CreatedAt),
		})
	}

	command.PrintTable(vikingCli.Out, data)

	return nil
}
