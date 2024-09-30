package dockerhelper

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
	return c.cmd.Exit()
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

func DialSSH(sshClient *ssh.Client) (*client.Client, error) {
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		cmd := sshexec.Command(sshClient, "docker", "system", "dial-stdio")
		inWriter, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
		}
		outReader, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start command: %w", err)
		}

		return &cmdConn{
			cmd: cmd,
			in:  outReader,
			out: inWriter,
		}, nil
	}

	httpClient := &http.Client{
		// No tls
		// No proxy
		Transport: &http.Transport{
			DialContext: dialContext,
		},
	}

	var clientOpts []client.Opt

	clientOpts = append(clientOpts,
		client.WithHTTPClient(httpClient),
		client.WithHost("http://docker.example.com"),
		client.WithDialContext(dialContext),
	)

	version := os.Getenv("DOCKER_API_VERSION")

	if version != "" {
		clientOpts = append(clientOpts, client.WithVersion(version))
	} else {
		clientOpts = append(clientOpts, client.WithAPIVersionNegotiation())
	}

	return client.NewClientWithOpts(clientOpts...)
}
