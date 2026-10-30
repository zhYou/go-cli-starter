// Package config 解析应用配置，不使用全局状态。
package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/zhYou/go-cli-starter/internal/base/validation"
	"go.yaml.in/yaml/v3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	LogLevel string `yaml:"log_level" validate:"oneof=debug info warn error" label:"日志级别"`
}
type Options struct {
	AppName, Path, UserConfigDir string
	LookupEnv                    func(string) (string, bool)
	LogLevel                     *string
	Context                      context.Context
	Validator                    *validation.Validator
}

// Load 按显式选项 > 非空环境变量 > YAML > 默认值解析。
func Load(opts Options) (Config, error) {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := Config{LogLevel: "warn"}
	if err := ctx.Err(); err != nil {
		return cfg, err
	}
	k := koanf.New(".")
	layer := func(values map[string]any) error { return k.Load(confmap.Provider(values, "."), nil) }
	if err := layer(map[string]any{"log_level": "warn"}); err != nil {
		return cfg, err
	}
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
		values, err := readYAML(file)
		if err != nil {
			return cfg, fmt.Errorf("解析配置文件 %q 失败：%w", path, err)
		}
		if err := layer(values); err != nil {
			return cfg, err
		}
	}
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	prefix := strings.ToUpper(strings.ReplaceAll(opts.AppName, "-", "_"))
	if value, ok := lookup(prefix + "_LOG_LEVEL"); ok && value != "" {
		if err := layer(map[string]any{"log_level": value}); err != nil {
			return cfg, err
		}
	}
	if opts.LogLevel != nil {
		if err := layer(map[string]any{"log_level": *opts.LogLevel}); err != nil {
			return cfg, err
		}
	}
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "yaml", DecoderConfig: &mapstructure.DecoderConfig{WeaklyTypedInput: false, ErrorUnused: true}}); err != nil {
		return cfg, fmt.Errorf("解码配置失败：%w", err)
	}
	cfg.LogLevel = strings.ToLower(cfg.LogLevel)
	v := opts.Validator
	if v == nil {
		v = validation.New()
	}
	if err := v.Struct(ctx, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// readYAML 先严格验证结构，再返回实际存在的非 null 值。
func readYAML(input io.Reader) (map[string]any, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var typed Config
	if err := decoder.Decode(&typed); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("必须只包含一个 YAML 文档")
	}
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	for key, value := range values {
		if value == nil {
			delete(values, key)
			continue
		}
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("配置项 %s 必须为字符串", key)
		}
	}
	return values, nil
}
