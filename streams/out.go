package streams

import (
	"io"
	"os"
	"sync"
)

type Out struct {
	stream

	out    io.Writer
	outMu  *sync.Mutex
	prefix string
}

func NewOut(out io.Writer) *Out {
	return &Out{
		out:   out,
		outMu: &sync.Mutex{},
	}
}

func (o *Out) Write(p []byte) (n int, err error) {
	o.outMu.Lock()
	defer o.outMu.Unlock()

	if o.prefix != "" {
		prefixedData := append([]byte(o.prefix), p...)
		return o.out.Write(prefixedData)
	}
	return o.out.Write(p)
}

func (o *Out) SetOutput(out io.Writer) {
	o.outMu.Lock()
	defer o.outMu.Unlock()
	o.out = out
}

func (o *Out) WithPrefix(prefix string) *Out {
	return &Out{
		stream: o.stream,
		out:    o.out,
		outMu:  o.outMu,
		prefix: o.prefix + prefix,
	}
}

var (
	// StdOut is the standard output stream.
	StdOut = NewOut(os.Stdout)
	// StdErr is the standard error stream.
	StdErr = NewOut(os.Stderr)
)
