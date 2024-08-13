package machine

import (
	"net"
	"sort"
	"strconv"
	"strings"

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
			"HOSTS",
			"PORT",
			"KEY",
			"CREATED",
		},
	}

	for _, machine := range machines {
		host := strings.Join(func(hosts []net.IP) []string {
			strs := make([]string, len(hosts))
			for i, h := range hosts {
				strs[i] = h.String()
			}
			return strs
		}(machine.Host), ", ")

		data = append(data, []string{
			machine.Name,
			host,
			strconv.Itoa(machine.ConnPort()),
			machine.Key,
			humanize.Time(machine.CreatedAt),
		})
	}

	command.PrintTable(vikingCli.Out, data)

	return nil
}
