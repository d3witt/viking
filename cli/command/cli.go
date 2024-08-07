package command

import (
	"github.com/d3witt/viking/config"
	"github.com/d3witt/viking/streams"
)

type Cli struct {
	Config   *config.Config
	Out, Err *streams.Out
	In       *streams.In
}
