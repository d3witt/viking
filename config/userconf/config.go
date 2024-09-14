package userconf

import (
	"github.com/BurntSushi/toml"
	"github.com/d3witt/viking/config"
)

const (
	VIKING_CONFIG_DIR = "VIKING_CONFIG_DIR"
)

type Config struct {
	Keys    map[string]Key
	Profile Profile
}

func defaultConfig() Config {
	return Config{
		Keys: make(map[string]Key),
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

	return config.WriteConfigFile(filename, data)
}
