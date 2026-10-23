package main

import (
	"encoding/json"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/tfournet/GoDryCrap/internal/gate"
)

// stubBuildInfo makes readBuildInfo return info for the rest of the test.
func stubBuildInfo(t *testing.T, info *debug.BuildInfo, ok bool) {
	t.Helper()
	saved := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return info, ok }
	t.Cleanup(func() { readBuildInfo = saved })
}

func TestModuleVersionFallbacks(t *testing.T) {
	rev := func(r, modified string) []debug.BuildSetting {
		return []debug.BuildSetting{{Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: r}, {Key: "vcs.modified", Value: modified}}
	}
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"go install @version", &debug.BuildInfo{Main: debug.Module{Version: "v1.1.0"}}, true, "v1.1.0"},
		{"clean checkout", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: rev("08f3dc978db5cd2ef3fcf933b8aed98c2788de81", "false")}, true, "08f3dc978db5"},
		{"modified checkout", &debug.BuildInfo{Settings: rev("08f3dc978db5cd2ef3fcf933b8aed98c2788de81", "true")}, true, "08f3dc978db5+dirty"},
		{"short revision", &debug.BuildInfo{Settings: rev("abc123", "false")}, true, "abc123"},
		{"no vcs stamp", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, "devel"},
		{"no build info", nil, false, "devel"},
	}
	for _, tc := range cases {
		if got := moduleVersion(tc.info, tc.ok); got != tc.want {
			t.Errorf("%s: moduleVersion = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	stubBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v9.8.7"}}, true)
	want := "godrycrap v9.8.7 (" + runtime.Version() + ")\n"
	for _, arg := range []string{"version", "-version", "--version"} {
		res := runIn(t, arg)
		if res.status != gate.OK || res.stdout != want || res.stderr != "" {
			t.Errorf("%s: status %v, stdout %q, stderr %q; want %q", arg, res.status, res.stdout, res.stderr, want)
		}
	}
}

func TestBannerPerFormat(t *testing.T) {
	stubBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v9.8.7"}}, true)
	line := "godrycrap v9.8.7 (" + runtime.Version() + ")"
	r := newRepo(t)
	r.stage(map[string]string{"a.go": "package m\n\n" + fn("A", 2)})
	for _, cmd := range []string{"complexity", "dupes"} {
		if got := r.run(cmd).stdout; !strings.HasPrefix(got, line+"\ngodrycrap "+cmd+": ") {
			t.Errorf("%s text output =\n%s", cmd, got)
		}
		if got := r.run(cmd, "-format", "markdown").stdout; !strings.HasPrefix(got, line+"\n\n### godrycrap "+cmd+"\n") {
			t.Errorf("%s markdown output =\n%s", cmd, got)
		}
		if got := r.run(cmd, "-format", "tsv").stdout; strings.Contains(got, "godrycrap") {
			t.Errorf("%s tsv output has a banner:\n%s", cmd, got)
		}
		var payload struct {
			Version   string `json:"version"`
			GoVersion string `json:"go_version"`
		}
		if err := json.Unmarshal([]byte(r.run(cmd, "-format", "json").stdout), &payload); err != nil || payload.Version != "v9.8.7" || payload.GoVersion != runtime.Version() {
			t.Errorf("%s json version = %+v, %v", cmd, payload, err)
		}
	}
}

func TestBannerOnCrap(t *testing.T) {
	stubBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v9.8.7"}}, true)
	res := runIn(t, "crap", "-coverprofile", "../../testdata/crapper/coverage/coverage.out", "../../testdata/crapper/coverage")
	if !strings.HasPrefix(res.stdout, "godrycrap v9.8.7 (") {
		t.Errorf("crap text output =\n%s%s", res.stdout, res.stderr)
	}
}
