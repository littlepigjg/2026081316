package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            int           `json:"port"`
	Host            string        `json:"host"`
	HealthInterval  time.Duration `json:"health_interval"`
	RestartDelay    time.Duration `json:"restart_delay"`
	MaxRestartCount int           `json:"max_restart_count"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	LogLevel        string        `json:"log_level"`
	EnableRecovery  bool          `json:"enable_recovery"`
	Version         string        `json:"version"`
}

func Default() *Config {
	return &Config{
		Port:            8080,
		Host:            "0.0.0.0",
		HealthInterval:  10 * time.Second,
		RestartDelay:    2 * time.Second,
		MaxRestartCount: 5,
		ShutdownTimeout: 5 * time.Second,
		LogLevel:        "info",
		EnableRecovery:  true,
		Version:         "1.0.0",
	}
}

func LoadFromEnv() *Config {
	cfg := Default()

	if v := os.Getenv("GUARDIAN_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}

	if v := os.Getenv("GUARDIAN_HOST"); v != "" {
		cfg.Host = v
	}

	if v := os.Getenv("GUARDIAN_HEALTH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HealthInterval = d
		}
	}

	if v := os.Getenv("GUARDIAN_RESTART_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RestartDelay = d
		}
	}

	if v := os.Getenv("GUARDIAN_MAX_RESTART"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxRestartCount = n
		}
	}

	if v := os.Getenv("GUARDIAN_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ShutdownTimeout = d
		}
	}

	if v := os.Getenv("GUARDIAN_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	if v := os.Getenv("GUARDIAN_ENABLE_RECOVERY"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.EnableRecovery = b
		}
	}

	return cfg
}

func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	if c.HealthInterval <= 0 {
		return fmt.Errorf("health interval must be positive")
	}

	if c.RestartDelay < 0 {
		return fmt.Errorf("restart delay must be non-negative")
	}

	if c.MaxRestartCount < 0 {
		return fmt.Errorf("max restart count must be non-negative")
	}

	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}

	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}

	if !validLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}

	return nil
}

func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *Config) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}