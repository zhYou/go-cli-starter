package cli

import (
	"context"
	"github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/validation"
	"go.uber.org/zap"
	"io"
)

// Dependencies 描述命令依赖；构造时复制并补齐默认值，不修改调用者对象。
type Dependencies struct {
	LoadConfig func(config.Options) (config.Config, error)
	Validator  *validation.Validator
	Greet      func(context.Context, greeting.Options) (greeting.Result, error)
	NewLogger  func(io.Writer, string) (*zap.Logger, error)
}
type invocation struct {
	results              resultRegistry
	asJSON               bool
	configPath, logLevel string
	deps                 Dependencies
	logger               *zap.Logger
}

func newInvocation(provided *Dependencies) *invocation {
	var deps Dependencies
	if provided != nil {
		deps = *provided
	}
	if deps.Validator == nil {
		deps.Validator = validation.New()
	}
	if deps.LoadConfig == nil {
		deps.LoadConfig = config.Load
	}
	if deps.Greet == nil {
		deps.Greet = greeting.New(deps.Validator).Run
	}
	if deps.NewLogger == nil {
		deps.NewLogger = logging.New
	}
	return &invocation{deps: deps, logLevel: "warn", logger: zap.NewNop()}
}
