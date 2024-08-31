package sshexec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

type PtyOptions struct {
	h, w int
}

type ExitError struct {
	Content string
	Status  int
}

func (e ExitError) Error() string {
	if e.Content != "" {
		return e.Content
	}

	return fmt.Sprintf("exited with status %v", e.Status)
}

type Cmd struct {
	client  *ssh.Client
	session *ssh.Session
	logger  *slog.Logger

	Name           string
	Args           []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer

	pty *PtyOptions
}

func Command(client *ssh.Client, name string, args ...string) *Cmd {
	return &Cmd{
		client: client,
		Name:   name,
		Args:   args,
	}
}

func (c *Cmd) Start() error {
	if c.session != nil {
		return errors.New("command already started")
	}

	if c.client == nil {
		return errors.New("client not set")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer func() {
		if err != nil {
			_ = session.Close()
		}
	}()

	session.Stdin = c.Stdin
	session.Stdout = c.Stdout
	session.Stderr = c.Stderr

	if c.pty != nil {
		modes := ssh.TerminalModes{
			ssh.ECHO:          1,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}
		if err := session.RequestPty("xterm-256color", c.pty.h, c.pty.w, modes); err != nil {
			return err
		}
	}

	c.session = session

	if c.logger != nil {
		c.logger.Info("starting command", "addr", c.client.RemoteAddr(), "cmd", c.argv())
	}

	if err := session.Start(c.argv()); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	return nil
}

func (c *Cmd) Wait() error {
	if c.session == nil {
		return errors.New("failed to wait command: command not started")
	}
	defer func() {
		c.session.Close()
		c.session = nil
	}()

	if err := c.session.Wait(); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return &ExitError{
				Status:  exitErr.ExitStatus(),
				Content: exitErr.String(),
			}
		}

		return fmt.Errorf("failed to wait command: %w", err)
	}

	return nil
}

func (c *Cmd) Close() error {
	if c.session == nil {
		return nil
	}

	err := c.session.Close()
	c.session = nil
	return err
}

func (c *Cmd) Run() error {
	var b bytes.Buffer

	if c.Stderr == nil {
		c.Stderr = &b
	}

	if err := c.Start(); err != nil {
		return err
	}

	if err := c.Wait(); err != nil {
		return fmt.Errorf("%w.\n%s", err, b.String())
	}

	return nil
}

func (c *Cmd) Output() (string, error) {
	if c.Stdout != nil {
		return "", errors.New("stdout already set")
	}

	var b bytes.Buffer
	c.Stdout = &b
	err := c.Run()
	return b.String(), err
}

type singleWriter struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func (w *singleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (c *Cmd) CombinedOutput() (string, error) {
	if c.Stdout != nil {
		return "", errors.New("stdout already set")
	}
	if c.Stderr != nil {
		return "", errors.New("stderr already set")
	}

	var b singleWriter
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()

	return b.b.String(), err
}

func (c *Cmd) argv() string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

func (c *Cmd) String() string {
	return c.argv()
}

func (c *Cmd) SetLogger(logger *slog.Logger) {
	c.logger = logger
}

func (c *Cmd) SetPty(h, w int) {
	c.pty = &PtyOptions{h, w}
}
