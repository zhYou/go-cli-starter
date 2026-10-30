package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInitializePreservesHistoricalBacklogReferences 防止改名伪造来源 PR，并验证例外不扩展到其他文本或文件。
func TestInitializePreservesHistoricalBacklogReferences(t *testing.T) {
	root := projectFixture(t, TemplateModule, TemplateName)
	const reference = "[PR #5](https://github.com/zhYou/go-cli-starter/pull/5)"
	data, err := os.ReadFile("../../../docs/backlog.md")
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(data), reference))
	input := string(data) + "\napp=go-cli-starter module=github.com/zhYou/go-cli-starter env=GO_CLI_STARTER_HOME\n" +
		"[PR #6](https://github.com/zhYou/go-cli-starter/pull/6)\nhttps://github.com/zhYou/go-cli-starter/pull/5\n"
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0700))
	// #nosec G703 -- root 是 projectFixture 创建的临时目录，固定路径不来自文档内容。
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs/backlog.md"), []byte(input), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte(reference), 0600))
	unknown := reference + "\napp=go-cli-starter\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "user.md"), []byte(unknown), 0600))

	_, err = Initialize(root, "example.com/team/mycli", "mycli")
	require.NoError(t, err)
	after := read(t, root, "docs/backlog.md")
	require.Equal(t, 2, strings.Count(after, reference), "历史来源 PR 必须保持真实 URL")
	require.Contains(t, after, "app=mycli module=example.com/team/mycli env=MYCLI_HOME")
	require.Contains(t, after, "[PR #6](https://example.com/team/mycli/pull/6)")
	require.Contains(t, after, "\nhttps://example.com/team/mycli/pull/5\n")
	remaining := strings.ReplaceAll(after, reference, "")
	require.NotContains(t, remaining, "go-cli-starter")
	require.NotContains(t, remaining, "GO_CLI_STARTER")
	require.Equal(t, "[PR #5](https://example.com/team/mycli/pull/5)", read(t, root, "README.md"))
	require.Equal(t, unknown, read(t, root, "user.md"))

	changed, err := Initialize(root, "example.com/team/mycli", "mycli")
	require.NoError(t, err)
	require.Empty(t, changed)
	require.Equal(t, after, read(t, root, "docs/backlog.md"))
	_, err = Initialize(root, "example.com/other/tool", "tool")
	require.ErrorContains(t, err, "已初始化")
	require.Equal(t, after, read(t, root, "docs/backlog.md"))
}
