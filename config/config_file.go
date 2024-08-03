package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/BurntSushi/toml"
)

const (
	VIKING_CONFIG_DIR = "VIKING_CONFIG_DIR"
)

func ConfigDir() (string, error) {
	var path string
	if a := os.Getenv(VIKING_CONFIG_DIR); a != "" {
		path = a
	} else {
		b, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("failed to retrieve config dir path: %w", err)
		}

		path = filepath.Join(b, "viking")
	}

	if !dirExists(path) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", fmt.Errorf("failed to create config dir: %w", err)
		}

	}

	return path, nil
}

func dirExists(path string) bool {
	f, err := os.Stat(path)
	return err == nil && f.IsDir()
}

func fileExists(path string) bool {
	f, err := os.Stat(path)
	return err == nil && !f.IsDir()
}

func configFile() (string, error) {
	path, err := ConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(path, "viking.toml"), nil
}

func ParseDefaultConfig() (Config, error) {
	path, err := configFile()
	if err != nil {
		return Config{}, err
	}

	return parseConfig(path)
}

func readConfigFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, pathError(err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func writeConfigFile(filename string, data []byte) error {
	err := os.MkdirAll(filepath.Dir(filename), 0771)
	if err != nil {
		return pathError(err)
	}

	cfgFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600) // cargo coded from setup
	if err != nil {
		return err
	}
	defer cfgFile.Close()

	_, err = cfgFile.Write(data)
	return err
}

func parseConfigFile(filename string) (cfg Config, err error) {
	data, err := readConfigFile(filename)
	if err != nil {
		return cfg, err
	}

	_, err = toml.Decode(string(data), &cfg)
	return
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

func pathError(err error) error {
	var pathError *os.PathError
	if errors.As(err, &pathError) && errors.Is(pathError.Err, syscall.ENOTDIR) {
		if p := findRegularFile(pathError.Path); p != "" {
			return fmt.Errorf("remove or rename regular file `%s` (must be a directory)", p)
		}

	}
	return err
}

func findRegularFile(p string) string {
	for {
		if s, err := os.Stat(p); err == nil && s.Mode().IsRegular() {
			return p
		}
		newPath := filepath.Dir(p)
		if newPath == p || newPath == "/" || newPath == "." {
			break
		}
		p = newPath
	}
	return ""
}
