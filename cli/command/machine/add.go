package machine

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config/appconf"
	"github.com/urfave/cli/v2"
)

func NewAddCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Adds machines to the viking.toml configuration file",
		Args:      true,
		ArgsUsage: "[USER@]HOST[:PORT]...",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "SSH key name",
			},
		},
		Action: func(ctx *cli.Context) error {
			hosts := ctx.Args().Slice()
			key := ctx.String("key")

			return runAdd(vikingCli, hosts, key)
		},
	}
}

func runAdd(vikingCli *command.Cli, hosts []string, key string) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	if key != "" {
		if _, err := vikingCli.Config.GetKeyByName(key); err != nil {
			return err
		}
	}

	machines, err := buildMachines(hosts, key)
	if err != nil {
		return err
	}

	err = conf.AddMachine(machines...)
	if err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "Machines %v added to configuration. Remember to run 'viking sync' to apply changes to the Swarm.\n", hosts)

	return nil
}

func parseMachine(val string) (user string, ip net.IP, port int, err error) {
	user = "root"
	port = 22

	if idx := strings.Index(val, "@"); idx != -1 {
		user = val[:idx]
		val = val[idx+1:]
	}

	host, portStr, splitErr := net.SplitHostPort(val)
	if splitErr != nil {
		host = val
	} else {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			err = errors.New("invalid port number")
			return
		}
	}

	ip = net.ParseIP(host)
	if ip == nil {
		err = errors.New("invalid IP address")
	}

	return
}

func buildMachines(hosts []string, key string) ([]appconf.Machine, error) {
	machines := make([]appconf.Machine, 0, len(hosts))
	for _, host := range hosts {
		user, ip, port, err := parseMachine(host)
		if err != nil {
			return nil, err
		}

		m := appconf.Machine{
			User: user,
			IP:   ip,
			Port: port,
			Key:  key,
		}

		machines = append(machines, m)
	}

	return machines, nil
}
