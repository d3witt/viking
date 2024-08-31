package dockerhelper

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/d3witt/viking/sshexec"
	"github.com/docker/docker/client"
	"golang.org/x/crypto/ssh"
)

// cmdConn implements net.Conn interface over ssh command
type cmdConn struct {
	cmd *sshexec.Cmd
	in  io.Reader
	out io.Writer
}

func (c *cmdConn) Read(p []byte) (n int, err error) {
	return c.in.Read(p)
}

func (c *cmdConn) Write(p []byte) (n int, err error) {
	return c.out.Write(p)
}

func (c *cmdConn) Close() error {
	return c.cmd.Close()
}

func (c *cmdConn) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   nil,
		Zone: "dummy",
	}
}

func (c *cmdConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   nil,
		Zone: "dummy",
	}
}

func (c *cmdConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *cmdConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *cmdConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type Client struct {
	*client.Client
	SSH *ssh.Client
}

func DialSSH(c *ssh.Client) (*Client, error) {
	var clientOpts []client.Opt

	inReader, inWriter := io.Pipe()   // TODO: Close when cmdConn is closed
	outReader, outWriter := io.Pipe() // TODO: Close when cmdConn is closed

	cmd := sshexec.Command(c, "docker system dial-stdio")
	cmd.Stdin = inReader
	cmd.Stdout = outWriter

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	clientOpts = append(clientOpts,
		client.WithAPIVersionNegotiation(),
		client.WithDialContext(func(ctx context.Context, network, addr string) (net.Conn, error) {
			return &cmdConn{
				cmd: cmd,
				in:  outReader,
				out: inWriter,
			}, nil
		}),
		client.WithTimeout(10*time.Second),
	)

	dockerClient, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	_, err = dockerClient.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to ping Docker daemon: %w", err)
	}

	return &Client{
		Client: dockerClient,
		SSH:    c,
	}, nil
}
