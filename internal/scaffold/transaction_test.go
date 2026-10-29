package scaffold

import (
	"bufio"
	"context"
	"errors"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type faultFS struct {
	afero.Fs
	rename func(string, string) error
}

func (f faultFS) Rename(a, b string) error {
	if f.rename != nil {
		return f.rename(a, b)
	}
	return f.Fs.Rename(a, b)
}
func memoryInitializer(t *testing.T) (*Initializer, string) {
	fs := afero.NewMemMapFs()
	root, err := filepath.Abs(filepath.Join(t.TempDir(), "repo"))
	require.NoError(t, err)
	require.NoError(t, fs.MkdirAll(root, 0755))
	for n, s := range map[string]string{"go.mod": "module " + TemplateModule + "\n\ngo 1.27.0\n", "main.go": "package main\nconst appName = \"" + TemplateName + "\"\nfunc main(){}\n", "README.md": TemplateName} {
		require.NoError(t, afero.WriteFile(fs, filepath.Join(root, n), []byte(s), 0600))
	}
	return &Initializer{FS: fs, RunCommand: func(context.Context, string, ...string) ([]byte, error) { return nil, nil }, Lock: func(string) (func() error, error) { return func() error { return nil }, nil }}, root
}
func crashedTransaction(t *testing.T) (*Initializer, string) {
	i, root := memoryInitializer(t)
	base := i.FS
	i.FS = faultFS{Fs: base, rename: func(a, b string) error {
		err := base.Rename(a, b)
		if err == nil && b == filepath.Join(root, "go.mod") {
			panic("模拟进程中断")
		}
		return err
	}}
	require.Panics(t, func() {
		_, err := i.Initialize(context.Background(), root, "example.com/tool", "tool")
		require.NoError(t, err)
	})
	i.FS = base
	return i, root
}
func TestRecoverBeforeIdentityCheck(t *testing.T) {
	i, root := crashedTransaction(t)
	module, _ := afero.ReadFile(i.FS, filepath.Join(root, "go.mod"))
	require.Contains(t, string(module), "example.com/tool")
	main, _ := afero.ReadFile(i.FS, filepath.Join(root, "main.go"))
	require.Contains(t, string(main), TemplateName)
	_, err := i.Initialize(context.Background(), root, "example.com/tool", "tool")
	require.NoError(t, err)
	main, _ = afero.ReadFile(i.FS, filepath.Join(root, "main.go"))
	require.Contains(t, string(main), `"tool"`)
}
func TestRecoveryRejectsEditedOrEscapingPaths(t *testing.T) {
	for _, mode := range []string{"edited", "escape", "conflict", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			i, root := crashedTransaction(t)
			path := filepath.Join(root, "main.go")
			journal := filepath.Join(root, ".template-init", "journal.json")
			switch mode {
			case "edited":
				require.NoError(t, afero.WriteFile(i.FS, path, []byte("人工修改"), 0600))
			case "escape":
				data, _ := afero.ReadFile(i.FS, journal)
				require.NoError(t, afero.WriteFile(i.FS, journal, []byte(strings.Replace(string(data), `"go.mod"`, `"../outside"`, 1)), 0600))
			case "corrupt":
				require.NoError(t, afero.WriteFile(i.FS, journal, []byte("{"), 0600))
			}
			before, _ := afero.ReadFile(i.FS, path)
			module := "example.com/tool"
			if mode == "conflict" {
				module = "example.com/other"
			}
			_, err := i.Initialize(context.Background(), root, module, "tool")
			require.Error(t, err)
			after, _ := afero.ReadFile(i.FS, path)
			require.Equal(t, before, after)
			ok, _ := afero.Exists(i.FS, journal)
			require.True(t, ok)
		})
	}
}
func TestRollbackFailureKeepsRecoveryData(t *testing.T) {
	i, root := memoryInitializer(t)
	base := i.FS
	targets := 0
	i.FS = faultFS{Fs: base, rename: func(a, b string) error {
		if filepath.Dir(b) == root && !strings.HasPrefix(filepath.Base(b), ".") {
			targets++
			if targets >= 2 {
				return errors.New("写入和回滚失败")
			}
		}
		return base.Rename(a, b)
	}}
	_, err := i.Initialize(context.Background(), root, "example.com/tool", "tool")
	require.Error(t, err)
	ok, _ := afero.Exists(base, filepath.Join(root, ".template-init", "journal.json"))
	require.True(t, ok)
	i.FS = base
	_, err = i.Initialize(context.Background(), root, "example.com/tool", "tool")
	require.NoError(t, err)
}
func TestWriteFailureRestoresOriginalBytes(t *testing.T) {
	for failAt := 1; failAt <= 3; failAt++ {
		i, root := memoryInitializer(t)
		base := i.FS
		before := map[string][]byte{}
		for _, n := range []string{"go.mod", "main.go", "README.md"} {
			before[n], _ = afero.ReadFile(base, filepath.Join(root, n))
		}
		count := 0
		i.FS = faultFS{Fs: base, rename: func(a, b string) error {
			if filepath.Dir(b) == root && !strings.HasPrefix(filepath.Base(b), ".") {
				count++
				if count == failAt {
					return errors.New("指定写入失败")
				}
			}
			return base.Rename(a, b)
		}}
		_, err := i.Initialize(context.Background(), root, "example.com/tool", "tool")
		require.Error(t, err)
		for n, want := range before {
			got, _ := afero.ReadFile(base, filepath.Join(root, n))
			require.Equal(t, want, got, n)
		}
	}
}
func TestLockHelper(t *testing.T) {
	if os.Getenv("TEMPLATE_LOCK_HELPER") != "1" {
		return
	}
	unlock, err := osLock(os.Getenv("TEMPLATE_LOCK_PATH"))
	if err != nil {
		os.Exit(3)
	}
	defer func() { _ = unlock() }()
	_, _ = os.Stdout.WriteString("就绪\n")
	_, _ = os.Stdin.Read(make([]byte, 1))
}
func TestConcurrentInitializationDoesNotRecoverLiveTransaction(t *testing.T) {
	root := fixture(t)
	lock := filepath.Join(root, ".template-init.lock")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelper$")
	cmd.Env = append(os.Environ(), "TEMPLATE_LOCK_HELPER=1", "TEMPLATE_LOCK_PATH="+lock)
	in, err := cmd.StdinPipe()
	require.NoError(t, err)
	out, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = in.Close(); _ = cmd.Process.Kill() })
	line, err := bufio.NewReader(out).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "就绪\n", line)
	_, err = Initialize(root, "example.com/tool", "tool")
	require.Error(t, err)
	require.Contains(t, read(t, root, "go.mod"), TemplateModule)
	require.NoError(t, cmd.Process.Kill())
	require.Error(t, cmd.Wait())
	_, err = Initialize(root, "example.com/tool", "tool")
	require.NoError(t, err)
}
