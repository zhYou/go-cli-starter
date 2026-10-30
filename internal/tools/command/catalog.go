package command

import (
	"embed"
	"io/fs"
	"sync"

	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
)

//go:embed locales/*.json
var catalogs embed.FS

// Catalog 返回工具执行模块的只读消息目录，供应用入口组合。
func Catalog() fs.FS { return catalogs }

var (
	programRequired     = fault.Definition{Code: "command.program_required", Kind: fault.Runtime}
	argumentRequired    = fault.Definition{Code: "command.argument_required", Kind: fault.Runtime}
	waitDelayInvalid    = fault.Definition{Code: "command.wait_delay_invalid", Kind: fault.Runtime}
	captureLimitInvalid = fault.Definition{Code: "command.capture_limit_invalid", Kind: fault.Runtime}
	batchOptionsInvalid = fault.Definition{Code: "command.batch_options_invalid", Kind: fault.Runtime}
	batchItemFailed     = fault.Definition{Code: "command.batch_item_failed", Kind: fault.Runtime}
	batchItemIncomplete = fault.Definition{Code: "command.batch_item_incomplete", Kind: fault.Runtime}
)

var defaultBundle = sync.OnceValues(func() (*messages.Bundle, error) { return messages.NewBundle(Catalog()) })

func commandLocalizer(provided *messages.Localizer) *messages.Localizer {
	if provided != nil {
		return provided
	}
	bundle, err := defaultBundle()
	if err != nil {
		return nil
	}
	return bundle.Localizer("zh-CN")
}
