package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type ChunkRules struct {
	Small  int `yaml:"small"`
	Medium int `yaml:"medium"`
	Large  int `yaml:"large"`
	XLarge int `yaml:"xlarge"`
}

type Config struct {
	DownloadDir          string     `yaml:"download_dir"`
	MaxRetries           int        `yaml:"max_retries"`
	Timeout              int        `yaml:"timeout"`
	ChunkRules           ChunkRules `yaml:"chunk_rules"`
	SmallSize            int64      `yaml:"small_size"`
	MediumSize           int64      `yaml:"medium_size"`
	LargeSize            int64      `yaml:"large_size"`
	MaxConcurrent        int        `yaml:"max_concurrent_downloads"`
	GlobalSpeedLimit     int64      `yaml:"global_speed_limit"` // bytes/sec, 0 = unlimited
	ProxyURL             string     `yaml:"proxy_url"`          // "http://host:port", "socks5://host:port", "" = no proxy
	Notifications        bool       `yaml:"notifications"`
	HistoryRetentionDays int        `yaml:"history_retention_days"` // 0 = selamanya

	// AllowLocalHosts mengizinkan download dari LAN, loopback, dan link-local
	// (NAS, server rumah, localhost). Default false supaya URL dari browser
	// tidak bisa dipakai memindai jaringan internal. Endpoint metadata cloud
	// tetap diblokir walau opsi ini aktif.
	AllowLocalHosts bool `yaml:"allow_local_hosts"`

	path string `yaml:"-"`
}

// SetPath menentukan lokasi file yang dipakai Save(). Dipakai di test supaya
// tidak menimpa config asli user.
func (c *Config) SetPath(p string) {
	c.path = p
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
		SmallSize:            1 << 30,         // 1 GB
		MediumSize:           5 << 30,         // 5 GB
		LargeSize:            100 * (1 << 30), // 100 GB
		MaxConcurrent:        2,
		Notifications:        true,
		HistoryRetentionDays: 30,
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

// Validate rejects configuration that could panic, deadlock, or silently
// bypass an explicitly configured proxy.
func (c *Config) Validate() error {
	if c.MaxConcurrent < 1 || c.MaxConcurrent > 64 {
		return fmt.Errorf("max_concurrent_downloads must be between 1 and 64")
	}
	if c.MaxRetries < 0 || c.MaxRetries > 20 {
		return fmt.Errorf("max_retries must be between 0 and 20")
	}
	if c.Timeout < 1 || c.Timeout > 3600 {
		return fmt.Errorf("timeout must be between 1 and 3600 seconds")
	}
	for _, chunks := range []int{c.ChunkRules.Small, c.ChunkRules.Medium, c.ChunkRules.Large, c.ChunkRules.XLarge} {
		if chunks < 1 || chunks > 64 {
			return fmt.Errorf("chunk counts must be between 1 and 64")
		}
	}
	if c.SmallSize <= 0 || c.MediumSize <= c.SmallSize || c.LargeSize <= c.MediumSize {
		return fmt.Errorf("size thresholds must be positive and increasing")
	}
	if c.GlobalSpeedLimit < 0 || c.HistoryRetentionDays < 0 {
		return fmt.Errorf("speed limit and history retention cannot be negative")
	}
	if c.ProxyURL != "" {
		u, err := url.Parse(c.ProxyURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") {
			return fmt.Errorf("proxy_url must be a valid http, https, or socks5 URL")
		}
		if u.Hostname() == "" {
			return fmt.Errorf("proxy_url must include a hostname")
		}
		if port := u.Port(); port != "" {
			n, perr := strconv.Atoi(port)
			if perr != nil || n < 1 || n > 65535 {
				return fmt.Errorf("proxy_url port must be between 1 and 65535")
			}
		}
	}
	return nil
}

// Save menulis config ke file asalnya (default ~/.config/lunefetch/config.yaml).
func (c *Config) Save() error {
	configFile := c.path
	if configFile == "" {
		configFile = DefaultPath()
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configFile), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// DefaultPath mengembalikan lokasi config default.
func DefaultPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "lunefetch", "config.yaml")
}

func Load() (*Config, error) {
	cfg := Default()

	configFile := DefaultPath()
	configDir := filepath.Dir(configFile)
	cfg.path = configFile

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0700); err != nil {
			return cfg, fmt.Errorf("create config dir: %w", err)
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return cfg, fmt.Errorf("marshal default config: %w", err)
		}
		if err := os.WriteFile(configFile, data, 0600); err != nil {
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
	if strings.HasPrefix(cfg.DownloadDir, "~/") {
		cfg.DownloadDir = filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(cfg.DownloadDir, "~/"))
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate config: %w", err)
	}
	if err := os.Chmod(configFile, 0600); err != nil {
		return cfg, fmt.Errorf("secure config permissions: %w", err)
	}

	return cfg, nil
}

// readFile dan unmarshal dipisah supaya bisa dipakai di test tanpa menyentuh
// path config default.
func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

func unmarshal(data []byte, c *Config) error { return yaml.Unmarshal(data, c) }
