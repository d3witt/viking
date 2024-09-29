package dockerhelper

import (
	"fmt"
	"strings"

	"github.com/d3witt/viking/sshexec"
	"golang.org/x/crypto/ssh"
)

func IsDockerInstalled(c *ssh.Client) bool {
	cmd := sshexec.Command(c, "docker", "info")
	return cmd.Run() == nil
}

func isSuperUser(c *ssh.Client) bool {
	cmd := sshexec.Command(c, "sudo", "-n", "true")
	return cmd.Run() == nil
}

func InstallDocker(c *ssh.Client) error {
	if !isSuperUser(c) {
		return fmt.Errorf("not a super user")
	}

	installCmd := `
        curl -fsSL https://get.docker.com | sudo sh || \
        wget -qO- https://get.docker.com | sudo sh || \
        exit 1
    `

	if err := sshexec.Command(c, installCmd).Run(); err != nil {
		return fmt.Errorf("failed to install Docker: %w", err)
	}

	if err := configureDockerLogging(c); err != nil {
		return fmt.Errorf("failed to configure Docker logging: %w", err)
	}

	setupCmds := [][]string{
		{"sudo", "systemctl", "enable", "docker.service"},
		{"sudo", "systemctl", "enable", "containerd.service"},
		{"sudo", "systemctl", "restart", "docker.service"},
		{"sudo", "docker", "run", "--privileged", "--rm", "tonistiigi/binfmt", "--install", "all"},
	}

	for _, args := range setupCmds {
		cmd := sshexec.Command(c, args[0], args[1:]...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to execute %v: %w", args, err)
		}
	}

	return nil
}

func configureDockerLogging(c *ssh.Client) error {
	config := `{
	    "log-driver": "local",
	    "log-opts": {
	        "max-size": "100m",
	        "max-file": "3"
	    }
	}`

	cmd := sshexec.Command(c, "sudo", "mkdir", "-p", "/etc/docker")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create docker config directory: %w", err)
	}

	tmpFile := "/tmp/docker-daemon.json"
	cmd = sshexec.Command(c, "sh", "-c", fmt.Sprintf("echo '%s' > %s", escapeSingleQuotes(config), tmpFile))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write temp docker config: %w", err)
	}

	cmd = sshexec.Command(c, "sudo", "mv", tmpFile, "/etc/docker/daemon.json")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to move docker config: %w", err)
	}

	return nil
}

func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}
