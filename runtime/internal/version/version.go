// Package version holds Novexa release metadata injected at build time.
//
// Default values keep local development builds identifiable without requiring
// every developer to pass ldflags. Release pipelines override Version, Commit,
// and BuildDate using -X flags so `novexa version` can report the exact release
// it was built from.
package version

import "fmt"

// Release metadata. Override at link time with:
//
//	-X github.com/novexa/novexa/runtime/internal/version.Version=0.1.0
//	-X github.com/novexa/novexa/runtime/internal/version.Commit=<sha>
//	-X github.com/novexa/novexa/runtime/internal/version.BuildDate=<iso>
var (
	Version   = "0.1.0"
	Commit    = "dev"
	BuildDate = "unknown"
)

// Short returns the version string only. It is the stable part of the
// `novexa version` output and defaults to "0.1.0" for development builds.
func Short() string {
	return Version
}

// Full returns the version together with build metadata. It is used by release
// builds to show the exact commit and build date without breaking the one-line
// default output for development builds.
func Full() string {
	return fmt.Sprintf("Novexa Runtime %s\nCommit: %s\nBuild Date: %s\n", Version, Commit, BuildDate)
}
