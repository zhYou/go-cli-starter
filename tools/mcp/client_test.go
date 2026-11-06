package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolResultRejectsProtocolAndToolErrors(t *testing.T) {
	for _, data := range []string{`invalid`, `{"isError":true,"content":[{"type":"text","text":"backend failed"}]}`} {
		_, err := toolText(json.RawMessage(data))
		require.Error(t, err)
	}
	text, err := toolText(
		json.RawMessage(`{"content":[{"type":"text","text":"first"},{"type":"text","text":"second"}]}`),
	)
	require.NoError(t, err)
	require.Equal(t, "first\nsecond", text)
}

func TestSessionInstructionsRequireAnActualSession(t *testing.T) {
	id, err := sessionFromInstructions(
		"<session>\nYour Serena session id is `abc-123`. Pass it as the `session` parameter.\n</session>",
	)
	require.NoError(t, err)
	require.Equal(t, "abc-123", id)
	_, err = sessionFromInstructions("No project is active")
	require.Error(t, err)
}
