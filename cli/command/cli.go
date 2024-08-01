package command

import (
	"io"

	"github.com/d3witt/viking/config"
	"golang.org/x/term"
)

type Cli struct {
	Config   *config.Config
	Out, Err io.Writer
	In       io.ReadCloser
	InFd     int
	OutFd    int
}

// TerminalSize returns the width and height of the terminal.
func (c *Cli) TerminalSize() (int, int, error) {
	return term.GetSize(c.InFd)
}
