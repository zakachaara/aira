// Package config manages AIRA configuration via Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for AIRA.
type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
	Collect  CollectConfig  `mapstructure:"collect"`
	Digest   DigestConfig   `mapstructure:"digest"`
	Schedule ScheduleConfig `mapstructure:"schedule"`
}

// DatabaseConfig controls persistence settings.
type DatabaseConfig struct {
	Driver string `mapstructure:"driver"` // "sqlite3" | "postgres"
	DSN    string `mapstructure:"dsn"`
}

// LogConfig controls logging behaviour.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Pretty bool   `mapstructure:"pretty"`
}

// CollectConfig controls feed-collection behaviour.
type CollectConfig struct {
	TimeoutSeconds  int `mapstructure:"timeout_seconds"`
	MaxConcurrent   int `mapstructure:"max_concurrent"`
	RetryAttempts   int `mapstructure:"retry_attempts"`
	UserAgent       string `mapstructure:"user_agent"`
}

// DigestConfig controls digest generation.
type DigestConfig struct {
	MaxEntriesPerSection int    `mapstructure:"max_entries_per_section"`
	TrendWindowDays      int    `mapstructure:"trend_window_days"`
	OutputDir            string `mapstructure:"output_dir"`
}

// ScheduleConfig defines cron expressions for automated runs.
type ScheduleConfig struct {
	Collect string `mapstructure:"collect"` // cron expression
	Digest  string `mapstructure:"digest"`
}

// Load reads and returns the configuration.  It searches (in order):
//  1. $AIRA_CONFIG env var
//  2. $HOME/.aira/config.yaml
//  3. /etc/aira/config.yaml
//  4. ./config.yaml
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("database.driver", "sqlite3")
	v.SetDefault("database.dsn", defaultDBPath())
	v.SetDefault("log.level", "info")
	v.SetDefault("log.pretty", true)
	v.SetDefault("collect.timeout_seconds", 30)
	v.SetDefault("collect.max_concurrent", 5)
	v.SetDefault("collect.retry_attempts", 2)
	v.SetDefault("collect.user_agent", "AIRA/1.0 (+https://github.com/aira/aira)")
	v.SetDefault("digest.max_entries_per_section", 10)
	v.SetDefault("digest.trend_window_days", 7)
	v.SetDefault("digest.output_dir", defaultOutputDir())
	v.SetDefault("schedule.collect", "0 */4 * * *")  // every 4 hours
	v.SetDefault("schedule.digest", "0 8 * * *")     // daily at 08:00

	// Environment overrides
	v.SetEnvPrefix("AIRA")
	v.AutomaticEnv()

	// Config file
	if cfgFile := os.Getenv("AIRA_CONFIG"); cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		home, _ := os.UserHomeDir()
		v.AddConfigPath(filepath.Join(home, ".aira"))
		v.AddConfigPath("/etc/aira")
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		// No config file is fine – we use defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	return &cfg, nil
}

// WriteDefault writes a starter config file to $HOME/.aira/config.yaml.
func WriteDefault(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultYAML()), 0o644)
}

func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aira", "aira.db")
}

func defaultOutputDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aira", "digests")
}

func defaultYAML() string {
	return `# AIRA Configuration
database:
  driver: sqlite3
  dsn: ~/.aira/aira.db

log:
  level: info
  pretty: true

collect:
  timeout_seconds: 30
  max_concurrent: 5
  retry_attempts: 2
  user_agent: "AIRA/1.0 (+https://github.com/zakachaara/aira)"

digest:
  max_entries_per_section: 10
  trend_window_days: 7
  output_dir: ~/.aira/digests

schedule:
  collect: "0 */4 * * *"
  digest:  "0 8 * * *"
`
}
