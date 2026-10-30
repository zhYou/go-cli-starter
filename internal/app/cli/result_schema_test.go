package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"testing"
)

func TestSchemaGenerationDoesNotExecuteCommands(t *testing.T) {
	count := 0
	deps := Dependencies{LoadConfig: func(config.Options) (config.Config, error) { count++; return config.Config{}, nil }, Greet: func(context.Context, greeting.Options) (greeting.Result, error) {
		count++
		return greeting.Result{}, nil
	}}
	var out bytes.Buffer
	require.Zero(t, Run(context.Background(), []string{"schema", "greet"}, Options{Out: &out, Dependencies: &deps}))
	var doc struct {
		Commands []struct {
			Result json.RawMessage `json:"result_schema"`
		} `json:"commands"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	require.Len(t, doc.Commands, 1)
	require.NotEmpty(t, doc.Commands[0].Result)
	require.Zero(t, count)
}
