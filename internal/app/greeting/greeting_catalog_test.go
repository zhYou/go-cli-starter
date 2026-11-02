package greeting

import (
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
)

// 由工具包的覆盖构建测试运行，替换真实内嵌目录后验证调用边界。
func TestCorruptCatalogProcess(t *testing.T) {
	if os.Getenv("TEST_CORRUPT_I18N_CATALOG") != "1" {
		t.Skip("仅由损坏目录子进程执行")
	}
	result, err := Run(t.Context(), Options{Name: "Ada"})
	require.Error(t, err)
	require.Empty(t, result.Message)
	bundle, err := messages.NewBundle(fstest.MapFS{"active.en.toml": {Data: []byte("[\"greeting.message\"]\nother = \"Injected {{.Name}}\"\n")}})
	require.NoError(t, err)
	result, err = Run(t.Context(), Options{Name: "Ada", Localizer: bundle.Localizer("en")})
	require.NoError(t, err)
	require.Equal(t, "Injected Ada", result.Message)
}
