package scaffold

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestOriginWithoutGitAndLegacyProject(t *testing.T) {
	root := fixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".template-version"), []byte("0.1.0\n"), 0600))
	_, err := Initialize(root, "example.com/tool", "tool")
	require.NoError(t, err)
	_, err = makeOrigin(root, "example.com/tool", "tool")
	require.NoError(t, err)
	origin, err := ReadOrigin(afero.NewOsFs(), root)
	require.NoError(t, err)
	require.Equal(t, "unknown", origin.SourceCommit)
	require.Equal(t, TemplateModule, origin.Template)
	require.Equal(t, "0.1.0", origin.TemplateVersion)
	legacy := fixture(t)
	_, err = Initialize(legacy, "example.com/tool", "tool")
	require.NoError(t, err)
	_, err = ReadOrigin(afero.NewOsFs(), legacy)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = Initialize(legacy, "example.com/tool", "tool")
	require.NoError(t, err)
}
func TestOriginIsIdempotent(t *testing.T) {
	root := fixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".template-version"), []byte("0.1.0\n"), 0600))
	_, err := Initialize(root, "example.com/tool", "tool")
	require.NoError(t, err)
	before := read(t, root, ".template-origin.json")
	changed, err := Initialize(root, "example.com/tool", "tool")
	require.NoError(t, err)
	require.Empty(t, changed)
	require.Equal(t, before, read(t, root, ".template-origin.json"))
}
func TestOriginFailureRollsBackRename(t *testing.T) {
	i, root := memoryInitializer(t)
	base := i.FS
	require.NoError(t, afero.WriteFile(base, filepath.Join(root, ".template-version"), []byte("0.1.0\n"), 0600))
	before, _ := afero.ReadFile(base, filepath.Join(root, "go.mod"))
	i.FS = faultFS{Fs: base, rename: func(a, b string) error {
		if filepath.Base(b) == ".template-origin.json" {
			return errors.New("来源写入失败")
		}
		return base.Rename(a, b)
	}}
	_, err := i.Initialize(context.Background(), root, "example.com/tool", "tool")
	require.Error(t, err)
	after, _ := afero.ReadFile(base, filepath.Join(root, "go.mod"))
	require.Equal(t, before, after)
	ok, _ := afero.Exists(base, filepath.Join(root, ".template-origin.json"))
	require.False(t, ok)
}
