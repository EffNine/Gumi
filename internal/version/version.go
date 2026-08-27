// Package version holds build metadata for the gumi CLI.
package version

// Values are overridden at build time with -ldflags.
var (
	Version   = "v1.0.0"
	Commit    = "unknown"
	BuildDate = "unknown"
)
