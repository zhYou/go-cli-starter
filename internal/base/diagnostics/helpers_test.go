package diagnostics

import (
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeMetadataPath(path string, value metadata) error {
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return writeMetadata(root, value)
}

// startDiagnosticProcess 在成功启动后登记唯一 Wait 所有者，早退也终止并等待子进程。
func startDiagnosticProcess(t *testing.T, child *exec.Cmd) func() error {
	t.Helper()
	require.NoError(t, child.Start())
	var once sync.Once
	var waitErr error
	wait := func() error { once.Do(func() { waitErr = child.Wait() }); return waitErr }
	t.Cleanup(func() { _ = child.Process.Kill(); _ = wait() })
	return wait
}
