package command

import (
	"errors"
	"io"

	"github.com/d3witt/viking/config"
	"github.com/mattn/go-isatty"
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
	if !isatty.IsTerminal(uintptr(c.InFd)) {
		return 0, 0, errors.New("not a terminal")
	}

	return term.GetSize(c.InFd)
}
