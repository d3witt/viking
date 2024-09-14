package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func ConfigDir(dir, configDirEnv string) (string, error) {
	var path string
	if a := os.Getenv(configDirEnv); a != "" {
		path = a
	} else {
		b, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("failed to retrieve config dir path: %w", err)
		}

		path = filepath.Join(b, dir)
	}

	if !dirExists(path) {
		if err := os.MkdirAll(path, 0o755); err != nil {
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

func ReadConfigFile(filename string) ([]byte, error) {
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

func WriteConfigFile(filename string, data []byte) error {
	err := os.MkdirAll(filepath.Dir(filename), 0o771)
	if err != nil {
		return pathError(err)
	}

	cfgFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600) // cargo coded from setup
	if err != nil {
		return err
	}
	defer cfgFile.Close()

	_, err = cfgFile.Write(data)
	return err
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
