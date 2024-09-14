package appconf

import (
	"bytes"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Name     string             `toml:"name"`
	Machines map[string]Machine `toml:"machines"`
}

func defaultConfig(name string) Config {
	return Config{
		Name:     name,
		Machines: map[string]Machine{},
	}
}

func (c Config) Save() error {
	data, err := marshal(&c)
	if err != nil {
		return err
	}

	return writeConfigFile(configFile, data)
}

func marshal(v any) ([]byte, error) {
	buff := new(bytes.Buffer)
	encoder := toml.NewEncoder(buff)
	encoder.Indent = ""

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}
