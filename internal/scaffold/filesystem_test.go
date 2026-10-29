package scaffold

import (
	"context"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryRejectsRealSymlink(t *testing.T) {
	root := fixture(t)
	fs := afero.NewOsFs()
	i := &Initializer{FS: faultFS{Fs: fs, rename: func(a, b string) error {
		err := fs.Rename(a, b)
		if err == nil && b == filepath.Join(root, "go.mod") {
			panic("中断")
		}
		return err
	}}}
	require.Panics(t, func() { _, _ = i.Initialize(context.Background(), root, "example.com/tool", "tool") })
	outside := filepath.Join(t.TempDir(), "outside")
	require.NoError(t, os.WriteFile(outside, []byte("人工内容"), 0600))
	require.NoError(t, os.Remove(filepath.Join(root, "main.go")))
	if err := os.Symlink(outside, filepath.Join(root, "main.go")); err != nil {
		t.Skipf("当前平台不支持创建符号链接：%v", err)
	}
	_, err := Initialize(root, "example.com/tool", "tool")
	require.Error(t, err)
	data, err := os.ReadFile(outside)
	require.NoError(t, err)
	require.Equal(t, "人工内容", string(data))
}
func TestCanceledInitializationDoesNotWrite(t *testing.T) {
	i, root := memoryInitializer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := i.Initialize(ctx, root, "example.com/tool", "tool")
	require.ErrorIs(t, err, context.Canceled)
	ok, _ := afero.Exists(i.FS, filepath.Join(root, journalDir))
	require.False(t, ok)
}
