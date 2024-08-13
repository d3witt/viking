package command

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
)

type CmdLogHandler struct {
	slog.Handler
	logger *log.Logger
}

func (h *CmdLogHandler) Handle(ctx context.Context, r slog.Record) error {
	var sb strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		sb.WriteString(fmt.Sprintf("%s=\"%s\" ", a.Key, fmt.Sprintf("%v", a.Value.Any())))
		return true
	})

	if sb.Len() > 0 {
		h.logger.Printf("%s: %s", r.Message, sb.String())
	} else {
		h.logger.Println(r.Message)
	}
	return nil
}

func NewCmdLogHandler(
	out io.Writer,
	opts *slog.HandlerOptions,
) *CmdLogHandler {
	h := &CmdLogHandler{
		Handler: slog.NewTextHandler(out, opts),
		logger:  log.New(out, "", 0),
	}

	return h
}
