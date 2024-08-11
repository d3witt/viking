package streams

import "golang.org/x/term"

type stream struct {
	fd    int
	state *term.State
}

func (s *stream) IsTerminal() bool {
	return term.IsTerminal(s.fd)
}

func (s *stream) MakeRaw() error {
	state, err := term.MakeRaw(s.fd)
	if err != nil {
		return err
	}

	s.state = state
	return nil
}

func (s *stream) Restore() error {
	if s.state == nil {
		return nil
	}

	err := term.Restore(s.fd, s.state)
	s.state = nil
	return err
}

func (s *stream) Size() (width int, height int, err error) {
	return term.GetSize(s.fd)
}
