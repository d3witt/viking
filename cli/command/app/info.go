package app

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/d3witt/viking/cli/command"
	"github.com/d3witt/viking/dockerhelper"
	"github.com/urfave/cli/v2"
)

func NewInfoCmd(vikingCli *command.Cli) *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Display information about the app",
		Action: func(ctx *cli.Context) error {
			return runInfo(ctx.Context, vikingCli)
		},
	}
}

func runInfo(ctx context.Context, vikingCli *command.Cli) error {
	var info struct {
		AppName  string
		Status   string
		Image    string
		Replicas uint64
		Ports    []string
		Networks []string
		Env      map[string]string
		Labels   map[string]string
		Tasks    []struct {
			IP     string
			Status string
		}
	}

	conf, err := vikingCli.AppConfig()
	if err != nil {
		return fmt.Errorf("failed to get app config: %w", err)
	}
	info.AppName = conf.Name

	sshClients, err := vikingCli.DialMachines(ctx)
	if err != nil {
		return fmt.Errorf("failed to dial machines: %w", err)
	}
	defer command.CloseSSHClients(sshClients)

	swarm, err := vikingCli.Swarm(ctx, sshClients)
	if err != nil {
		return fmt.Errorf("failed to get swarm: %w", err)
	}
	defer swarm.Close()

	service, err := dockerhelper.GetService(ctx, swarm, conf.Name)
	if err != nil {
		return fmt.Errorf("failed to get service info: %w", err)
	}

	if service == nil {
		info.Status = "Not deployed"
	} else {
		info.Status = "Deployed"
		info.Image = service.Spec.TaskTemplate.ContainerSpec.Image
		info.Replicas = *service.Spec.Mode.Replicated.Replicas

		for _, port := range service.Endpoint.Ports {
			info.Ports = append(info.Ports, fmt.Sprintf("%d:%d", port.PublishedPort, port.TargetPort))
		}

		for _, network := range service.Spec.TaskTemplate.Networks {
			info.Networks = append(info.Networks, network.Target)
		}

		info.Env = make(map[string]string)
		for _, env := range service.Spec.TaskTemplate.ContainerSpec.Env {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				info.Env[parts[0]] = parts[1]
			}
		}

		info.Labels = service.Spec.Labels

		tasks, err := dockerhelper.ListTasks(ctx, swarm, service.ID)
		if err != nil {
			return fmt.Errorf("failed to list tasks: %w", err)
		}

		for _, task := range tasks {
			node, _ := swarm.GetNode(ctx, task.NodeID)

			info.Tasks = append(info.Tasks, struct {
				IP     string
				Status string
			}{
				IP:     node.Status.Addr,
				Status: string(task.Status.State),
			})
		}
	}

	printInfo(vikingCli.Out, &info)
	return nil
}

func printInfo(w io.Writer, info *struct {
	AppName  string
	Status   string
	Image    string
	Replicas uint64
	Ports    []string
	Networks []string
	Env      map[string]string
	Labels   map[string]string
	Tasks    []struct {
		IP     string
		Status string
	}
},
) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "Name:\t%s\n", info.AppName)
	fmt.Fprintf(tw, "Status:\t%s\n", info.Status)
	if info.Status == "Deployed" {
		fmt.Fprintf(tw, "Image:\t%s\n", info.Image)
		fmt.Fprintf(tw, "Replicas:\t%d\n", info.Replicas)
	}
	tw.Flush()

	if info.Status == "Deployed" {
		printList(w, "Ports", info.Ports)
		printList(w, "Networks", info.Networks)
		printMap(w, "Environment Variables", info.Env)
		printMap(w, "Labels", info.Labels)
	}

	if len(info.Tasks) > 0 {
		fmt.Fprintln(w, "Machines:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, task := range info.Tasks {
			fmt.Fprintf(tw, "  %s\t%s\n", task.IP, task.Status)
		}
		tw.Flush()
	}
}

func printList(w io.Writer, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	for _, item := range items {
		fmt.Fprintf(w, "  - %s\n", item)
	}
}

func printMap(w io.Writer, title string, data map[string]string) {
	if len(data) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, k := range keys {
		fmt.Fprintf(tw, "  %s:\t%s\n", k, data[k])
	}
	tw.Flush()
}
