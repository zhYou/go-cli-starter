// Package paths 只解析应用私有目录，不创建文件或修改环境。
package paths

import (
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type Directories struct{ Config, Data, State, Cache string }
type Options struct {
	AppName, GOOS string
	LookupEnv     func(string) (string, bool)
}

func EnvPrefix(app string) string { return strings.ToUpper(strings.ReplaceAll(app, "-", "_")) }
func Resolve(opts Options) (Directories, error) {
	platform := opts.GOOS
	if platform == "" {
		platform = runtime.GOOS
	}
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	env := func(k string) string {
		v, ok := lookup(k)
		if !ok {
			return ""
		}
		return v
	}
	join := func(parts ...string) string { return path.Join(parts...) }
	absolute := func(s string) bool { return strings.HasPrefix(s, "/") }
	if platform == "windows" {
		join = func(parts ...string) string {
			for i := range parts {
				parts[i] = strings.ReplaceAll(parts[i], string(rune(92)), "/")
			}
			joined := path.Join(parts...)
			if strings.HasPrefix(parts[0], "//") {
				joined = "/" + joined
			}
			return strings.ReplaceAll(joined, "/", string(rune(92)))
		}
		absolute = func(s string) bool {
			normalized := strings.ReplaceAll(s, string(rune(92)), "/")
			if len(normalized) > 2 && normalized[1] == ':' && normalized[2] == '/' {
				return normalized[0] >= 'A' && normalized[0] <= 'Z' || normalized[0] >= 'a' && normalized[0] <= 'z'
			}
			if strings.HasPrefix(normalized, "//") {
				parts := strings.Split(strings.TrimPrefix(normalized, "//"), "/")
				return len(parts) >= 2 && parts[0] != "" && parts[1] != ""
			}
			return false
		}
	}
	fail := func(field string) (Directories, error) {
		return Directories{}, (fault.Definition{Code: "paths.absolute", Kind: fault.Configuration}).New(fault.WithParams(fault.Params{"Field": field}))
	}
	if opts.AppName == "" || opts.AppName == "." || opts.AppName == ".." || strings.ContainsAny(opts.AppName, "/"+string(rune(92))) {
		return fail("AppName")
	}
	homeKey := EnvPrefix(opts.AppName) + "_HOME"
	if home := env(homeKey); home != "" {
		if !absolute(home) {
			return fail(homeKey)
		}
		home = join(home)
		return Directories{home, join(home, "data"), join(home, "state"), join(home, "cache")}, nil
	}
	if platform == "windows" {
		roaming, local := env("APPDATA"), env("LOCALAPPDATA")
		if !absolute(roaming) {
			return fail("APPDATA")
		}
		if !absolute(local) {
			return fail("LOCALAPPDATA")
		}
		return Directories{join(roaming, opts.AppName), join(local, opts.AppName, "data"), join(local, opts.AppName, "state"), join(local, opts.AppName, "cache")}, nil
	}
	home := env("HOME")
	if platform == "darwin" {
		if !absolute(home) {
			return fail("HOME")
		}
		support := join(home, "Library", "Application Support", opts.AppName)
		return Directories{support, join(support, "data"), join(support, "state"), join(home, "Library", "Caches", opts.AppName)}, nil
	}
	roots := []struct{ env, suffix string }{{"XDG_CONFIG_HOME", ".config"}, {"XDG_DATA_HOME", ".local/share"}, {"XDG_STATE_HOME", ".local/state"}, {"XDG_CACHE_HOME", ".cache"}}
	values := make([]string, 4)
	for i, root := range roots {
		value := env(root.env)
		// XDG 规范要求忽略相对路径，回退到 HOME。
		if !absolute(value) {
			if !absolute(home) {
				return fail("HOME")
			}
			value = join(home, root.suffix)
		}
		values[i] = join(value, opts.AppName)
	}
	return Directories{values[0], values[1], values[2], values[3]}, nil
}
func (d Directories) ConfigFile() string { return filepath.Join(d.Config, "config.toml") }
