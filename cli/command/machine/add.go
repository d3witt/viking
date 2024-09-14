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
		Name:        "add",
		Usage:       "Add a new machine(s)",
		Description: "This command adds a new machine(s) to the viking config. No action is taken on the machine itself.",
		Args:        true,
		ArgsUsage:   "[USER@]HOST[:PORT]...",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "user",
				Aliases: []string{"u"},
				Value:   "root",
				Usage:   "SSH user name",
			},
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "SSH key name",
			},
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   22,
			},
		},
		Action: func(ctx *cli.Context) error {
			hosts := ctx.Args().Slice()
			user := ctx.String("user")
			key := ctx.String("key")
			port := ctx.Int("port")

			return runAdd(vikingCli, hosts, port, user, key)
		},
	}
}

func runAdd(vikingCli *command.Cli, hosts []string, port int, user, key string) error {
	if key != "" {
		_, err := vikingCli.Config.GetKeyByName(key)
		if err != nil {
			return err
		}
	}

	machines := make([]appconf.Machine, 0, len(hosts))
	for _, host := range hosts {
		user, hostIp, port, err := parseMachine(host, user, port)
		if err != nil {
			return err
		}

		m := appconf.Machine{
			IP:   hostIp,
			Port: port,
			User: user,
			Key:  key,
		}

		machines = append(machines, m)
	}

	conf, err := vikingCli.AppConfig()
	if err != nil {
		return err
	}

	err = conf.AddMachine(machines...)
	if err != nil {
		return err
	}

	fmt.Fprint(vikingCli.Out, strings.Join(hosts, ", "))

	return nil
}

func parseMachine(val, defaultUser string, defaultPort int) (user string, ip net.IP, port int, err error) {
	user = defaultUser
	port = defaultPort

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
