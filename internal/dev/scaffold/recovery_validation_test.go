package scaffold

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestRecoveryRejectsInconsistentExistence(t *testing.T) {
	for _, mode := range []string{"false", "missing", "null", "missing-mode"} {
		t.Run(mode, func(t *testing.T) {
			i, root := crashedTransaction(t)
			path := filepath.Join(root, journalDir, "journal.json")
			data, err := afero.ReadFile(i.FS, path)
			require.NoError(t, err)
			var doc map[string]any
			require.NoError(t, json.Unmarshal(data, &doc))
			file := doc["files"].([]any)[0].(map[string]any)
			switch mode {
			case "false":
				file["existed"] = false
			case "missing":
				delete(file, "existed")
			case "null":
				file["existed"] = nil
			case "missing-mode":
				delete(file, "mode")
			}
			modified, err := json.Marshal(doc)
			require.NoError(t, err)
			require.NoError(t, afero.WriteFile(i.FS, path, modified, 0600))
			before, err := afero.ReadFile(i.FS, filepath.Join(root, "go.mod"))
			require.NoError(t, err)
			_, err = i.Initialize(context.Background(), root, "example.com/tool", "tool")
			require.Error(t, err)
			after, err := afero.ReadFile(i.FS, filepath.Join(root, "go.mod"))
			require.NoError(t, err)
			require.Equal(t, before, after)
			backup, err := afero.ReadFile(i.FS, filepath.Join(root, journalDir, "000000.before"))
			require.NoError(t, err)
			require.Contains(t, string(backup), TemplateModule)
		})
	}
}
