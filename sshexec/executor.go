package sshexec

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Executor struct {
	client  *ssh.Client
	session *ssh.Session
}

func NewExecutor(client *ssh.Client) *Executor {
	return &Executor{
		client: client,
	}
}

func (e *Executor) Addr() net.Addr {
	return e.client.RemoteAddr()
}

func (e *Executor) Start(cmd string, in io.Reader, out, stderr io.Writer) error {
	if e.session != nil {
		return errors.New("command already stared")
	}
	session, err := e.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	session.Stdin = in
	session.Stdout = out
	session.Stderr = stderr

	e.session = session
	if err := e.session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start SSH session: %w", err)
	}

	return nil
}

func (e *Executor) StartInteractive(cmd string, in io.Reader, out, stderr io.Writer, h, w int) error {
	if e.session != nil {
		return errors.New("command already started")
	}

	session, err := e.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}

	session.Stdin = in
	session.Stdout = out
	session.Stderr = stderr

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", h, w, modes); err != nil {
		return err
	}

	e.session = session
	if err := e.session.Run(cmd); err != nil {
		return fmt.Errorf("failed to start SSH session: %w", err)
	}

	return nil
}

func (e *Executor) Wait() error {
	if e.session == nil {
		return errors.New("failed to wait command: command not started")
	}

	if err := e.session.Wait(); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return &ExitError{
				Status:  exitErr.ExitStatus(),
				Content: exitErr.String(),
			}
		}

		return fmt.Errorf("failed to wait SSH session: %w", err)
	}

	return nil
}

func (e *Executor) Close() error {
	if e.session == nil {
		return errors.New("failed to wait command: command not started")
	}

	return e.Close()
}

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
