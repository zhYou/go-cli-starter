package diagnostics

import (
	"os"
)

func writeMetadataPath(path string, value metadata) error {
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return writeMetadata(root, value)
}
