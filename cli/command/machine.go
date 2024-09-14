package command

import (
	"errors"
	"fmt"
	"sync"

	"github.com/d3witt/viking/config/appconf"
	"github.com/d3witt/viking/sshexec"
	"golang.org/x/crypto/ssh"
)

// DialMachines dials all hosts of a machine and returns a slice of SSH clients
// for available hosts. If there are no available hosts, DialMachine returns
// an error.
func (c *Cli) DialMachines() ([]*ssh.Client, error) {
	conf, err := c.AppConfig()
	if err != nil {
		return nil, err
	}

	machines := conf.ListMachines()

	var (
		clients = make([]*ssh.Client, 0, len(machines))
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for _, machine := range machines {
		wg.Add(1)
		go func(m appconf.Machine) {
			defer wg.Done()

			private, passphrase, err := c.getSSHKeyDetails(m.Key)
			if err != nil {
				fmt.Fprint(c.Out, err.Error())
				return
			}

			client, dialErr := sshexec.SSHClient(m.IP.String(), m.Port, m.User, private, passphrase)
			if dialErr != nil {
				fmt.Fprintf(c.Out, "Failed to dial %s: %v\n", m.IP, dialErr)
				return
			}

			mu.Lock()
			clients = append(clients, client)
			mu.Unlock()
		}(machine)
	}

	wg.Wait()

	if len(clients) == 0 {
		return nil, errors.New("no available hosts")
	}

	return clients, nil
}

func (c *Cli) DialMachine(machine string) (*ssh.Client, error) {
	conf, err := c.AppConfig()
	if err != nil {
		return nil, err
	}

	m, err := conf.GetMachine(machine)
	if err != nil {
		return nil, err
	}

	private, passphrase, err := c.getSSHKeyDetails(m.Key)
	if err != nil {
		return nil, err
	}

	return sshexec.SSHClient(m.IP.String(), m.Port, m.User, private, passphrase)
}

func (c *Cli) getSSHKeyDetails(key string) (private, passphrase string, err error) {
	if key == "" {
		return "", "", nil
	}

	k, err := c.Config.GetKeyByName(key)
	if err != nil {
		return "", "", err
	}

	return k.Private, k.Passphrase, nil
}
