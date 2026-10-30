package telemetry

import (
	"embed"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"io/fs"
)

//go:embed locales/*.json
var catalog embed.FS

// Catalog 提供本包的只读消息目录。
func Catalog() fs.FS { return catalog }

var (
	writerunavailable = fault.Definition{Code: "telemetry.writer_unavailable", Kind: fault.Runtime}
	unknownexporter   = fault.Definition{Code: "telemetry.unknown_exporter", Kind: fault.Runtime}
	sdkenvironment    = fault.Definition{Code: "telemetry.sdk_environment", Kind: fault.Runtime}
)
