package main

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/tfournet/GoDryCrap/internal/coverage"
	"github.com/tfournet/GoDryCrap/internal/gate"
)

func TestUserErrorsGetAFixHint(t *testing.T) {
	notRepo := t.TempDir()
	cases := []struct {
		name string
		args []string
		hint string
	}{
		{"missing profile", []string{"crap", "-coverprofile", "missing.out", "."}, "go test -coverprofile=coverage.out"},
		{"no profile flag", []string{"crap", "."}, "-coverprofile coverage.out"},
		{"bad flag", []string{"complexity", "-nope"}, "run 'godrycrap complexity -h'"},
		{"bad format", []string{"dupes", "-format", "xml"}, "run 'godrycrap dupes -h'"},
		{"not a git repo", []string{"complexity", "-C", notRepo}, "inside a git repository"},
		{"missing path", []string{"complexity", "no/such/dir"}, "check that each path exists"},
	}
	for _, tc := range cases {
		res := runIn(t, tc.args...)
		if res.status != gate.Error || !strings.Contains(res.stderr, "godrycrap: hint: ") || !strings.Contains(res.stderr, tc.hint) || strings.Contains(res.stderr, "internal error") {
			t.Errorf("%s: status %v, stderr:\n%s", tc.name, res.status, res.stderr)
		}
	}
}

func TestBadRangeGetsAFixHint(t *testing.T) {
	r := newRepo(t)
	for _, spec := range []string{"main", "..main", "nope..main"} {
		res := r.run("complexity", "-diff", spec)
		if res.status != gate.Error || !strings.Contains(res.stderr, "godrycrap: hint: use -diff BASE..HEAD") {
			t.Errorf("-diff %s: status %v, stderr:\n%s", spec, res.status, res.stderr)
		}
	}
}

func TestSyntaxErrorGetsAFixHint(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"a.go": "package m\n\nfunc A( {\n"})
	res := r.run("complexity")
	if res.status != gate.Error || !strings.Contains(res.stderr, "godrycrap: hint: fix the syntax error") {
		t.Errorf("status %v, stderr:\n%s", res.status, res.stderr)
	}
}

func TestProfileErrorHints(t *testing.T) {
	cases := []struct {
		err  error
		hint string
	}{
		{fmt.Errorf("%w: f.go", coverage.ErrMismatch), staleHint},
		{fmt.Errorf("%w: f.go", coverage.ErrNotInProfile), shardHint},
	}
	for _, tc := range cases {
		var b strings.Builder
		fail(&b, "crap", profileError(tc.err))
		if !strings.Contains(b.String(), "godrycrap: hint: "+tc.hint) {
			t.Errorf("%v: stderr = %q", tc.err, b.String())
		}
	}
	plain := errors.New("x")
	if profileError(plain) != plain {
		t.Error("profileError changed an unrelated error")
	}
}

func TestInternalErrorGetsTheBugHint(t *testing.T) {
	stubBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v9.8.7"}}, true)
	cmdline = []string{"crap", "-coverprofile", "it's.out", "./pkg"}
	var b strings.Builder
	if fail(&b, "crap", errors.New("bad header \"x\"\nsecond line")) != gate.Error {
		t.Fatal("fail did not return gate.Error")
	}
	got := b.String()
	for _, want := range []string{
		"godrycrap crap: bad header",
		"godrycrap: internal error. Report it: gh issue create -R tfournet/GoDryCrap --title 'crap: bad header \"x\"' --body 'godrycrap v9.8.7, go",
		`command: godrycrap crap -coverprofile '\''it'\''\'\'''\''s.out'\'' pkg'`,
		"godrycrap: or open https://github.com/tfournet/GoDryCrap/issues",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "hint:") {
		t.Errorf("an internal error got a user hint:\n%s", got)
	}
}

func TestPanicIsRecoveredWithTheBugHint(t *testing.T) {
	commands["boom"] = func([]string, io.Writer, io.Writer) gate.Status { panic("kaboom") }
	t.Cleanup(func() { delete(commands, "boom") })
	res := runIn(t, "boom", "-x")
	if res.status != gate.Error || !strings.Contains(res.stderr, "godrycrap boom: panic: kaboom") ||
		!strings.Contains(res.stderr, "--title 'boom: panic: kaboom'") || !strings.Contains(res.stderr, "command: godrycrap boom -x'") {
		t.Errorf("status %v, stderr:\n%s", res.status, res.stderr)
	}
}

func TestBugHintStripsDirectories(t *testing.T) {
	args := []string{"crap", "-coverprofile", "/home/dev/secret-proj/coverage.out", "-C=/srv/repos/acme", "./internal/billing", ".", "-summary", "--", "../x/y.go", "/"}
	got := strings.Join(redactArgs(args), " ")
	if want := "crap -coverprofile coverage.out -C=acme billing . -summary -- y.go /"; got != want {
		t.Errorf("redactArgs = %q, want %q", got, want)
	}
	msg := `open /home/dev/secret-proj/c.out: denied; parse internal/billing/tax.go:3:1: bad; git cat-file HEAD:pkg/a.go; C:\work\p\q.go, dir/`
	if got, want := stripDirs(msg), "open c.out: denied; parse tax.go:3:1: bad; git cat-file HEAD:a.go; q.go, dir"; got != want {
		t.Errorf("stripDirs = %q, want %q", got, want)
	}
	hint := bugHint("crap", "open /home/dev/secret-proj/c.out: denied", args)
	if strings.Contains(hint, "secret-proj") || strings.Contains(hint, "/srv") || !strings.Contains(hint, "--title 'crap: open c.out: denied'") {
		t.Errorf("hint leaks directories:\n%s", hint)
	}
}

func TestShortErrorAndQuoting(t *testing.T) {
	long := strings.Repeat("é", 100)
	if got := shortError(long); len([]rune(got)) != 80 || !strings.HasSuffix(got, "...") {
		t.Errorf("shortError = %q", got)
	}
	if got := shellJoin([]string{"a", "", "b c", "x=1,y@z"}); got != "a '' 'b c' x=1,y@z" {
		t.Errorf("shellJoin = %q", got)
	}
}

func TestNoHintOnSuccess(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"a.go": "package m\n\n" + fn("A", 9)})
	res := r.run("complexity")
	if res.status != gate.Warn || strings.Contains(res.stderr, "hint") || strings.Contains(res.stderr, "internal error") {
		t.Errorf("status %v, stderr:\n%s", res.status, res.stderr)
	}
}
