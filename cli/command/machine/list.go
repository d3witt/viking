package machine

import (
	"sort"
	"strconv"

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

	data := [][]string{}

	for _, machine := range machines {
		firstHost := machine.Hosts[0]
		data = append(data, []string{
			machine.Name,
			firstHost.IP.String(),
			strconv.Itoa(firstHost.Port),
			firstHost.Key,
			humanize.Time(machine.CreatedAt),
		})

		for _, host := range machine.Hosts[1:] {
			data = append(data, []string{
				" ",
				host.IP.String(),
				strconv.Itoa(host.Port),
				host.Key,
				" ",
			})
		}
	}

	command.PrintTable(vikingCli.Out, data)

	return nil
}
