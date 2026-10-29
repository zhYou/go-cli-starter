package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigurationPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("log_level: info\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, env, flag, want string
	}{
		{"默认值", "", "", "", "warn"},
		{"文件", path, "", "", "info"},
		{"环境变量", path, "error", "", "error"},
		{"显式选项", path, "error", "debug", "debug"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var override *string
			if tc.flag != "" {
				override = &tc.flag
			}
			cfg, err := Load(Options{AppName: "my-cli", Path: tc.path, UserConfigDir: dir, LogLevel: override,
				LookupEnv: func(key string) (string, bool) { return tc.env, key == "MY_CLI_LOG_LEVEL" && tc.env != "" },
			})
			if err != nil || cfg.LogLevel != tc.want {
				t.Fatalf("cfg=%+v err=%v", cfg, err)
			}
		})
	}
}

func TestInvalidConfigurationFails(t *testing.T) {
	for _, content := range []string{"[broken", "unknown: value\n", "log_level: noisy\n", "log_level: [debug]\n", "log_level: warn\n---\nlog_level: error\n"} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(Options{AppName: "testcli", Path: path}); err == nil {
			t.Errorf("错误地接受了无效配置 %q", content)
		}
	}
	if _, err := Load(Options{AppName: "testcli", Path: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("错误地接受了不存在的显式配置文件")
	}
}

func TestLoadsDefaultUserConfiguration(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "mycli"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mycli", "config.yaml"), []byte("log_level: debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(Options{AppName: "mycli", UserConfigDir: dir, LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil || cfg.LogLevel != "debug" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}
