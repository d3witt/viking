package sshexec

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/ssh"
)

type Executor interface {
	Start(cmd string, in io.Reader, out, stderr io.Writer) error
	StartInteractive(cmd string, in io.Reader, out, stderr io.Writer, h, w int) error
	Wait() error
	Close() error
	Addr() string
}

type executor struct {
	host   string
	client func() (*ssh.Client, error)

	session *ssh.Session
}

func NewExecutor(host, user, private, passphrase string) Executor {
	return &executor{
		host: host,
		client: sync.OnceValues(func() (*ssh.Client, error) {
			client, err := SshClient(host, user, private, passphrase)
			if err != nil {
				return nil, err
			}

			return client, nil
		}),
	}
}

func (e *executor) Addr() string {
	return e.host
}

func (e *executor) Start(cmd string, in io.Reader, out, stderr io.Writer) error {
	client, err := e.client()
	if err != nil {
		return err
	}

	if e.session != nil {
		return errors.New("command already stared")
	}

	session, err := client.NewSession()
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

func (e *executor) StartInteractive(cmd string, in io.Reader, out, stderr io.Writer, h, w int) error {
	client, err := e.client()
	if err != nil {
		return err
	}

	if e.session != nil {
		return errors.New("command already started")
	}

	session, err := client.NewSession()
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

func (e *executor) Wait() error {
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

func (e *executor) Close() error {
	if e.session != nil {
		if err := e.session.Close(); err != nil {
			return err
		}
	}

	client, err := e.client()
	if err != nil {
		return err
	}

	return client.Close()
}
