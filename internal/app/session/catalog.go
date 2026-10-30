package session

import (
	"embed"
	"io/fs"
)

//go:embed locales/*.json
var catalog embed.FS

// Catalog 提供只读的 Session 文案目录。
func Catalog() fs.FS { return catalog }
