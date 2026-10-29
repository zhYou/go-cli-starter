package contract

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestContractNormalization(t *testing.T) {
	a := []byte(`{"commands":[{"path":"contract-cli greet","description":"旧文案","flags":[{"name":"name","type":"string","required":true,"description":"姓名"}]}]}`)
	b := []byte(`{"commands":[{"path":"contract-cli greet","description":"新文案","flags":[{"name":"name","type":"string","required":true,"description":"输入"}]}]}`)
	x, err := Normalize(a)
	require.NoError(t, err)
	y, err := Normalize(b)
	require.NoError(t, err)
	require.JSONEq(t, string(x), string(y))
	changes, err := Diff(x, []byte(`{"commands":{"contract-cli greet":{"path":"contract-cli greet","flags":{"name":{"name":"name","type":"number","required":true}}}}}`))
	require.NoError(t, err)
	require.NotEmpty(t, changes)
	require.Equal(t, "breaking", changes[0].Kind)
}

func TestExitCodeChangeRequiresBreakingVersion(t *testing.T) {
	changes, err := Diff([]byte(`{"schema":{"exit_codes":{"runtime":1}}}`), []byte(`{"schema":{"exit_codes":{"runtime":3}}}`))
	require.NoError(t, err)
	require.Len(t, changes, 1)
	record := Record{ID: "exit-code", Compatibility: "review-required", Summary: "修改运行退出码", Paths: []string{"/schema/exit_codes/runtime"}, PreviousVersion: "v1.0.0", TargetVersion: "v1.0.1", Migration: "更新调用者"}
	require.Error(t, ValidateRecords(changes, []Record{record}, false))
	record.Compatibility = "breaking"
	require.Error(t, ValidateRecords(changes, []Record{record}, false))
	record.TargetVersion = "v2.0.0"
	require.NoError(t, ValidateRecords(changes, []Record{record}, false))
}
