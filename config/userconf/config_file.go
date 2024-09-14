package userconf

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/d3witt/viking/config"
)

func ConfigDir() (string, error) {
	return config.ConfigDir("viking", VIKING_CONFIG_DIR)
}

func configFile() (string, error) {
	path, err := ConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(path, defaultProfileName+".toml"), nil
}

func ParseDefaultConfig() (Config, error) {
	path, err := configFile()
	if err != nil {
		return Config{}, err
	}

	return parseConfig(path)
}

func parseConfig(filename string) (Config, error) {
	cfg, err := parseConfigFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
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
