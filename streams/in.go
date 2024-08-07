package streams

import (
	"io"
	"os"
)

type In struct {
	stream
	in io.ReadCloser
}

var StdIn = NewIn(os.Stdin, int(os.Stdin.Fd()))

func NewIn(in io.ReadCloser, fd int) *In {
	i := new(In)
	i.fd = fd
	i.in = in

	return i
}

func (i *In) Read(p []byte) (n int, err error) {
	return i.in.Read(p)
}

func (i *In) Close() error {
	return i.in.Close()
}
