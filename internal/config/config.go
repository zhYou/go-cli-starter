// Package config resolves application configuration without global state.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Config holds application settings. Add typed fields here as the CLI grows.
type Config struct {
	LogLevel string `yaml:"log_level"`
}

// Options identifies configuration sources. A nil override means a flag was not set.
type Options struct {
	AppName       string
	Path          string
	UserConfigDir string
	LookupEnv     func(string) (string, bool)
	LogLevel      *string
}

// Load resolves explicit flags > environment > YAML > defaults.
func Load(opts Options) (Config, error) {
	cfg := Config{LogLevel: "warn"}
	path := opts.Path
	if path == "" {
		dir := opts.UserConfigDir
		if dir == "" {
			var err error
			dir, err = os.UserConfigDir()
			if err != nil {
				return cfg, fmt.Errorf("locate user configuration: %w", err)
			}
		}
		path = filepath.Join(dir, opts.AppName, "config.yaml")
	}
	file, err := os.Open(path)
	if err != nil {
		if opts.Path != "" || !errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("read configuration %q: %w", path, err)
		}
	} else {
		defer func() { _ = file.Close() }()
		decoder := yaml.NewDecoder(file)
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
			return cfg, fmt.Errorf("parse configuration %q: %w", path, err)
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			return cfg, fmt.Errorf("configuration %q must contain a single YAML document", path)
		}
	}
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	prefix := strings.ToUpper(strings.ReplaceAll(opts.AppName, "-", "_"))
	if value, ok := lookup(prefix + "_LOG_LEVEL"); ok && value != "" {
		cfg.LogLevel = value
	}
	if opts.LogLevel != nil {
		cfg.LogLevel = *opts.LogLevel
	}
	cfg.LogLevel = strings.ToLower(cfg.LogLevel)
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
		return cfg, nil
	default:
		return cfg, fmt.Errorf("invalid log level %q: use debug, info, warn, or error", cfg.LogLevel)
	}
}
