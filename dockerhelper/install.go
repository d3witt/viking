package dockerhelper

import (
	"fmt"

	"github.com/d3witt/viking/sshexec"
)

func isDockerInstalled(e *sshexec.Executor) bool {
	return sshexec.Command(e, "docker", "-v").Run() == nil
}

func isSuperUser(e *sshexec.Executor) bool {
	cmdStr := "id -un | grep -qx 'root' || command -v sudo || command -v su || exit 1"

	return sshexec.Command(e, cmdStr).Run() == nil
}

func installDocker(e *sshexec.Executor) error {
	installCmd := `
        curl -fsSL https://get.docker.com | sudo sh || \
        wget -qO- https://get.docker.com | sudo sh || \
        exit 1
    `

	if err := sshexec.Command(e, installCmd).Run(); err != nil {
		return fmt.Errorf("failed to install Docker: %w", err)
	}

	setupCmd := `
        sudo systemctl enable docker.service && \
        sudo systemctl enable containerd.service && \
        docker run --privileged --rm tonistiigi/binfmt --install all
    `

	if err := sshexec.Command(e, setupCmd).Run(); err != nil {
		return fmt.Errorf("failed to setup Docker services and binfmt: %w", err)
	}

	return nil
}

func InstallDockerIfMissing(e *sshexec.Executor) error {
	if !isDockerInstalled(e) {
		if err := installDocker(e); err != nil {
			return err
		}
	}

	return nil
}
