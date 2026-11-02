package i18ncheck

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func pair(en, zh string) fstest.MapFS {
	return fstest.MapFS{
		"locales/active.en.toml":    {Data: []byte(en)},
		"locales/active.zh-CN.toml": {Data: []byte(zh)},
	}
}

func TestCheckAcceptsParameterUnionAcrossPluralForms(t *testing.T) {
	catalog := pair(`["items.count"]
 description = "items"
 one = "{{.Name}}"
 other = "{{.Count}}"
`, `["items.count"]
 other = "{{.Count}} {{.Name}}"
`)
	require.NoError(t, Check(catalog))
}

func TestCheckFindsNestedTemplateParameters(t *testing.T) {
	catalog := pair(`["account"]
other = '{{if .Active}}{{.Name}}{{else}}{{index . "Fallback"}}{{end}}'
`, `["account"]
other = "{{.Fallback}} {{.Active}} {{.Name}}"
`)
	require.NoError(t, Check(catalog))
}

func TestCheckRejectsInvalidOrMismatchedCatalogs(t *testing.T) {
	valid := `["sample.message"]
other = "Hello {{.Name}}"
`
	tests := map[string]fstest.MapFS{
		"empty directory":      {},
		"missing language":     {"locales/active.en.toml": {Data: []byte(valid)}},
		"empty catalog":        pair("", valid),
		"duplicate id":         pair(valid+valid, valid),
		"unknown field":        pair(valid+"typo = \"x\"\n", valid),
		"missing other":        pair("[\"sample.message\"]\none = \"hi\"\n", valid),
		"empty message":        pair("[\"sample.message\"]\nother = \"   \"\n", valid),
		"empty plural form":    pair(valid+"one = \"\"\n", valid),
		"invalid template":     pair("[\"sample.message\"]\nother = \"{{.Name\"\n", valid),
		"unquoted dotted id":   pair("[sample.message]\nother = \"hi\"\n", valid),
		"different ids":        pair(valid, "[\"other.message\"]\nother = \"Hello {{.Name}}\"\n"),
		"different parameters": pair(valid, "[\"sample.message\"]\nother = \"Hello {{.Other}}\"\n"),
		"extra language file": {
			"locales/active.en.toml":    {Data: []byte(valid)},
			"locales/active.zh-CN.toml": {Data: []byte(valid)},
			"locales/active.fr.toml":    {Data: []byte(valid)},
		},
	}
	for name, catalog := range tests {
		t.Run(name, func(t *testing.T) {
			require.Error(t, Check(catalog))
		})
	}
}

func TestCheckAcceptsRootCatalogFiles(t *testing.T) {
	catalog := fstest.MapFS{
		"active.en.toml":    {Data: []byte("[\"hello\"]\nother = \"Hello {{.Name}}\"\n")},
		"active.zh-CN.toml": {Data: []byte("[\"hello\"]\nother = \"你好 {{.Name}}\"\n")},
	}
	require.NoError(t, Check(catalog))
}

func TestCheckUnderstandsChangedTemplateDot(t *testing.T) {
	catalog := pair(`["profile"]
other = '{{with .User}}{{.Name}}{{end}} {{$.Count}}'
`, `["profile"]
other = "{{.User.Name}} {{.Count}}"
`)
	require.NoError(t, Check(catalog))
}

func TestCheckIgnoresNestedIndexAsRootParameter(t *testing.T) {
	catalog := pair(`["profile"]
other = '{{with .User}}{{index . "Name"}}{{end}}'
`, `["profile"]
other = "{{.User.Name}}"
`)
	require.NoError(t, Check(catalog))
}

func TestCheckComparesParametersAccessedThroughRootAliases(t *testing.T) {
	catalog := pair(`["profile"]
other = '{{$root := .}}{{$root.Name}}'
`, `["profile"]
other = '{{$root := .}}{{$root.Title}}'
`)
	require.Error(t, Check(catalog))
}
