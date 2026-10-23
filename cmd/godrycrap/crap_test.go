package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tfournet/GoDryCrap/internal/analysis"
	"github.com/tfournet/GoDryCrap/internal/gate"
)

// profile writes a coverage profile with the given records and returns its
// path.
func (r *repo) profile(name string, records ...string) string {
	r.t.Helper()
	path := filepath.Join(r.t.TempDir(), name)
	content := "mode: set\n" + strings.Join(records, "\n") + "\n"
	writeFile(r.t, path, content)
	return path
}

// cover is the profile go cover would write for src at file in module
// example.com/m: a record per block of its computed layout, which
// internal/coverlayout's tests pin to real go1.27.3 output. hits gives the
// i-th block's count.
func cover(t *testing.T, file, src string, hits func(i int) int) []string {
	t.Helper()
	f, err := analysis.ParseFile(file, []byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	records := make([]string, len(f.Layout))
	for i, b := range f.Layout {
		records[i] = fmt.Sprintf("example.com/m/%s:%d.%d,%d.%d %d %d", file, b.Start.Line, b.Start.Col, b.End.Line, b.End.Col, b.Statements, hits(i))
	}
	return records
}

func hit(n int) func(int) int { return func(int) int { return n } }

func firstHit(k int) func(int) int {
	return func(i int) int {
		if i < k {
			return 1
		}
		return 0
	}
}

func join(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

func crapRows(t *testing.T, res result) map[string]crapRow {
	t.Helper()
	var payload struct {
		Functions []crapRow `json:"functions"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &payload); err != nil {
		t.Fatalf("%v\n%s%s", err, res.stdout, res.stderr)
	}
	rows := map[string]crapRow{}
	for _, row := range payload.Functions {
		rows[row.Name] = row
	}
	return rows
}

// fn(name, n) has 2n-1 blocks of one statement each.
func TestCrapScores(t *testing.T) {
	r := newRepo(t)
	srcA, srcB, srcC := "package p\n"+fn("Covered", 9), "package p\n"+fn("Half", 4), "package p\n"+fn("Bare", 3)
	r.stage(map[string]string{"p/a.go": srcA, "p/b.go": srcB, "p/c.go": srcC})
	prof := r.profile("cover.out", join(cover(t, "p/a.go", srcA, hit(1)), cover(t, "p/b.go", srcB, firstHit(4)), cover(t, "p/c.go", srcC, hit(0)))...)
	res := r.run("crap", "-coverprofile", prof, "-format", "json")
	res.want(t, gate.Fail)
	rows := crapRows(t, res)
	cases := map[string]struct {
		cov, crap float64
		status    string
	}{
		"Covered": {100, 9, "warn"},
		"Half":    {57.1429, 5.2595, "ok"},
		"Bare":    {0, 12, "fail"},
	}
	for name, w := range cases {
		got := rows[name]
		if got.Coverage != w.cov || got.CRAP != w.crap || got.Status != w.status {
			t.Errorf("%s = %+v, want coverage %v crap %v %s", name, got, w.cov, w.crap, w.status)
		}
	}
}

func TestCrapRoundsLikeCrapper(t *testing.T) {
	r := newRepo(t)
	src := "package p\n" + fn("F", 2)
	r.stage(map[string]string{"p/a.go": src})
	prof := r.profile("cover.out", cover(t, "p/a.go", src, firstHit(1))...)
	row := crapRows(t, r.run("crap", "-coverprofile", prof, "-format", "json"))["F"]
	if row.Coverage != 33.3333 || row.CRAP != 3.1852 {
		t.Errorf("row = %+v, want coverage 33.3333 crap 3.1852", row)
	}
}

func TestCrapMissingProfile(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"p/a.go": "package p\n" + fn("F", 2)})
	r.run("crap", "-coverprofile", filepath.Join(t.TempDir(), "nope.out")).want(t, gate.Error, "no such file")
	r.run("crap").want(t, gate.Error, "-coverprofile is required")
}

func TestCrapEmptyProfile(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"p/a.go": "package p\n" + fn("F", 2)})
	r.run("crap", "-coverprofile", r.profile("empty.out")).want(t, gate.Error, "no blocks")
	empty := filepath.Join(t.TempDir(), "zero.out")
	writeFile(t, empty, "")
	r.run("crap", "-coverprofile", empty).want(t, gate.Error, "no blocks")
}

func TestCrapMalformedProfile(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"p/a.go": "package p\n" + fn("F", 2)})
	for _, record := range []string{"garbage", "example.com/m/p/a.go:2.1,3.1 x 1", "example.com/m/p/a.go:2.1,3.1 1 x", "example.com/m/p/a.go:nope 1 1", "example.com/m/p/a.go:0.0,0.0 1 1"} {
		r.run("crap", "-coverprofile", r.profile("bad.out", record)).want(t, gate.Error, "line 2")
	}
}

func TestCrapWrongShardProfile(t *testing.T) {
	r := newRepo(t)
	srcQ := "package q\n" + fn("G", 2)
	r.stage(map[string]string{"p/a.go": "package p\n" + fn("F", 2), "q/b.go": srcQ})
	shard := r.profile("shard.out", append(cover(t, "q/b.go", srcQ, hit(1)), "example.com/m/z/z.go:1.1,2.1 1 1")...)
	r.run("crap", "-coverprofile", shard).want(t, gate.Error, "no blocks for this file: p/a.go")
	res := r.run("crap", "-coverprofile", shard, "-profiled-only", "-format", "tsv")
	res.want(t, gate.OK, "example.com/m/q.G")
	res.lacks(t, "p.F")
	other := r.profile("other.out", "example.com/m/z/z.go:1.1,2.1 1 1")
	r.run("crap", "-coverprofile", other, "-profiled-only").want(t, gate.Error, "none of the 2 changed files")
}

func TestCrapStaleProfile(t *testing.T) {
	r := newRepo(t)
	src := "package p\n" + fn("F", 2)
	r.stage(map[string]string{"p/a.go": src})
	r.run("crap", "-coverprofile", r.profile("eof.out", "example.com/m/p/a.go:3.2,99.2 2 1")).want(t, gate.Error, "does not match the source: p/a.go has a block 3.2,99.2 with 2 statements that go cover would not produce from this source")
	fresh := r.profile("fresh.out", cover(t, "p/a.go", src, hit(1))...)
	r.run("crap", "-coverprofile", fresh, "-format", "tsv").want(t, gate.OK, "ok\t2\t2\t100%\texample.com/m/p.F")

	// A function appended after the profile was made has no blocks.
	r.stage(map[string]string{"p/a.go": src + fn("G", 1)})
	r.run("crap", "-coverprofile", fresh).want(t, gate.Error, "does not match the source: example.com/m/p.G at p/a.go:8: the profile has 0 blocks for it, this source has 1")

	// A body shifted by an inserted line no longer lines up with its blocks.
	r.stage(map[string]string{"p/a.go": "package p\n\nvar v = 1\n" + fn("F", 2)})
	r.run("crap", "-coverprofile", fresh).want(t, gate.Error, "does not match the source: p/a.go has a block")
}

// TestCrapExpandedBodyKeepsFirstStatement is round 2's finding 1: a body
// grown with new branches, its first statement still at 3.2, must not pass
// on the old profile.
func TestCrapExpandedBodyKeepsFirstStatement(t *testing.T) {
	r := newRepo(t)
	before := "package p\nfunc F(x int) int {\n\treturn x\n}\n"
	r.stage(map[string]string{"p/a.go": before})
	old := r.profile("old.out", cover(t, "p/a.go", before, hit(1))...)
	r.run("crap", "-coverprofile", old, "-format", "tsv").want(t, gate.OK, "ok\t1\t1\t100%\texample.com/m/p.F")
	after := "package p\nfunc F(x int) int {\n\tif x > 0 {\n\t\tx--\n\t}\n\tif x > 1 {\n\t\tx--\n\t}\n\treturn x\n}\n"
	r.stage(map[string]string{"p/a.go": after})
	r.run("crap", "-coverprofile", old).want(t, gate.Error, "does not match the source")
}

func TestCrapBlockCountAndStatementMismatch(t *testing.T) {
	r := newRepo(t)
	src := "package p\n" + fn("F", 3)
	r.stage(map[string]string{"p/a.go": src})
	records := cover(t, "p/a.go", src, hit(1))
	r.run("crap", "-coverprofile", r.profile("short.out", records[1:]...)).want(t, gate.Error, "example.com/m/p.F at p/a.go:2: the profile has 4 blocks for it, this source has 5")
	wrong := append([]string{strings.Replace(records[0], " 1 1", " 2 1", 1)}, records[1:]...)
	r.run("crap", "-coverprofile", r.profile("stmts.out", wrong...)).want(t, gate.Error, "with 2 statements that go cover would not produce")
}

func TestCrapEmptyFunction(t *testing.T) {
	r := newRepo(t)
	srcP, srcQ := "package p\nfunc Empty() {}\n", "package q\n"+fn("G", 2)
	r.stage(map[string]string{"p/a.go": srcP, "q/b.go": srcQ})
	own := r.profile("own.out", join(cover(t, "p/a.go", srcP, hit(0)), cover(t, "q/b.go", srcQ, hit(1)))...)
	r.run("crap", "-coverprofile", own, "-format", "tsv").want(t, gate.OK, "ok\t2\t1\t0%\texample.com/m/p.Empty")
	unrelated := r.profile("unrelated.out", cover(t, "q/b.go", srcQ, hit(1))...)
	r.run("crap", "-coverprofile", unrelated).want(t, gate.Error, "no blocks for this file: p/a.go")
}

func TestCrapOnlyEmptyFunctionsChangedUnrelatedProfile(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"p/a.go": "package p\nfunc Empty() {}\n"})
	unrelated := r.profile("unrelated.out", cover(t, "z/z.go", "package z\n"+fn("Z", 2), hit(1))...)
	r.run("crap", "-coverprofile", unrelated).want(t, gate.Error, "no blocks for this file: p/a.go")
	r.run("crap", "-coverprofile", unrelated, "-profiled-only").want(t, gate.Error, "none of the 1 changed files")
}

func TestCrapMergesShards(t *testing.T) {
	r := newRepo(t)
	srcP, srcQ := "package p\n"+fn("F", 2), "package q\n"+fn("G", 2)
	r.stage(map[string]string{"p/a.go": srcP, "q/b.go": srcQ})
	one := r.profile("one.out", cover(t, "p/a.go", srcP, hit(0))...)
	two := r.profile("two.out", join(cover(t, "p/a.go", srcP, hit(1)), cover(t, "q/b.go", srcQ, hit(1)))...)
	rows := crapRows(t, r.run("crap", "-coverprofile", one, "-coverprofile", two, "-format", "json"))
	if rows["F"].Coverage != 100 || rows["G"].Coverage != 100 {
		t.Errorf("rows = %+v, want both fully covered", rows)
	}
}

func TestCrapNothingChangedStillChecksProfile(t *testing.T) {
	r := newRepo(t)
	r.run("crap", "-coverprofile", r.profile("empty.out")).want(t, gate.Error, "no blocks")
	r.run("crap", "-coverprofile", r.profile("ok.out", "example.com/m/p/a.go:1.1,2.1 1 1")).want(t, gate.OK, "0 functions")
}

func TestCrapRatchet(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"p/a.go": "package p\n" + fn("Legacy", 4) + fn("Worse", 4)})
	r.commit("base")
	src := "package p\n" + fn("Legacy", 4) + fn("Worse", 5)
	r.stage(map[string]string{"p/a.go": src})
	prof := r.profile("cover.out", cover(t, "p/a.go", src, hit(0))...)
	r.run("crap", "-coverprofile", prof).want(t, gate.Fail)
	res := r.run("crap", "-coverprofile", prof, "-ratchet", "-format", "json")
	res.want(t, gate.Fail)
	rows := crapRows(t, res)
	if rows["Worse"].Status != "fail" {
		t.Errorf("Worse = %+v, want fail (its CC rose)", rows["Worse"])
	}
	edited := "package p\n" + fn("Legacy", 4) + strings.Replace(fn("Worse", 4), "return x", "return x + 0", 1)
	r.stage(map[string]string{"p/a.go": edited})
	prof = r.profile("edited.out", cover(t, "p/a.go", edited, hit(0))...)
	res = r.run("crap", "-coverprofile", prof, "-ratchet", "-format", "json")
	res.want(t, gate.Warn)
	if row := crapRows(t, res)["Worse"]; !row.Ratchet || row.CRAP != 20 {
		t.Errorf("Worse = %+v, want ratcheted CRAP 20", row)
	}
}

func TestCrapDiffRangeAndPaths(t *testing.T) {
	r := newRepo(t)
	base := r.commit("base")
	src := "package p\n" + fn("F", 3)
	r.stage(map[string]string{"p/a.go": src})
	r.commit("head")
	prof := r.profile("cover.out", cover(t, "p/a.go", src, hit(1))...)
	r.run("crap", "-coverprofile", prof, "-diff", base+"..HEAD", "-format", "tsv").want(t, gate.OK, "ok\t3\t3\t100%\texample.com/m/p.F")
	res := runIn(t, "crap", "-coverprofile", prof, "-format", "tsv", filepath.Join(r.dir, "p"))
	res.want(t, gate.OK, "ok\t3\t3\t100%\texample.com/m/p.F")
}

func TestCrapMarkdown(t *testing.T) {
	r := newRepo(t)
	src := "package p\n" + fn("F", 3)
	r.stage(map[string]string{"p/a.go": src})
	prof := r.profile("cover.out", cover(t, "p/a.go", src, hit(0))...)
	r.run("crap", "-coverprofile", prof, "-format", "markdown").want(t, gate.Fail,
		"### godrycrap crap\n\n1 functions, 1 fail, 0 warn\n\n| status | crap | cc | coverage | function | location |\n| --- | --- | --- | --- | --- | --- |\n| fail | 12 | 3 | 0% | example.com/m/p.F | p/a.go:2 |\n")
}

func TestCrapWrongModuleProfile(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"p/a.go": "package p\n" + fn("F", 2)})
	prof := r.profile("other.out", "other.com/x/p/a.go:3.2,7.2 2 1")
	r.run("crap", "-coverprofile", prof).want(t, gate.Error, "no blocks for this file: p/a.go")
}

func TestCrapRootFileDoesNotTakeNestedCoverage(t *testing.T) {
	r := newRepo(t)
	nested := "package p\n" + fn("Nested", 2)
	r.stage(map[string]string{"a.go": "package m\n" + fn("Root", 2), "p/a.go": nested})
	prof := r.profile("nested.out", cover(t, "p/a.go", nested, hit(1))...)
	r.run("crap", "-coverprofile", prof).want(t, gate.Error, "no blocks for this file: a.go")
	res := r.run("crap", "-coverprofile", prof, "-profiled-only", "-format", "tsv")
	res.want(t, gate.OK, "example.com/m/p.Nested")
	res.lacks(t, "Root")
}

// TestCrapLineDirectives is round 3's finding 1: //line directives must not
// move a function's lines away from the physical lines coverage records.
func TestCrapLineDirectives(t *testing.T) {
	before := "package p\n//line a.go:100\n" + fn("F", 3)
	inside := "package p\n" + strings.Replace(fn("G", 3), "\treturn x", "//line a.go:1\n\treturn x", 1)
	r := newRepo(t)
	r.stage(map[string]string{"p/a.go": before, "q/a.go": inside})
	prof := r.profile("fresh.out", join(cover(t, "p/a.go", before, hit(1)), cover(t, "q/a.go", inside, hit(1)))...)
	res := r.run("crap", "-coverprofile", prof, "-format", "json")
	res.want(t, gate.OK)
	rows := crapRows(t, res)
	if f := rows["F"]; f.Coverage != 100 || f.CRAP != 3 || f.Line != 3 || f.EndLine != 11 {
		t.Errorf("F = %+v, want 100%% coverage, CRAP 3, physical lines 3-11", f)
	}
	if g := rows["G"]; g.Coverage != 100 || g.CRAP != 3 || g.Line != 2 || g.EndLine != 11 {
		t.Errorf("G = %+v, want 100%% coverage, CRAP 3, physical lines 2-11", g)
	}
	r.run("complexity", "-format", "tsv").want(t, gate.OK, "example.com/m/p.F\tp/a.go:3", "example.com/m/q.G\tq/a.go:2")
}
