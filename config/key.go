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

func (c *Config) ListKeys() []Key {
	keys := make([]Key, 0, len(c.Keys))

	for name, key := range c.Keys {
		key.Name = name
		keys = append(keys, key)
	}

	return keys
}

func (c *Config) AddKey(key Key) error {
	if _, ok := c.Keys[key.Name]; ok {
		return fmt.Errorf("Key already exists: %s", key.Name)
	}

	c.Keys[key.Name] = key

	return c.Save()
}

func (c *Config) RemoveKey(name string) error {
	if _, ok := c.Keys[name]; !ok {
		return fmt.Errorf("Key not found: %s", name)
	}

	delete(c.Keys, name)

	return c.Save()
}

func (c *Config) GetKeyByName(name string) (Key, error) {
	if key, ok := c.Keys[name]; ok {
		key.Name = name
		return key, nil
	}

	return Key{}, fmt.Errorf("Key not found: %s", name)
}