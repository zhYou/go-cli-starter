package diagnostics

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
	leaseunavailable       = fault.Definition{Code: "diagnostics.lease_unavailable", Kind: fault.Runtime}
	unknownartifact        = fault.Definition{Code: "diagnostics.unknown_artifact", Kind: fault.Runtime}
	invalidmaintenancelock = fault.Definition{Code: "diagnostics.invalid_maintenance_lock", Kind: fault.Runtime}
	invaliddirectory       = fault.Definition{Code: "diagnostics.invalid_directory", Kind: fault.Runtime}
	unknowncontent         = fault.Definition{Code: "diagnostics.unknown_content", Kind: fault.Runtime}
	invalidmetadata        = fault.Definition{Code: "diagnostics.invalid_metadata", Kind: fault.Runtime}
	unrecognizedmetadata   = fault.Definition{Code: "diagnostics.unrecognized_metadata", Kind: fault.Runtime}
)
