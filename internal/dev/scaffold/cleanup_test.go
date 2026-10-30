package scaffold

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type cleanupFaultFS struct {
	afero.Fs
	root, point                          string
	rollback, failedCleanup, failedApply bool
}

func (f *cleanupFaultFS) Rename(a, b string) error {
	if f.rollback && !f.failedApply && b == filepath.Join(f.root, "cmd", TemplateName, "main.go") {
		f.failedApply = true
		return errors.New("模拟应用失败")
	}
	return f.Fs.Rename(a, b)
}
func (f *cleanupFaultFS) Remove(path string) error {
	dir := filepath.Join(f.root, journalDir)
	match := false
	switch f.point {
	case "before":
		match = path == filepath.Join(dir, "000000.before")
	case "after":
		match = path == filepath.Join(dir, "000000.after")
	case "journal":
		match = path == filepath.Join(dir, "journal.json")
	case "directory":
		match = path == dir
	}
	if match && !f.failedCleanup {
		f.failedCleanup = true
		if f.point != "directory" {
			if err := f.Fs.Remove(path); err != nil {
				return err
			}
		}
		return errors.New("模拟部分清理后中断")
	}
	return f.Fs.Remove(path)
}
func (f *cleanupFaultFS) RemoveAll(path string) error {
	if path == filepath.Join(f.root, journalDir) && !f.failedCleanup {
		f.failedCleanup = true
		suffix := "000000." + f.point
		if f.point == "journal" {
			suffix = "journal.json"
		}
		if f.point == "directory" {
			if err := f.Fs.RemoveAll(path); err != nil {
				return err
			}
			if err := f.Mkdir(path, 0700); err != nil {
				return err
			}
		} else if err := f.Fs.Remove(filepath.Join(path, suffix)); err != nil {
			return err
		}
		return errors.New("模拟部分清理后中断")
	}
	return f.Fs.RemoveAll(path)
}
func TestTerminalCleanupIsRetryable(t *testing.T) {
	for _, outcome := range []string{"committed", "rolled-back"} {
		for _, point := range []string{"before", "after", "journal", "directory"} {
			t.Run(outcome+"/"+point, func(t *testing.T) {
				i, root := memoryInitializer(t)
				base := i.FS
				fault := &cleanupFaultFS{Fs: base, root: root, point: point, rollback: outcome == "rolled-back"}
				i.FS = fault
				_, err := i.Initialize(context.Background(), root, "example.com/tool", "tool")
				require.Error(t, err)
				require.True(t, fault.failedCleanup)
				mod, err := afero.ReadFile(base, filepath.Join(root, "go.mod"))
				require.NoError(t, err)
				if outcome == "committed" {
					require.Contains(t, string(mod), "example.com/tool")
				} else {
					require.Contains(t, string(mod), TemplateModule)
				}
				i.FS = base
				_, err = i.Initialize(context.Background(), root, "example.com/tool", "tool")
				require.NoError(t, err)
				mod, err = afero.ReadFile(base, filepath.Join(root, "go.mod"))
				require.NoError(t, err)
				require.True(t, strings.Contains(string(mod), "example.com/tool"))
				ok, err := afero.Exists(base, filepath.Join(root, journalDir))
				require.NoError(t, err)
				require.False(t, ok)
			})
		}
	}
}
