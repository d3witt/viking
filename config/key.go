package config

import (
	"fmt"
	"time"
)

type Key struct {
	Name       string `toml:"-"`
	Private    string
	Public     string
	Passphrase string
	CreatedAt  time.Time
}

var KeyNameRequiredError = fmt.Errorf("key name is required")

func (c *Config) ListKeys() []Key {
	keys := make([]Key, 0, len(c.Keys))

	for name, key := range c.Keys {
		key.Name = name
		keys = append(keys, key)
	}

	return keys
}

func (c *Config) AddKey(key Key) error {
	_, err := c.GetKeyByName(key.Name)
	if err == nil {
		return fmt.Errorf("key already exists: %s", key.Name)
	}

	c.Keys[key.Name] = key

	return c.Save()
}

func (c *Config) RemoveKey(name string) error {
	_, err := c.GetKeyByName(name)
	if err != nil {
		return err
	}

	delete(c.Keys, name)

	return c.Save()
}

func (c *Config) GetKeyByName(name string) (Key, error) {
	if name == "" {
		return Key{}, KeyNameRequiredError
	}

	if key, ok := c.Keys[name]; ok {
		key.Name = name
		return key, nil
	}

	return Key{}, fmt.Errorf("key not found: %s", name)
}
