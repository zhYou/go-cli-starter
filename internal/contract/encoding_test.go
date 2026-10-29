package contract

import (
	"encoding/json"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

type textEncoded int

func (textEncoded) MarshalText() ([]byte, error) { return []byte("编码文本"), nil }
func TestSpecialJSONEncodingRequiresOverride(t *testing.T) {
	for _, value := range []any{struct {
		Data []byte `json:"data"`
	}{[]byte{1, 2}}, struct {
		Count int `json:"count,string"`
	}{42}, struct {
		Text textEncoded `json:"text"`
	}{1}} {
		t.Run(reflect.TypeOf(value).String(), func(t *testing.T) {
			_, err := FromType(reflect.TypeOf(value), nil)
			require.Error(t, err)
			raw, err := json.Marshal(value)
			require.NoError(t, err)
			var actual map[string]any
			require.NoError(t, json.Unmarshal(raw, &actual))
			properties := map[string]any{}
			for name := range actual {
				properties[name] = map[string]any{"type": "string"}
			}
			override, err := json.Marshal(map[string]any{"type": "object", "properties": properties})
			require.NoError(t, err)
			data, err := FromType(reflect.TypeOf(value), override)
			require.NoError(t, err)
			var schema jsonschema.Schema
			require.NoError(t, json.Unmarshal(data, &schema))
			resolved, err := schema.Resolve(nil)
			require.NoError(t, err)
			require.NoError(t, resolved.Validate(actual))
		})
	}
}
