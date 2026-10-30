package logging

import (
	"embed"
	"io/fs"

	"github.com/zhYou/go-cli-starter/internal/base/fault"
)

//go:embed locales/*.json
var catalog embed.FS

// Catalog 提供本包的只读消息目录。
func Catalog() fs.FS { return catalog }

var (
	writerunavailable = fault.Definition{Code: "logging.writer_unavailable", Kind: fault.Runtime}
)
