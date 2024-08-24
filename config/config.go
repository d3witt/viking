package config

import (
	"github.com/BurntSushi/toml"
)

type Config struct {
	Keys     map[string]Key
	Machines map[string]Machine
	Profile  Profile
}

func defaultConfig() Config {
	return Config{
		Keys:     make(map[string]Key),
		Machines: make(map[string]Machine),
	}
}

func (c Config) Save() error {
	filename, err := configFile()
	if err != nil {
		return err
	}

	data, err := toml.Marshal(&c)
	if err != nil {
		return err
	}

	return writeConfigFile(filename, data)
}
