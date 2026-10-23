package main

import (
	"runtime"
	"runtime/debug"
)

// readBuildInfo is debug.ReadBuildInfo; tests replace it.
var readBuildInfo = debug.ReadBuildInfo

// version is the version line: "godrycrap v1.1.0 (go1.27.3)".
func version() string {
	return "godrycrap " + moduleVersion(readBuildInfo()) + " (" + runtime.Version() + ")"
}

// moduleVersion is the module version go install stamped, else the VCS
// revision go build stamped (with +dirty for a modified tree), else devel.
func moduleVersion(info *debug.BuildInfo, ok bool) string {
	if !ok {
		return "devel"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return revision(info.Settings)
}

func revision(settings []debug.BuildSetting) string {
	var rev, dirty string
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "+dirty"
			}
		}
	}
	if rev == "" {
		return "devel"
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return rev + dirty
}
