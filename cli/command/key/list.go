package key

import (
	"sort"

	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"
	"github.com/workdate-dev/viking/cli/command"
)

func NewListCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "List all SSH keys",
		Action: func(ctx *cli.Context) error {
			return listKeys(vikingCli)
		},
	}
}

func listKeys(vikingCli *command.Cli) error {
	keys := vikingCli.Config.ListKeys()

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreatedAt.After(keys[j].CreatedAt)
	})

	data := [][]string{
		{
			"NAME",
			"CREATED",
		},
	}

	for _, machine := range keys {
		data = append(data, []string{
			machine.Name,
			humanize.Time(machine.CreatedAt),
		})
	}

	command.PrintTable(vikingCli.Out, data)

	return nil
}
