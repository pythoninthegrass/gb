// Package version provides build-time version information for the gb application.
package version

import (
	"fmt"
	"runtime"
)

// Build-time variables set via ldflags during compilation
var (
	// Version is the semantic version of the application
	Version = "dev"
	// BuildTime is when the binary was built
	BuildTime = "unknown"
	// Commit is the git commit hash
	Commit = "unknown"
)

// Info contains comprehensive version and build information
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
	Arch      string `json:"arch"`
}

// Get returns comprehensive version information
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// String returns a human-readable version string
func String() string {
	return Version
}

// BuildInfo returns detailed build information as a formatted string
func BuildInfo() string {
	info := Get()
	return fmt.Sprintf(`Version:    %s
Commit:     %s
Built:      %s
Go:         %s
Platform:   %s/%s`,
		info.Version,
		info.Commit,
		info.BuildTime,
		info.GoVersion,
		info.Platform,
		info.Arch,
	)
}