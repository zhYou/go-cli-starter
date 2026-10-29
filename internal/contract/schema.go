// Package contract 描述并比较 CLI 的机器接口。
package contract

import (
	"encoding/json"
	"fmt"
	"github.com/google/jsonschema-go/jsonschema"
	"reflect"
	"strings"
)

const Draft = "https://json-schema.org/draft/2020-12/schema"

// FromType 从 Go 类型推导结果结构；显式覆盖用于特殊编码，禁止加载外部引用。
func FromType(t reflect.Type, override json.RawMessage) (json.RawMessage, error) {
	var schema *jsonschema.Schema
	if len(override) > 0 {
		schema = &jsonschema.Schema{}
		if err := json.Unmarshal(override, schema); err != nil {
			return nil, fmt.Errorf("解析结果结构覆盖失败：%w", err)
		}
	} else {
		if t == nil {
			return nil, fmt.Errorf("结果类型不能为空")
		}
		if err := checkEncoding(t, map[reflect.Type]bool{}); err != nil {
			return nil, err
		}
		var err error
		schema, err = jsonschema.ForType(t, nil)
		if err != nil {
			return nil, fmt.Errorf("推导结果结构失败，请提供显式覆盖：%w", err)
		}
		openStructs(schema)
	}
	schema.Schema = Draft
	if _, err := schema.Resolve(nil); err != nil {
		return nil, fmt.Errorf("结果结构无效：%w", err)
	}
	return json.Marshal(schema)
}
func checkEncoding(t reflect.Type, seen map[reflect.Type]bool) error {
	if seen[t] {
		return nil
	}
	seen[t] = true
	marshaler := reflect.TypeFor[json.Marshaler]()
	if t.Implements(marshaler) || reflect.PointerTo(t).Implements(marshaler) {
		return fmt.Errorf("类型 %s 使用自定义 JSON 编码，需要显式结果结构", t)
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return checkEncoding(t.Elem(), seen)
	case reflect.Struct:
		for field := range t.Fields() {
			if field.IsExported() && strings.Split(field.Tag.Get("json"), ",")[0] != "-" {
				if err := checkEncoding(field.Type, seen); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func openStructs(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if s.AdditionalProperties != nil {
		raw, _ := json.Marshal(s.AdditionalProperties)
		if string(raw) == "false" {
			s.AdditionalProperties = &jsonschema.Schema{}
		} else {
			if s.Type == "object" {
				s.Type = ""
				s.Types = []string{"object", "null"}
			}
			openStructs(s.AdditionalProperties)
		}
	}
	for _, property := range s.Properties {
		openStructs(property)
	}
	for _, def := range s.Defs {
		openStructs(def)
	}
	openStructs(s.Items)
	for _, sub := range s.AnyOf {
		openStructs(sub)
	}
}
