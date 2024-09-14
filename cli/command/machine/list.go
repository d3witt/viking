package machine

import (
	"strconv"

	"github.com/d3witt/viking/cli/command"
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
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	machines := conf.ListMachines()

	data := [][]string{
		{" ", "IP", "Port", "Key"},
	}

	for i, machine := range machines {
		data = append(data, []string{
			strconv.Itoa(i + 1),
			machine.IP.String(),
			strconv.Itoa(machine.Port),
			machine.Key,
		})
	}

	command.PrintTable(vikingCli.Out, data)

	return nil
}
