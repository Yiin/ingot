// Package buildinfo exposes the running binary's version string.
package buildinfo

import "runtime/debug"

// version is overridden at release build time via:
//
//	-ldflags "-X github.com/Yiin/ingot/internal/buildinfo.version=$(VERSION)"
var version = "dev"

// Version returns the build version. It prefers the version embedded via
// -ldflags at release build time; otherwise it falls back to the VCS
// revision reported by the Go toolchain, and finally to "dev" for
// unreleased local builds where neither is available.
func Version() string {
	if version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}

	return version
}
