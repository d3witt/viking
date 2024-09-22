package machine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/config/appconf"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/d3witt/viking/parallel"
	"github.com/d3witt/viking/sshexec"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewAddCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add machines to the config and join them to the Swarm",
		Args:      true,
		ArgsUsage: "[USER@]HOST[:PORT]...",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "SSH key name",
			},
			&cli.BoolFlag{
				Name:  "config-only",
				Usage: "Do not join the machines to the Swarm after adding",
			},
		},
		Action: func(ctx *cli.Context) error {
			hosts := ctx.Args().Slice()
			key := ctx.String("key")
			configOnly := ctx.Bool("config-only")

			return runAdd(ctx.Context, vikingCli, hosts, key, configOnly)
		},
	}
}

func runAdd(ctx context.Context, vikingCli *command.Cli, hosts []string, key string, configOnly bool) error {
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

	if !configOnly {
		joinMachinesToSwarm(ctx, vikingCli, machines)
	}

	err = conf.AddMachine(machines...)
	if err != nil {
		return err
	}

	fmt.Fprintln(vikingCli.Out, strings.Join(hosts, ", "))

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

func joinMachinesToSwarm(ctx context.Context, vikingCli *command.Cli, machines []appconf.Machine) error {
	swarm, err := vikingCli.DialSwarm(ctx)
	if err != nil {
		return err
	}
	defer swarm.Close()

	sshClients := make([]*ssh.Client, len(machines))
	defer func() {
		for _, client := range sshClients {
			if client != nil {
				client.Close()
			}
		}
	}()

	if err := parallel.RunFirstErr(ctx, len(machines), func(i int) error {
		m := machines[i]

		private, passphrase, err := vikingCli.GetSSHKeyDetails(m.Key)
		if err != nil {
			return err
		}

		sshClients[i], err = sshexec.SSHClient(m.IP.String(), m.Port, m.User, private, passphrase)
		return err
	}); err != nil {
		return err
	}

	if err := checkDockerInstalled(ctx, vikingCli, sshClients); err != nil {
		return err
	}

	dockerClients := make([]*dockerhelper.Client, len(sshClients))
	defer func() {
		for _, client := range dockerClients {
			if client != nil {
				client.Close()
			}
		}
	}()

	if err := parallel.RunFirstErr(ctx, len(sshClients), func(i int) error {
		client := sshClients[i]

		dockerClients[i], err = dockerhelper.DialSSH(client)
		return err
	}); err != nil {
		return err
	}

	if !swarm.Exists(ctx) {
		swarm.Clients = dockerClients

		fmt.Fprintln(vikingCli.Out, "Swarm does not exist. Creating a new one...")
		return swarm.Init(ctx)
	}

	return swarm.JoinNodes(ctx, dockerClients)
}
