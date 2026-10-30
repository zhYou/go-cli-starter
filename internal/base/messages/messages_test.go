package messages

import (
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"testing"
	"testing/fstest"
)

func TestBundleIsolationFallbackAndErrorCauses(t *testing.T) {
	fs := fstest.MapFS{
		"locales/active.en.json":    {Data: []byte(`{"sample":{"other":"Hello, {{.Name}}!"}}`)},
		"locales/active.zh-CN.json": {Data: []byte(`{"sample":{"other":"你好，{{.Name}}！"}}`)},
	}
	b, err := NewBundle(fs)
	require.NoError(t, err)
	en, zh := b.Localizer("en"), b.Localizer("zh-CN")
	require.Equal(t, "Hello, Ada!", en.Text("sample", map[string]any{"Name": "Ada"}))
	require.Equal(t, "你好，Ada！", zh.Text("sample", map[string]any{"Name": "Ada"}))
	require.NotContains(t, en.Text("missing", nil), "log")
	cause := errors.New("disk full")
	e := (fault.Definition{Code: "sample", Kind: fault.Runtime}).Wrap(cause, fault.WithParams(fault.Params{"Name": "Ada"}))
	require.Contains(t, en.Render(e), "Hello, Ada!")
	require.Contains(t, en.Render(e), "disk full")
	require.NotContains(t, en.Text("sample", nil), "<no value>")
	require.Equal(t, "Hello, <no value>!", en.Text("sample", map[string]any{"Name": "<no value>"}))
}
