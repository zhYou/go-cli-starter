package cli

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/zhYou/go-cli-starter/internal/base/schema"
	"reflect"
)

type resultRegistry map[*cobra.Command]json.RawMessage

func registerResult[T any](registry *resultRegistry, cmd *cobra.Command, override json.RawMessage) error {
	resultSchema, err := schema.FromType(reflect.TypeFor[T](), override)
	if err != nil {
		return err
	}
	if *registry == nil {
		*registry = make(resultRegistry)
	}
	(*registry)[cmd] = resultSchema
	return nil
}
