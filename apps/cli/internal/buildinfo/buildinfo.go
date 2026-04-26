package buildinfo

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// These variables are intended to be overridden at build time via -ldflags "-X ...".
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func NormalizedVersion() string {
	v := strings.TrimSpace(Version)
	v = strings.TrimPrefix(v, "cli/")
	v = strings.TrimPrefix(v, "v")
	return v
}

func UserAgent(binaryName string) string {
	name := strings.TrimSpace(binaryName)
	if name == "" {
		name = "labkit"
	}
	v := NormalizedVersion()
	if v == "" {
		v = "dev"
	}
	return fmt.Sprintf("%s/%s (%s; %s)", name, v, runtime.GOOS, runtime.GOARCH)
}

// VersionCode converts SemVer MAJOR.MINOR.PATCH into an int: MAJOR*1_000_000 + MINOR*1_000 + PATCH.
// Non-semver inputs return 0.
func VersionCode() int {
	major, minor, patch, ok := parseSemver(NormalizedVersion())
	if !ok {
		return 0
	}
	return major*1_000_000 + minor*1_000 + patch
}

func parseSemver(v string) (int, int, int, bool) {
	parts := strings.Split(strings.TrimSpace(v), ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return 0, 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return 0, 0, 0, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

