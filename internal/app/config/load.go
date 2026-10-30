// Package config 合并模块配置，不向业务暴露配置容器。
package config

import (
	"context"
	"errors"
	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/pelletier/go-toml/v2"
	"github.com/zhYou/go-cli-starter/internal/base/diagnostics"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
	"github.com/zhYou/go-cli-starter/internal/base/telemetry"
	"github.com/zhYou/go-cli-starter/internal/base/validation"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Logging     logging.Config     `mapstructure:"logging"`
	Diagnostics diagnostics.Config `mapstructure:"diagnostics"`
	Telemetry   telemetry.Config   `mapstructure:"telemetry"`
}
type Options struct {
	AppName, Path, UserConfigDir string
	LookupEnv                    func(string) (string, bool)
	LogLevel                     *string
	Context                      context.Context
	Validator                    *validation.Validator
	Explicit                     bool
}

func Defaults() Config {
	return Config{Logging: logging.Defaults(), Diagnostics: diagnostics.Defaults(), Telemetry: telemetry.Defaults()}
}

func problem(code string, cause error, params fault.Params) error {
	d := fault.Definition{Code: fault.Code("config." + code), Kind: fault.Configuration}
	if cause != nil {
		return d.Wrap(cause, fault.WithParams(params))
	}
	return d.New(fault.WithParams(params))
}

// Load 按显式选项 > 非空环境变量 > 唯一 TOML 文件 > 默认值解析。
func Load(opts Options) (Config, error) {
	cfg := Defaults()
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return cfg, err
	}
	if opts.AppName == "" {
		opts.AppName = "go-cli-starter"
	}
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if opts.Explicit && opts.Path == "" {
		return cfg, problem("path", nil, nil)
	}
	file := opts.Path
	// 显式 HOME 即使配合 --config 也必须合法，避免后续资源入口解释不一致。
	home, exists := lookup(paths.EnvPrefix(opts.AppName) + "_HOME")
	if !exists {
		home = ""
	}
	if home != "" || file == "" && opts.UserConfigDir == "" {
		dirs, err := paths.Resolve(paths.Options{AppName: opts.AppName, LookupEnv: lookup})
		if err != nil {
			return cfg, err
		}
		if file == "" {
			file = dirs.ConfigFile()
		}
	} else if file == "" {
		file = filepath.Join(opts.UserConfigDir, opts.AppName, "config.toml")
	}
	defaults := map[string]any{
		"logging.level": cfg.Logging.Level, "logging.output": cfg.Logging.Output,
		"diagnostics.max_size_mib": cfg.Diagnostics.MaxSizeMiB, "diagnostics.max_backups": cfg.Diagnostics.MaxBackups,
		"diagnostics.retention_days": cfg.Diagnostics.RetentionDays, "diagnostics.max_total_size_mib": cfg.Diagnostics.MaxTotalSizeMiB,
		"telemetry.exporter": cfg.Telemetry.Exporter, "telemetry.export_timeout": cfg.Telemetry.ExportTimeout,
		"telemetry.shutdown_timeout": cfg.Telemetry.ShutdownTimeout, "telemetry.queue_size": cfg.Telemetry.QueueSize,
		"telemetry.batch_size": cfg.Telemetry.BatchSize, "telemetry.batch_interval": cfg.Telemetry.BatchInterval,
	}
	k := koanf.New(".")
	layer := func(values map[string]any) error { return k.Load(confmap.Provider(values, "."), nil) }
	if err := layer(defaults); err != nil {
		return cfg, problem("decode", err, nil)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		if opts.Explicit || opts.Path != "" || !errors.Is(err, os.ErrNotExist) {
			return cfg, problem("read", err, fault.Params{"Path": file})
		}
	} else {
		var values map[string]any
		if err := toml.Unmarshal(data, &values); err != nil {
			return cfg, problem("parse", err, fault.Params{"Path": file})
		}
		// 文件结构独立验证，覆盖值不能掩盖未知键或弱类型。
		fileConfig := koanf.New(".")
		if err := fileConfig.Load(confmap.Provider(values, "."), nil); err != nil {
			return cfg, problem("decode", err, nil)
		}
		var typed Config
		if err := decode(fileConfig, &typed); err != nil {
			return cfg, problem("decode", err, nil)
		}
		if err := layer(values); err != nil {
			return cfg, problem("decode", err, nil)
		}
	}
	for key, def := range defaults {
		env := paths.EnvPrefix(opts.AppName) + "_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		value, ok := lookup(env)
		if !ok || value == "" {
			continue
		}
		var typed any = value
		if _, ok := def.(int); ok {
			number, err := strconv.Atoi(value)
			if err != nil {
				return cfg, problem("environment", err, fault.Params{"Field": env})
			}
			typed = number
		}
		if err := layer(map[string]any{key: typed}); err != nil {
			return cfg, problem("decode", err, nil)
		}
	}
	if value, ok := lookup(paths.EnvPrefix(opts.AppName) + "_LOG_LEVEL"); ok && value != "" {
		if err := layer(map[string]any{"logging.level": value}); err != nil {
			return cfg, problem("decode", err, nil)
		}
	}
	if opts.LogLevel != nil {
		if err := layer(map[string]any{"logging.level": *opts.LogLevel}); err != nil {
			return cfg, problem("decode", err, nil)
		}
	}
	if err := decode(k, &cfg); err != nil {
		return cfg, problem("decode", err, nil)
	}
	v := opts.Validator
	if v == nil {
		v = validation.New()
	}
	if err := v.Struct(ctx, cfg); err != nil {
		return cfg, problem("invalid", err, nil)
	}
	return cfg, ctx.Err()
}
func decode(k *koanf.Koanf, cfg *Config) error {
	return k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{Tag: "mapstructure", DecoderConfig: &mapstructure.DecoderConfig{
		WeaklyTypedInput: false, ErrorUnused: true, DecodeHook: strictHook, MatchName: func(a, b string) bool { return a == b },
	}})
}
func strictHook(from, to reflect.Type, value any) (any, error) {
	if to == reflect.TypeFor[time.Duration]() {
		switch v := value.(type) {
		case time.Duration:
			return v, nil
		case string:
			return time.ParseDuration(v)
		default:
			return nil, problem("duration", nil, nil)
		}
	}
	if to.Kind() == reflect.Int {
		if from.Kind() != reflect.Int && from.Kind() != reflect.Int64 {
			return nil, problem("integer", nil, nil)
		}
		number := reflect.ValueOf(value).Int()
		if reflect.New(to).Elem().OverflowInt(number) {
			return nil, problem("integer", nil, nil)
		}
	}
	return value, nil
}
