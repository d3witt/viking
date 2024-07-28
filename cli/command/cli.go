package command

import (
	"io"

	"github.com/workdate-dev/viking/config"
)

type Cli struct {
	Config   *config.Config
	Out, Err io.Writer
	In       io.ReadCloser
}
