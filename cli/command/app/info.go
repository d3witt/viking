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
	"github.com/docker/docker/api/types/swarm"
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

type appInfo struct {
	Name     string
	Status   string
	Image    string
	Replicas uint64
	Ports    []string
	Networks []string
	Env      map[string]string
	Labels   map[string]string
}

func runInfo(ctx context.Context, vikingCli *command.Cli) error {
	conf, err := vikingCli.AppConfig()
	if err != nil {
		return fmt.Errorf("get app config: %v", err)
	}

	info := &appInfo{Name: conf.Name, Status: "Not deployed"}

	sshClient, err := vikingCli.DialMachine()
	if err != nil {
		return fmt.Errorf("dial machine: %v", err)
	}
	defer sshClient.Close()

	if !dockerhelper.IsDockerInstalled(sshClient) {
		printInfo(vikingCli.Out, info)
		return nil
	}

	dockerClient, err := dockerhelper.DialSSH(sshClient)
	if err != nil {
		return fmt.Errorf("dial Docker: %v", err)
	}
	defer dockerClient.Close()

	inactive, err := dockerhelper.IsSwarmInactive(ctx, dockerClient)
	if err != nil {
		return fmt.Errorf("check if Swarm is inactive: %v", err)
	}

	if inactive {
		printInfo(vikingCli.Out, info)
		return nil
	}

	service, err := dockerhelper.GetService(ctx, dockerClient, conf.Name)
	if err != nil {
		return fmt.Errorf("get service info: %v", err)
	}

	if service != nil {
		info.Status = "Deployed"
		info.Image = service.Spec.TaskTemplate.ContainerSpec.Image
		info.Replicas = *service.Spec.Mode.Replicated.Replicas
		info.Ports = formatPorts(service.Endpoint.Ports)
		info.Networks = formatNetworks(service.Spec.TaskTemplate.Networks)
		info.Env = formatEnv(service.Spec.TaskTemplate.ContainerSpec.Env)
		info.Labels = service.Spec.Labels
	}

	printInfo(vikingCli.Out, info)
	return nil
}

func formatPorts(ports []swarm.PortConfig) []string {
	var out []string
	for _, port := range ports {
		out = append(out, fmt.Sprintf("%d:%d", port.PublishedPort, port.TargetPort))
	}
	return out
}

func formatNetworks(networks []swarm.NetworkAttachmentConfig) []string {
	var out []string
	for _, network := range networks {
		out = append(out, network.Target)
	}
	return out
}

func formatEnv(env []string) map[string]string {
	out := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

func printInfo(w io.Writer, info *appInfo) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	fmt.Fprintf(tw, "Name:\t%s\n", info.Name)
	fmt.Fprintf(tw, "Status:\t%s\n", info.Status)
	if info.Status == "Deployed" {
		fmt.Fprintf(tw, "Image:\t%s\n", info.Image)
		fmt.Fprintf(tw, "Replicas:\t%d\n", info.Replicas)
		printList(tw, "Ports", info.Ports)
		printList(tw, "Networks", info.Networks)
		printMap(tw, "Environment Variables", info.Env)
		printMap(tw, "Labels", info.Labels)
	}
	fmt.Fprintln(tw)
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
	for _, k := range keys {
		fmt.Fprintf(w, "  %s:\t%s\n", k, data[k])
	}
}
