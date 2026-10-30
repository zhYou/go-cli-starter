package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func projectFixture(t *testing.T, module, name string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+module+"\n\ngo 1.27.0\n"), 0600))
	dir := filepath.Join(root, "cmd", name)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nconst appName = \""+name+"\"\nfunc main() {}\n"), 0600))
	return root
}

func TestInspectCommandProject(t *testing.T) {
	root := projectFixture(t, "example.com/team/repository", "my-cli")
	project, err := Inspect(root)
	require.NoError(t, err)
	require.Equal(t, Project{"example.com/team/repository", "my-cli", "cmd/my-cli"}, project)
}

func TestInspectRejectsAmbiguousOrInvalidEntry(t *testing.T) {
	for _, mode := range []string{"missing", "multiple", "mismatch", "invalid-name", "syntax", "package", "no-main", "main-args", "variable"} {
		t.Run(mode, func(t *testing.T) {
			root := projectFixture(t, "example.com/tool", "tool")
			file := filepath.Join(root, "cmd", "tool", "main.go")
			switch mode {
			case "missing":
				require.NoError(t, os.Remove(file))
			case "multiple":
				require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd", "other"), 0755))
				require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "other", "main.go"), []byte("package main\nconst appName=\"other\"\nfunc main(){}"), 0600))
			default:
				source := map[string]string{
					"mismatch":     "package main\nconst appName=\"other\"\nfunc main(){}",
					"invalid-name": "package main\nconst appName=\"Bad\"\nfunc main(){}",
					"syntax":       "package main\n{",
					"package":      "package library\nconst appName=\"tool\"\nfunc main(){}",
					"no-main":      "package main\nconst appName=\"tool\"",
					"main-args":    "package main\nconst appName=\"tool\"\nfunc main(x int){}",
					"variable":     "package main\nvar appName=\"tool\"\nfunc main(){}",
				}[mode]
				require.NoError(t, os.WriteFile(file, []byte(source), 0600))
			}
			_, err := Inspect(root)
			require.Error(t, err)
		})
	}
}

func TestInspectRejectsSymlink(t *testing.T) {
	root := projectFixture(t, "example.com/tool", "tool")
	outside := filepath.Join(t.TempDir(), "outside.go")
	require.NoError(t, os.WriteFile(outside, []byte("package main\nconst appName=\"tool\"\nfunc main(){}"), 0600))
	file := filepath.Join(root, "cmd", "tool", "main.go")
	require.NoError(t, os.Remove(file))
	if err := os.Symlink(outside, file); err != nil {
		t.Skipf("当前平台无法创建符号链接：%v", err)
	}
	_, err := Inspect(root)
	require.Error(t, err)
}
