package cli

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/zhYou/go-cli-starter/internal/contract"
	"reflect"
)

type resultRegistry map[*cobra.Command]json.RawMessage

func registerResult[T any](registry *resultRegistry, cmd *cobra.Command, override json.RawMessage) error {
	schema, err := contract.FromType(reflect.TypeFor[T](), override)
	if err != nil {
		return err
	}
	if *registry == nil {
		*registry = make(resultRegistry)
	}
	(*registry)[cmd] = schema
	return nil
}
