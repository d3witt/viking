package sshexec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type Cmd struct {
	*Executor

	Name           string
	Args           []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

func Command(exec *Executor, name string, args ...string) *Cmd {
	return &Cmd{
		Executor: exec,
		Name:     name,
		Args:     args,
	}
}

func (c *Cmd) Start() error {
	return c.Executor.Start(c.argv(), c.Stdin, c.Stdout, c.Stderr)
}

func (c *Cmd) Run() error {
	var b bytes.Buffer

	if c.Stderr == nil {
		c.Stderr = &b
	}

	if err := c.Start(); err != nil {
		slog.Error("Failed to start command", "message", b.String(), "cmd", c.argv())
		return err
	}

	if err := c.Wait(); err != nil {
		slog.Error("Failed to run command", "message", b.String(), "cmd", c.argv())
		return err
	}

	return nil
}

func (c *Cmd) Output() (string, error) {
	if c.Stdout != nil {
		return "", errors.New("Stdout already set")
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
		return "", errors.New("Stdout already set")
	}
	if c.Stdin != nil {
		return "", errors.New("Stderr already set")
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

type ExitError struct {
	Content string
	Status  int
}

func (e ExitError) Error() string {
	if e.Content != "" {
		return e.Content
	}

	return fmt.Sprintf("Exited with status %v", e.Status)
}
