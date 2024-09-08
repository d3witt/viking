package command

import (
	"fmt"
	"sync"

	"github.com/d3witt/viking/config"
	"github.com/d3witt/viking/sshexec"
	"golang.org/x/crypto/ssh"
)

// DialMachine dials all hosts of a machine and returns a slice of SSH clients
// for available hosts. If there are no available hosts, DialMachine returns
// an error.
func (c *Cli) DialMachine(machine string) ([]*ssh.Client, error) {
	m, err := c.Config.GetMachineByName(machine)
	if err != nil {
		return nil, err
	}

	var (
		clients = make([]*ssh.Client, 0, len(m.Hosts))
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for _, host := range m.Hosts {
		wg.Add(1)
		go func(h config.Host) {
			defer wg.Done()
			client, dialErr := c.DialHost(h)

			if dialErr != nil {
				fmt.Fprintf(c.Out, "Failed to dial %s: %v\n", h.IP, dialErr)
				return
			}

			mu.Lock()
			clients = append(clients, client)
			mu.Unlock()
		}(host)
	}

	wg.Wait()

	if len(clients) == 0 {
		return nil, fmt.Errorf("no hosts available for machine %s", machine)
	}

	return clients, nil
}

func (c *Cli) DialHost(host config.Host) (*ssh.Client, error) {
	if host.Key == "" {
		return sshexec.SSHClient(host.IP.String(), host.Port, host.User, "", "")
	}

	key, err := c.Config.GetKeyByName(host.Key)
	if err != nil {
		return nil, err
	}

	return sshexec.SSHClient(host.IP.String(), host.Port, host.User, key.Private, key.Passphrase)
}
