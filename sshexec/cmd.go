package sshexec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

type Cmd struct {
	Executor

	Name           string
	Args           []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

func Command(exec Executor, name string, args ...string) *Cmd {
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
		return err
	}

	if err := c.Wait(); err != nil {
		return fmt.Errorf("%w.\n%s", err, b.String())
	}

	return nil
}

func (c *Cmd) RunInteractive(in io.Reader, out, stderr io.Writer, w, h int) error {
	if c.Stderr == nil {
		c.Stderr = stderr
	}

	if err := c.StartInteractive(c.argv(), in, out, stderr, w, h); err != nil {
		return err
	}

	if err := c.Wait(); err != nil {
		return err
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
