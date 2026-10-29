package contract

import (
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

type encoded string

func (encoded) MarshalJSON() ([]byte, error) { return []byte("42"), nil }
func TestResultSchemaMatchesEncoding(t *testing.T) {
	type result struct {
		Message  string         `json:"message"`
		Items    []string       `json:"items"`
		Optional *string        `json:"optional,omitempty"`
		Pointer  *string        `json:"pointer"`
		Map      map[string]int `json:"map"`
	}
	data, err := FromType(reflect.TypeFor[result](), nil)
	require.NoError(t, err)
	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal(data, &schema))
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	for _, value := range []result{{Message: "你好"}, {Message: "你好", Items: []string{}, Map: map[string]int{"x": 1}}} {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		var v any
		require.NoError(t, json.Unmarshal(raw, &v))
		require.NoError(t, resolved.Validate(v))
	}
	valid := map[string]any{"message": "你好", "items": nil, "pointer": nil, "map": nil, "future": true}
	require.NoError(t, resolved.Validate(valid))
	delete(valid, "message")
	require.Error(t, resolved.Validate(valid))
	valid["message"] = 42
	require.Error(t, resolved.Validate(valid))
	require.Empty(t, cmp.Diff([]string{"message", "items", "pointer", "map"}, schema.Required))
	_, err = FromType(reflect.TypeFor[encoded](), nil)
	require.Error(t, err)
	override, err := FromType(reflect.TypeFor[encoded](), json.RawMessage(`{"type":"integer"}`))
	require.NoError(t, err)
	require.Contains(t, string(override), `"integer"`)
	type recursive struct{ Next *recursive }
	_, err = FromType(reflect.TypeFor[recursive](), nil)
	require.Error(t, err)
	_, err = FromType(reflect.TypeFor[string](), json.RawMessage(`{"$ref":"https://example.invalid/schema"}`))
	require.Error(t, err)
}
