package greeting

import (
	"embed"
	"io/fs"
)

//go:embed locales/*.json
var catalogs embed.FS

// Catalog 返回只读的模块消息目录。
func Catalog() fs.FS { return catalogs }
