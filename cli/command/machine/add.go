package machine

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config"
	"github.com/urfave/cli/v2"
)

func NewAddCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:        "add",
		Usage:       "Add a new machine",
		Description: "This command adds a new machine to the list of machines. No action is taken on the machine itself. Ensure your computer has SSH access to this machine.",
		Args:        true,
		ArgsUsage:   "[USER@]HOST[:PORT]...",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Machine name",
			},
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
			name := ctx.String("name")
			user := ctx.String("user")
			key := ctx.String("key")
			port := ctx.Int("port")

			return runAdd(vikingCli, hosts, port, name, user, key)
		},
	}
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

func runAdd(vikingCli *command.Cli, hosts []string, port int, name, user, key string) error {
	if name == "" {
		name = command.GenerateRandomName()
	}

	if key != "" {
		_, err := vikingCli.Config.GetKeyByName(key)
		if err != nil {
			return err
		}
	}

	m := config.Machine{
		Name:      name,
		Hosts:     []config.Host{},
		CreatedAt: time.Now(),
	}

	for _, host := range hosts {
		user, hostIp, port, err := parseMachine(host, user, port)
		if err != nil {
			return err
		}

		m.Hosts = append(m.Hosts, config.Host{
			IP:   hostIp,
			Port: port,
			User: user,
			Key:  key,
		})
	}

	if err := vikingCli.Config.AddMachine(m); err != nil {
		return err
	}

	fmt.Fprintf(vikingCli.Out, "Machine %s added.\n", name)

	return nil
}
