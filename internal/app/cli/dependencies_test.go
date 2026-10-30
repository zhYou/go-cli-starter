package cli

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"go.uber.org/zap"
	"io"
	"testing"
)

func TestTwoInvocationsDoNotShareDependencies(t *testing.T) {
	var outA, outB bytes.Buffer
	var levels []string
	makeRoot := func(label string, out *bytes.Buffer) *invocationFixture {
		deps := Dependencies{LoadConfig: func(o config.Options) (config.Config, error) {
			level := "warn"
			if o.LogLevel != nil {
				level = *o.LogLevel
			}
			return config.Config{LogLevel: level}, nil
		}, Greet: func(_ context.Context, o greeting.Options) (greeting.Result, error) {
			return greeting.Result{Message: label + o.Name}, nil
		}, NewLogger: func(w io.Writer, l string) (*zap.Logger, error) { levels = append(levels, l); return logging.New(w, l) }}
		root := NewRoot(Options{Out: out, Dependencies: &deps})
		require.Nil(t, deps.Validator)
		return &invocationFixture{run: func(args ...string) {
			root.SetArgs(args)
			require.NoError(t, root.ExecuteContext(context.Background()))
		}}
	}
	a := makeRoot("甲", &outA)
	b := makeRoot("乙", &outB)
	a.run("greet", "--name", "A", "--json", "--log-level", "debug")
	b.run("greet", "--name", "B")
	require.JSONEq(t, fmt.Sprintf("{%q:%q}", "message", "甲A"), outA.String())
	require.Equal(t, "乙B\n", outB.String())
	require.Equal(t, []string{"debug", "warn"}, levels)
}

type invocationFixture struct{ run func(...string) }

func TestInvalidArgsNeverLoadConfigOrRunBusiness(t *testing.T) {
	for _, args := range [][]string{{"greet"}, {"greet", "--name", " "}, {"greet", "--name", "x", "extra"}, {"greet", "--unknown"}} {
		count := 0
		deps := Dependencies{LoadConfig: func(config.Options) (config.Config, error) { count++; return config.Config{}, nil }, Greet: func(context.Context, greeting.Options) (greeting.Result, error) {
			count++
			return greeting.Result{}, nil
		}}
		require.Equal(t, 2, Run(context.Background(), args, Options{Dependencies: &deps}))
		require.Zero(t, count)
	}
}
func TestDiscoveryNeverLoadsConfiguration(t *testing.T) {
	count := 0
	deps := Dependencies{LoadConfig: func(config.Options) (config.Config, error) { count++; return config.Config{}, nil }}
	for _, args := range [][]string{{"help", "greet"}, {"greet", "--help"}, {"version"}, {"--version"}, {"schema"}, {"completion", "bash"}, {"__complete", "greet", ""}} {
		require.Zero(t, Run(context.Background(), args, Options{Dependencies: &deps}), "%v", args)
	}
	require.Zero(t, count)
}
