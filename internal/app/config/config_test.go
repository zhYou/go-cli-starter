package config

import (
	"github.com/stretchr/testify/require"
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

func TestExplicitDefaultOverridesEnvironment(t *testing.T) {
	dir := t.TempDir()
	warn := "warn"
	for _, override := range []*string{nil, &warn} {
		cfg, err := Load(Options{AppName: "my-cli", UserConfigDir: dir, LogLevel: override, LookupEnv: func(string) (string, bool) { return "debug", true }})
		require.NoError(t, err)
		want := "debug"
		if override != nil {
			want = "warn"
		}
		require.Equal(t, want, cfg.LogLevel)
	}
}
func TestStrictYAML(t *testing.T) {
	for _, tc := range []struct {
		text  string
		valid bool
		want  string
	}{
		{"", true, "warn"}, {"null\n", true, "warn"}, {"log_level: null\n", true, "warn"}, {"log_level: INFO\n", true, "info"},
		{"unknown: x", false, ""}, {"log_level: warn\n---\n", false, ""}, {"log_level: warn\nlog_level: error", false, ""}, {"log_level: [debug]", false, ""}, {"log_level: 123", false, ""},
	} {
		t.Run(tc.text, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(p, []byte(tc.text), 0600))
			cfg, err := Load(Options{Path: p, LookupEnv: func(string) (string, bool) { return "", true }})
			if tc.valid {
				require.NoError(t, err)
				require.Equal(t, tc.want, cfg.LogLevel)
			} else {
				require.Error(t, err)
			}
		})
	}
}
