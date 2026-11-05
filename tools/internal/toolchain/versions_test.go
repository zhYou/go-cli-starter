package toolchain

import (
	"debug/buildinfo"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovulncheckBuildInfoExact(t *testing.T) {
	valid := buildinfo.BuildInfo{Path: "golang.org/x/vuln/cmd/govulncheck", Main: debug.Module{Path: "golang.org/x/vuln", Version: "v1.9.0"}}
	require.NoError(t, MatchGovulncheckBuildInfo(&valid, "1.9.0"))
	for _, mutate := range []func(*buildinfo.BuildInfo){
		func(b *buildinfo.BuildInfo) { b.Path = "example.org/other" },
		func(b *buildinfo.BuildInfo) { b.Main.Path = "example.org/other" },
		func(b *buildinfo.BuildInfo) { b.Main.Version = "v1.9.00" },
		func(b *buildinfo.BuildInfo) { b.Main.Version = "v1.9.0-rc.1" },
		func(b *buildinfo.BuildInfo) { b.Main.Replace = &debug.Module{Path: "local"} },
	} {
		b := valid
		mutate(&b)
		require.Error(t, MatchGovulncheckBuildInfo(&b, "1.9.0"))
	}
	spec, err := Spec("go:golang.org/x/vuln/cmd/govulncheck")
	require.NoError(t, err)
	require.Equal(t, []string{"-h"}, spec.Args)
}

func TestVersionRejectsPrereleaseOrBuildSuffix(t *testing.T) {
	for _, version := range []string{"1.27.30", "1.27.3-rc.1", "1.27.3+patched"} {
		require.Error(t, Match("go", "go version go"+version+" linux/amd64", "1.27.3"))
	}
}

func TestGoplsVersionMismatchSuggestsSetup(t *testing.T) {
	err := Match("go:golang.org/x/tools/gopls", "golang.org/x/tools/gopls v0.24.1", "0.24.0")
	require.ErrorContains(t, err, "mise run setup")
	require.NotContains(t, err.Error(), ":agent")
}
