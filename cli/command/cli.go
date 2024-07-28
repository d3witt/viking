package command

import (
	"io"

	"github.com/d3witt/viking/config"
)

type Cli struct {
	Config   *config.Config
	Out, Err io.Writer
	In       io.ReadCloser
}
