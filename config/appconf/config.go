package appconf

import (
	"github.com/BurntSushi/toml"
)

type Config struct {
	Name     string             `toml:"name"`
	Machines map[string]Machine `toml:"machines"`
}

func (c Config) Save() error {
	data, err := toml.Marshal(&c)
	if err != nil {
		return err
	}

	return writeConfigFile(configFile, data)
}
