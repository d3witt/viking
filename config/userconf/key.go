package userconf

import (
	"errors"
	"time"
)

type Key struct {
	Name       string `toml:"-"`
	Private    string
	Public     string
	Passphrase string
	CreatedAt  time.Time
}

var (
	ErrKeyNameRequired = errors.New("key name is required")
	ErrKeyNotFound     = errors.New("key not found")
	ErrKeyExist        = errors.New("key already exists")
)

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
		return ErrKeyExist
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
		return Key{}, ErrKeyNameRequired
	}

	if key, ok := c.Keys[name]; ok {
		key.Name = name
		return key, nil
	}

	return Key{}, ErrKeyNotFound
}
