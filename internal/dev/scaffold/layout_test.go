package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitializeMovesCommandDirectory(t *testing.T) {
	root := projectFixture(t, TemplateModule, TemplateName)
	source := `package main
import _ "` + TemplateModule + `/internal/app/cli"
const appName = "` + TemplateName + `"
func main() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", TemplateName, "main.go"), []byte(source), 0640))
	changed, err := Initialize(root, "example.com/team/repository", "my-cli")
	require.NoError(t, err)
	require.NotEmpty(t, changed)
	project, err := Inspect(root)
	require.NoError(t, err)
	require.Equal(t, Project{"example.com/team/repository", "my-cli", "cmd/my-cli"}, project)
	require.NoDirExists(t, filepath.Join(root, "cmd", TemplateName))
	data, err := os.ReadFile(filepath.Join(root, "cmd/my-cli/main.go"))
	require.NoError(t, err)
	require.Contains(t, string(data), "example.com/team/repository/internal/app/cli")
	changed, err = Initialize(root, "example.com/team/repository", "my-cli")
	require.NoError(t, err)
	require.Empty(t, changed)
}

func TestInitializeSameNameDoesNotMove(t *testing.T) {
	root := projectFixture(t, TemplateModule, TemplateName)
	_, err := Initialize(root, "example.com/team/repository", TemplateName)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, "cmd", TemplateName, "main.go"))
	changed, err := Initialize(root, "example.com/team/repository", TemplateName)
	require.NoError(t, err)
	require.Empty(t, changed)
}

func TestOriginUsesCommandEntry(t *testing.T) {
	root := projectFixture(t, TemplateModule, TemplateName)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".template-version"), []byte("0.1.0\n"), 0600))
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		return string(out)
	}
	git("-c", "init.templateDir=", "init", "--quiet")
	git("remote", "add", "origin", "https://"+TemplateModule+".git")
	git("add", ".")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "commit", "--quiet", "-m", "fixture")
	origin, err := makeOrigin(root, "example.com/tool", "tool")
	require.NoError(t, err)
	require.NotEqual(t, "unknown", origin.SourceCommit)
	git("remote", "set-url", "origin", "https://example.com/copied.git")
	origin, err = makeOrigin(root, "example.com/tool", "tool")
	require.NoError(t, err)
	require.Equal(t, "unknown", origin.SourceCommit)
}

func TestInitializeRejectsGoVersionNamesBeforeWriting(t *testing.T) {
	for _, name := range []string{"v2", "v10", "v999999999999999999999999999999999999999"} {
		t.Run(name, func(t *testing.T) {
			root := projectFixture(t, TemplateModule, TemplateName)
			before, err := os.ReadFile(filepath.Join(root, "go.mod"))
			require.NoError(t, err)
			_, err = Initialize(root, "example.com/tool", name)
			require.Error(t, err)
			after, err := os.ReadFile(filepath.Join(root, "go.mod"))
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.NoDirExists(t, filepath.Join(root, "cmd", name))
		})
	}
}

func TestInspectRejectsGoVersionNames(t *testing.T) {
	for _, name := range []string{"v2", "v10"} {
		t.Run(name, func(t *testing.T) {
			root := projectFixture(t, "example.com/tool", name)
			_, err := Inspect(root)
			require.Error(t, err)
		})
	}
	for _, name := range []string{"v0", "v1", "v02"} {
		t.Run(name, func(t *testing.T) {
			root := projectFixture(t, "example.com/tool", name)
			project, err := Inspect(root)
			require.NoError(t, err)
			require.Equal(t, name, project.Name)
		})
	}
}
