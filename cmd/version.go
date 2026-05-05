package cmd

import "runtime/debug"

// version is set at build time via:
//
//	go build -ldflags "-X github.com/gnugomez/scribe/cmd.version=v1.2.3"
//
// When installed via 'go install github.com/gnugomez/scribe@vX.Y.Z', the
// module version from the embedded build info is used automatically.
var version = ""

// buildVersion returns the running binary's version, with the following
// priority:
//
//  1. ldflags injection (-X cmd.version=...)
//  2. Module version embedded by 'go install' / 'go build' from a tagged module
//  3. "dev" when building from an untagged working tree
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok &&
		info.Main.Version != "" &&
		info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
