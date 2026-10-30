package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecutableAcceptanceVerifiesIdentityAndMetadata(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	dir := t.TempDir()
	executable := filepath.Join(dir, appName)
	if runtime.GOOS == windowsPlatform {
		executable += executableSuffix
	}
	//nolint:gosec // G204：仅构建当前仓库 CLI 到临时目录。
	build := exec.Command("go", "build", "-ldflags", "-X main.version=0.2.0 -X main.commit=acceptance", "-o", executable, "./cmd/"+appName)
	build.Dir = root
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	example := filepath.Join(root, "configs/config.example.toml")
	require.NoError(t, verifyExecutable(executable, dir, appName, example, "0.2.0", "acceptance"))
	require.Error(t, verifyExecutable(executable, dir, "wrong-name", example, "0.2.0", "acceptance"))
	require.Error(t, verifyExecutable(executable, dir, appName, example, "wrong-version", "acceptance"))
	require.Error(t, verifyExecutable(executable, dir, appName, example, "0.2.0", "wrong-commit"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.toml"), []byte("unknown = true"), 0600))
	require.Error(t, verifyExecutable(executable, dir, appName, filepath.Join(dir, "bad.toml"), "0.2.0", "acceptance"))
}

func TestReleaseVerificationRequiresInstalledArtifact(t *testing.T) {
	require.Error(t, verifyRelease(releaseFixture(t)), "仅归档结构通过不能替代真实安装验收")
}
