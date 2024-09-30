package command

import (
	"github.com/d3witt/viking/sshexec"
	"golang.org/x/crypto/ssh"
)

func (c *Cli) DialMachine() (*ssh.Client, error) {
	conf, err := c.AppConfig()
	if err != nil {
		return nil, err
	}

	m, err := conf.GetMachine()
	if err != nil {
		return nil, err
	}

	private, passphrase, err := c.GetSSHKeyDetails(m.Key)
	if err != nil {
		return nil, err
	}

	return sshexec.SSHClient(m.IP.String(), m.Port, m.User, private, passphrase)
}

func (c *Cli) GetSSHKeyDetails(key string) (private, passphrase string, err error) {
	if key == "" {
		return "", "", nil
	}

	k, err := c.Config.GetKeyByName(key)
	if err != nil {
		return "", "", err
	}

	return k.Private, k.Passphrase, nil
}
