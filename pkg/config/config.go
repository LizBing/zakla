// Package config provides server configuration handling.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config represents the server configuration.
type Config struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	MaxPlayers int   `toml:"max_players"`
	Motd     string `toml:"motd"`

	Network struct {
		CompressionThreshold int `toml:"compression_threshold"`
		MaxConnections      int `toml:"max_connections"`
	} `toml:"network"`

	World struct {
		Name         string `toml:"name"`
		Seed         int64  `toml:"seed"`
		Difficulty   string `toml:"difficulty"`
		GameMode     string `toml:"gamemode"`
		Hardcore     bool   `toml:"hardcore"`
	} `toml:"world"`

	Logging struct {
		Level  string `toml:"level"`
		Output string `toml:"output"`
	} `toml:"logging"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Host:       "0.0.0.0",
		Port:       25565,
		MaxPlayers: 20,
		Motd:       "A Minecraft Server",
	}
}

// Load loads a configuration file.
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to a file.
func (c *Config) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}
