package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ChunkRules struct {
	Small  int `yaml:"small"`
	Medium int `yaml:"medium"`
	Large  int `yaml:"large"`
	XLarge int `yaml:"xlarge"`
}

type Config struct {
	DownloadDir           string     `yaml:"download_dir"`
	MaxRetries            int        `yaml:"max_retries"`
	Timeout               int        `yaml:"timeout"`
	ChunkRules            ChunkRules `yaml:"chunk_rules"`
	SmallSize             int64      `yaml:"small_size"`
	MediumSize            int64      `yaml:"medium_size"`
	LargeSize             int64      `yaml:"large_size"`
	MaxConcurrent         int        `yaml:"max_concurrent_downloads"`
	GlobalSpeedLimit      int64      `yaml:"global_speed_limit"`       // bytes/sec, 0 = unlimited
	PerDownloadSpeedLimit int64      `yaml:"per_download_speed_limit"` // bytes/sec, 0 = unlimited
}

func Default() *Config {
	return &Config{
		DownloadDir: filepath.Join(os.Getenv("HOME"), "Downloads"),
		MaxRetries:  3,
		Timeout:     30,
		ChunkRules: ChunkRules{
			Small:  4,
			Medium: 8,
			Large:  16,
			XLarge: 32,
		},
		SmallSize:     1 << 30,         // 1 GB
		MediumSize:    5 << 30,         // 5 GB
		LargeSize:     100 * (1 << 30), // 100 GB
		MaxConcurrent: 2,
	}
}

func (c *Config) ChunksForSize(size int64) int {
	if size < c.SmallSize {
		return c.ChunkRules.Small
	}
	if size < c.MediumSize {
		return c.ChunkRules.Medium
	}
	if size < c.LargeSize {
		return c.ChunkRules.Large
	}
	return c.ChunkRules.XLarge
}

// Save menulis config ke ~/.config/lunefetch/config.yaml.
func (c *Config) Save() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "lunefetch")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	configFile := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func Load() (*Config, error) {
	cfg := Default()

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "lunefetch")
	configFile := filepath.Join(configDir, "config.yaml")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return cfg, fmt.Errorf("create config dir: %w", err)
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return cfg, fmt.Errorf("marshal default config: %w", err)
		}
		if err := os.WriteFile(configFile, data, 0644); err != nil {
			return cfg, fmt.Errorf("write default config: %w", err)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}
