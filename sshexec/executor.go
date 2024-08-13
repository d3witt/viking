package sshexec

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"golang.org/x/crypto/ssh"
)

type Executor interface {
	Start(cmd string, in io.Reader, out, stderr io.Writer) error
	StartInteractive(cmd string, in io.Reader, out, stderr io.Writer, w, h int) error
	Wait() error
	Close() error
	Addr() string
	SetLogger(logger *slog.Logger)
}

// executor allows for the execution of multiple commands, but only one at a time. It is not safe for concurrent use.
type executor struct {
	host   string
	client func() (*ssh.Client, error)
	logger *slog.Logger

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
	return e.startSession(cmd, in, out, stderr, nil)
}

func (e *executor) StartInteractive(cmd string, in io.Reader, out, stderr io.Writer, w, h int) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	return e.startSession(cmd, in, out, stderr, &ptyOptions{h, w, modes})
}

type ptyOptions struct {
	h, w  int
	modes ssh.TerminalModes
}

func (e *executor) startSession(cmd string, in io.Reader, out, stderr io.Writer, pty *ptyOptions) error {
	if e.session != nil {
		return errors.New("another command is currently running")
	}

	client, err := e.client()
	if err != nil {
		return err
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}

	session.Stdin = in
	session.Stdout = out
	session.Stderr = stderr

	if pty != nil {
		if err := session.RequestPty("xterm-256color", pty.h, pty.w, pty.modes); err != nil {
			_ = session.Close()
			return err
		}
	}

	e.session = session

	if e.logger != nil {
		e.logger.Info("starting command", "host", e.host, "cmd", cmd)
	}

	if err := session.Start(cmd); err != nil {
		_ = e.closeSession()
		return fmt.Errorf("failed to start ssh session: %w", err)
	}

	return nil
}

func (e *executor) Wait() error {
	if e.session == nil {
		return errors.New("failed to wait command: command not started")
	}
	defer e.closeSession()

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

func (e *executor) SetLogger(logger *slog.Logger) {
	e.logger = logger
}

func (e *executor) Close() error {
	if err := e.closeSession(); err != nil {
		return err
	}

	client, err := e.client()
	if err != nil {
		return err
	}

	return client.Close()
}

func (e *executor) closeSession() error {
	if e.session != nil {
		if err := e.session.Close(); err != nil {
			if err != io.EOF {
				if e.logger != nil {
					e.logger.Error("failed to close SSH session", "host", e.host, "err", err)
				}

				return err
			}
		}

		e.session = nil
	}

	return nil
}
