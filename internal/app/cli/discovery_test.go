package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChineseVersionAndSchema(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		code, out, errOut := invoke(t, append([]string{"--lang", "zh"}, args...)...)
		if code != 0 || !strings.Contains(out, "版本") || !strings.Contains(out, "v1.2.3") || errOut != "" {
			t.Fatalf("参数=%v 退出码=%d 标准输出=%q 标准错误=%q", args, code, out, errOut)
		}
	}
	code, out, errOut := invoke(t, "--lang", "zh", "schema", "greet")
	var got struct {
		Commands []commandSchema `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if code != 0 || errOut != "" || len(got.Commands) != 1 || !strings.Contains(got.Commands[0].Description, "问候") {
		t.Fatalf("退出码=%d 标准输出=%q 标准错误=%q", code, out, errOut)
	}
}

func TestDiscoveryBypassesBrokenConfiguration(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(bad, []byte("[broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{}, {"--help"}, {"greet", "--help"}, {"version", "--json"},
		{"--version"}, {"schema"}, {"help", "greet"},
		{"completion", "bash"}, {"completion", "zsh"},
		{"completion", "fish"}, {"completion", "powershell"},
	} {
		code, out, errOut := invoke(t, append([]string{"--config", bad}, args...)...)
		if code != 0 || out == "" || errOut != "" {
			t.Errorf("args=%v code=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
	code, out, errOut := invoke(t, "greet", "--name", "Codex", "--config", bad)
	if code != 1 || out != "" || errOut == "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
}

func TestCompletionArityUsesPluralCatalog(t *testing.T) {
	code, out, stderr := invoke(t, "completion", "bash", "extra")
	require.Equal(t, 2, code)
	require.Empty(t, out)
	require.Contains(t, stderr, "received 2 arguments")
	require.NotContains(t, stderr, "Operation failed")
}
