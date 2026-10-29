package main

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func versionFixture(t *testing.T) string {
	root := t.TempDir()
	for _, name := range []string{"mise.toml", "mise.lock", "go.mod", "README.md", "AGENTS.md", ".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		dest := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0755))
		require.NoError(t, copyFile(filepath.Join("../..", name), dest))
	}
	return root
}
func TestVersionsRejectSubstringAndMissingPlatform(t *testing.T) {
	require.Error(t, matchToolVersion("go", "go version go1.27.30 linux/amd64", "1.27.3"))
	require.NoError(t, matchToolVersion("go", "go version go1.27.3 linux/amd64", "1.27.3"))
	root := versionFixture(t)
	require.NoError(t, checkVersions(root))
	file := filepath.Join(root, "mise.lock")
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	text := string(data)
	a := strings.Index(text, "[tools.go.\"platforms.windows-arm64\"]")
	require.GreaterOrEqual(t, a, 0)
	b := strings.Index(text[a+1:], "\n[")
	require.GreaterOrEqual(t, b, 0)
	text = text[:a] + text[a+1+b:]
	require.NoError(t, os.WriteFile(file, []byte(text), 0600))
	require.Error(t, checkVersions(root))
}
func TestHistoricalVersionTextIsNotCurrentDeclaration(t *testing.T) {
	root := versionFixture(t)
	path := filepath.Join(root, "docs/superpowers/specs/history.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte("Go 1.20.0 mise 2020.1.1"), 0600))
	require.NoError(t, checkVersions(root))
	file := filepath.Join(root, "README.md")
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(file, []byte(strings.Replace(string(data), "| Go | 1.27.3 |", "| Go | 1.27.30 |", 1)), 0600))
	require.Error(t, checkVersions(root))
}
