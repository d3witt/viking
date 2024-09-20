package machine

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

func NewLeaveCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:      "leave",
		Usage:     "Leave machine(s) from Docker Swarm",
		ArgsUsage: "[IP]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Automatically confirm the leave operation without prompting",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Force leave the machine from the swarm.",
			},
		},
		Action: func(ctx *cli.Context) error {
			ip := ctx.Args().First()
			yes := ctx.Bool("yes")
			force := ctx.Bool("force")

			return runLeave(ctx.Context, vikingCli, ip, yes, force)
		},
	}
}

func clientsToLeave(vikingCli *command.Cli, ip string) ([]*ssh.Client, error) {
	if ip == "" {
		return vikingCli.DialMachines()
	}

	client, err := vikingCli.DialMachine(ip)
	if err != nil {
		return nil, err
	}

	return []*ssh.Client{client}, nil
}

func clientsWithDocker(clients []*ssh.Client) []*ssh.Client {
	clientsWithDocker := make([]*ssh.Client, 0, len(clients))
	for _, client := range clients {
		if dockerhelper.IsDockerInstalled(client) {
			clientsWithDocker = append(clientsWithDocker, client)
		}
	}

	return clientsWithDocker
}

func runLeave(ctx context.Context, vikingCli *command.Cli, ip string, yes, force bool) error {
	clients, err := clientsToLeave(vikingCli, ip)
	if err != nil {
		return err
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	ips := make([]string, 0, len(clients))
	for _, client := range clients {
		host, _, err := net.SplitHostPort(client.Conn.RemoteAddr().String())
		if err != nil {
			return err
		}

		ips = append(ips, host)
	}

	if !yes {
		confirmed, err := command.PromptForConfirmation(vikingCli.In, vikingCli.Out, fmt.Sprintf("You want to leave swarm for %s. Are you sure?", strings.Join(ips, ", ")))
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}

	clientsWithDocker := clientsWithDocker(clients)
	if len(clientsWithDocker) > 0 {
		dockerClients := make([]*dockerhelper.Client, 0, len(clientsWithDocker))
		for _, client := range clientsWithDocker {
			dockerClient, err := dockerhelper.DialSSH(client)
			if err != nil {
				closeDockerClients(dockerClients)
				return err
			}

			dockerClients = append(dockerClients, dockerClient)
		}
		defer closeDockerClients(dockerClients)

		if ip == "" {
			dockerhelper.LeaveSwarm(ctx, dockerClients)
		} else {
			manager, err := vikingCli.DialManagerNode(ctx)
			if err != nil {
				return err
			}

			if err := dockerhelper.LeaveNode(ctx, manager, dockerClients[0], force); err != nil {
				return err
			}
		}
	}

	fmt.Fprintln(vikingCli.Out, strings.Join(ips, ", "))
	return nil
}
