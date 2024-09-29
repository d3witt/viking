package appconf

import (
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Name     string             `toml:"name"`
	Machines map[string]Machine `toml:"machines"`
	Replicas uint64             `toml:"replicas,omitempty"`
	Ports    []string           `toml:"ports,omitempty"`
	Env      map[string]string  `toml:"env,omitempty"`
	Networks []string           `toml:"networks,omitempty"`
	Label    map[string]string  `toml:"label,omitempty"`
}

func defaultConfig(name string) Config {
	return Config{
		Name:     name,
		Machines: map[string]Machine{},
	}
}

func (c Config) Save() error {
	data, err := toml.Marshal(&c)
	if err != nil {
		return err
	}

	return writeConfigFile(configFile, data)
}
