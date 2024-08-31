package dockerhelper

import (
	"fmt"

	"github.com/d3witt/viking/sshexec"
	"golang.org/x/crypto/ssh"
)

func IsDockerInstalled(c *ssh.Client) bool {
	return sshexec.Command(c, "docker", "-v").Run() == nil
}

func isSuperUser(c *ssh.Client) bool {
	cmdStr := "id -un | grep -qx 'root' || command -v sudo || command -v su || exit 1"

	return sshexec.Command(c, cmdStr).Run() == nil
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

	setupCmd := `
        sudo systemctl enable docker.service && \
        sudo systemctl enable containerd.service && \
        docker run --privileged --rm tonistiigi/binfmt --install all
    `

	if err := sshexec.Command(c, setupCmd).Run(); err != nil {
		return fmt.Errorf("failed to setup Docker services and binfmt: %w", err)
	}

	return nil
}
