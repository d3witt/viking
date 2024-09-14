package appconf

import (
	"errors"
	"os"
	"path"

	"github.com/BurntSushi/toml"
	"github.com/d3witt/viking/config"
)

var configFile = "./viking.toml"

func writeConfigFile(filename string, data []byte) error {
	cfgFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer cfgFile.Close()

	_, err = cfgFile.Write(data)
	return err
}

func ParseConfig() (Config, error) {
	return parseConfig(configFile)
}

func NewDefaultConfig() (Config, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}

	name := path.Base(currentDir)
	cfg := defaultConfig(name)
	err = cfg.Save()

	return cfg, err
}

func parseConfig(filename string) (Config, error) {
	cfg, err := parseConfigFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, errors.New("config file does not exist, please run 'viking init' first")
		}
	}

	return cfg, err
}

func parseConfigFile(filename string) (cfg Config, err error) {
	data, err := config.ReadConfigFile(filename)
	if err != nil {
		return cfg, err
	}

	_, err = toml.Decode(string(data), &cfg)
	return
}
