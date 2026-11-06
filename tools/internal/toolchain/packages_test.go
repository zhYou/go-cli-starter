package toolchain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstalledPackageVersions(t *testing.T) {
	for _, layout := range []struct{ key, relative, data, version string }{
		{"npm:markdownlint-cli2", "node_modules/markdownlint-cli2/package.json", `{"name":"markdownlint-cli2","version":"0.24.0"}`, "0.24.0"},
		{"npm:@prettier/plugin-xml", "node_modules/@prettier/plugin-xml/package.json", `{"name":"@prettier/plugin-xml","version":"3.4.2"}`, "3.4.2"},
		{"npm:prettier-plugin-toml", "lib/node_modules/prettier-plugin-toml/package.json", `{"name":"prettier-plugin-toml","version":"4.1.0"}`, "4.1.0"},
		{"pypi:pre-commit-hooks", "pre-commit-hooks/lib/python3.15/site-packages/pre_commit_hooks-6.0.0.dist-info/METADATA", "Name: pre_commit_hooks\nVersion: 6.0.0\n", "6.0.0"},
		{"pypi:pre-commit-hooks", "lib/python3.15/site-packages/pre_commit_hooks-6.0.0.dist-info/METADATA", "Name: pre_commit_hooks\nVersion: 6.0.0\n", "6.0.0"},
		{"pypi:pre-commit-hooks", "Lib/site-packages/pre_commit_hooks-6.0.0.dist-info/METADATA", "Name: pre-commit-hooks\nVersion: 6.0.0\n", "6.0.0"},
	} {
		t.Run(layout.relative, func(t *testing.T) {
			prefix := filepath.Join(t.TempDir(), "tool with spaces")
			file := filepath.Join(prefix, filepath.FromSlash(layout.relative))
			require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o700))
			require.NoError(t, os.WriteFile(file, []byte(layout.data), 0o600))
			require.NoError(t, Match(layout.key, prefix, layout.version))
			require.Error(t, Match(layout.key, prefix, "99.99.99"))
			require.NoError(t, os.WriteFile(file, []byte("invalid metadata"), 0o600))
			require.Error(t, Match(layout.key, prefix, layout.version))
			require.NoError(t, os.Remove(file))
			require.Error(t, Match(layout.key, prefix, layout.version))
		})
	}
}

func TestInstalledPackageRejectsWrongName(t *testing.T) {
	prefix := t.TempDir()
	file := filepath.Join(prefix, "node_modules/@prettier/plugin-xml/package.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o700))
	require.NoError(t, os.WriteFile(file, []byte(`{"name":"other","version":"3.4.2"}`), 0o600))
	require.Error(t, Match("npm:@prettier/plugin-xml", prefix, "3.4.2"))
	require.Error(t, Match("npm:@prettier/plugin-xml", "relative/path", "3.4.2"))
}
