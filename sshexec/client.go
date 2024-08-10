package sshexec

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func SshClient(host, user, private, passphrase string) (*ssh.Client, error) {
	var sshAuth ssh.AuthMethod
	var err error

	if private != "" {
		sshAuth, err = authorizeWithKey(private, passphrase)
	} else {
		sshAuth, err = authorizeWithSSHAgent()
	}
	if err != nil {
		return nil, err
	}

	// Set up SSH client configuration
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			sshAuth,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 5,
	}

	return ssh.Dial("tcp", host+":22", config)
}

func authorizeWithKey(key, passphrase string) (ssh.AuthMethod, error) {
	var signer ssh.Signer
	var err error

	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(key))
	}
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(signer), nil
}

func authorizeWithSSHAgent() (ssh.AuthMethod, error) {
	conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ssh-agent: %w", err)
	}
	defer conn.Close()

	sshAgent := agent.NewClient(conn)
	return ssh.PublicKeysCallback(sshAgent.Signers), nil
}
