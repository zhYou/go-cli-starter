package contract

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNormalizationPreservesBusinessFieldNames(t *testing.T) {
	for _, name := range []string{"description", "examples", "use"} {
		t.Run(name, func(t *testing.T) {
			document := func(kind string) []byte {
				value := map[string]any{"schema": map[string]any{"commands": []any{map[string]any{"path": "contract-cli greet", "description": "中文命令说明", "flags": []any{}, "commands": []any{}, "result_schema": map[string]any{"type": "object", "properties": map[string]any{name: map[string]any{"type": kind, "description": "字段说明"}}}}}}, "results": map[string]any{"greet": map[string]any{name: kind}}}
				raw, err := json.Marshal(value)
				require.NoError(t, err)
				return raw
			}
			before, err := Normalize(document("string"))
			require.NoError(t, err)
			after, err := Normalize(document("number"))
			require.NoError(t, err)
			var doc map[string]any
			require.NoError(t, json.Unmarshal(before, &doc))
			require.Equal(t, "string", doc["results"].(map[string]any)["greet"].(map[string]any)[name])
			command := doc["schema"].(map[string]any)["commands"].(map[string]any)["contract-cli greet"].(map[string]any)
			require.NotContains(t, command, "description")
			fields := command["result_schema"].(map[string]any)["properties"].(map[string]any)
			require.Contains(t, fields, name)
			require.Equal(t, "string", fields[name].(map[string]any)["type"])
			changes, err := Diff(before, after)
			require.NoError(t, err)
			require.Len(t, changes, 2)
			for _, change := range changes {
				require.Equal(t, "breaking", change.Kind)
			}
		})
	}
}
