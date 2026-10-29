// Package config 解析应用配置，不使用全局状态。
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

// Config 保存应用配置；扩展 CLI 时在此添加有类型的字段。
type Config struct {
	LogLevel string `yaml:"log_level"`
}

// Options 指定配置来源；覆盖值为 nil 表示未设置对应选项。
type Options struct {
	AppName       string
	Path          string
	UserConfigDir string
	LookupEnv     func(string) (string, bool)
	LogLevel      *string
}

// Load 按显式选项 > 环境变量 > YAML > 默认值的优先级解析配置。
func Load(opts Options) (Config, error) {
	cfg := Config{LogLevel: "warn"}
	path := opts.Path
	if path == "" {
		dir := opts.UserConfigDir
		if dir == "" {
			var err error
			dir, err = os.UserConfigDir()
			if err != nil {
				return cfg, fmt.Errorf("定位用户配置目录失败：%w", err)
			}
		}
		path = filepath.Join(dir, opts.AppName, "config.yaml")
	}
	file, err := os.Open(path)
	if err != nil {
		if opts.Path != "" || !errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("读取配置文件 %q 失败：%w", path, err)
		}
	} else {
		defer func() { _ = file.Close() }()
		decoder := yaml.NewDecoder(file)
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
			return cfg, fmt.Errorf("解析配置文件 %q 失败：%w", path, err)
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			return cfg, fmt.Errorf("配置文件 %q 必须只包含一个 YAML 文档", path)
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
		return cfg, fmt.Errorf("日志级别 %q 无效：请选择 debug、info、warn 或 error", cfg.LogLevel)
	}
}
