package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tfournet/GoDryCrap/internal/gate"
)

func nearFixture(t *testing.T) (copyA, copyB, other string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "dupes", "near.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	pick := func(name string) string {
		start := strings.Index(src, "func "+name+"(")
		end := strings.Index(src[start:], "\n}\n") + start + 3
		return src[start:end]
	}
	return pick("SumPositive"), pick("TotalAboveZero"), pick("JoinUpper")
}

const nearImports = "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nvar _ = fmt.Sprint\nvar _ = strings.ToUpper\n\n"

func TestDupesStagedNearCopyInPackage(t *testing.T) {
	a, b, other := nearFixture(t)
	r := newRepo(t)
	r.stage(map[string]string{"p/old.go": nearImports + a + "\n" + other})
	r.commit("base")
	r.stage(map[string]string{"p/new.go": nearImports + b})
	res := r.run("dupes", "-format", "tsv")
	res.want(t, gate.OK, "1.0000\texample.com/m/p.TotalAboveZero\tp/new.go:", "example.com/m/p.SumPositive\tp/old.go:")
	res.lacks(t, "JoinUpper")
}

func TestDupesOnlyComparesWithinPackage(t *testing.T) {
	a, b, _ := nearFixture(t)
	r := newRepo(t)
	r.stage(map[string]string{"q/old.go": strings.Replace(nearImports, "package p", "package q", 1) + a})
	r.commit("base")
	r.stage(map[string]string{"p/new.go": nearImports + b})
	r.run("dupes").want(t, gate.OK, "0 candidate pairs")
}

func TestDupesUnchangedPairsNotReported(t *testing.T) {
	a, b, _ := nearFixture(t)
	r := newRepo(t)
	r.stage(map[string]string{"p/old.go": nearImports + a + "\n" + b})
	r.commit("base")
	r.stage(map[string]string{"p/other.go": "package p\n" + fn("Unrelated", 2)})
	r.run("dupes").want(t, gate.OK, "0 candidate pairs")
}

func TestDupesBothChangedReportedOnce(t *testing.T) {
	a, b, _ := nearFixture(t)
	r := newRepo(t)
	r.stage(map[string]string{"p/new.go": nearImports + a + "\n" + b})
	res := r.run("dupes", "-format", "json")
	res.want(t, gate.OK)
	var payload struct {
		Pairs []dupeRow `json:"pairs"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Pairs) != 1 || payload.Pairs[0].Score != 1 || payload.Pairs[0].Left.Nodes < 20 {
		t.Errorf("pairs = %+v, want one pair scoring 1", payload.Pairs)
	}
}

func TestDupesThresholdAndFilters(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "dupes", "near.go")
	runIn(t, "dupes", path).want(t, gate.OK, "1 candidate pairs", "SumPositive", "../../testdata/dupes/near.go:9-18")
	runIn(t, "dupes", "-threshold", "0", path).want(t, gate.OK, "3 candidate pairs")
	runIn(t, "dupes", "-threshold", "0", "-min-lines", "1", "-min-nodes", "1", path).want(t, gate.OK, "6 candidate pairs")
	runIn(t, "dupes", "-min-nodes", "1000", path).want(t, gate.OK, "0 candidate pairs")
}

func TestDupesDiffRangeAndMarkdown(t *testing.T) {
	a, b, _ := nearFixture(t)
	r := newRepo(t)
	base := r.commit("base")
	r.stage(map[string]string{"p/new.go": nearImports + a + "\n" + b})
	r.commit("head")
	res := r.run("dupes", "-diff", base+"..HEAD", "-format", "markdown")
	res.want(t, gate.OK, "### godrycrap dupes\n\n1 candidate pairs at or above 0.8 (advisory)\n\n| score | function | location | similar to | location |\n", "| 1.0000 | example.com/m/p.SumPositive | p/new.go:")
}

func TestDupesErrors(t *testing.T) {
	r := newRepo(t)
	r.stage(map[string]string{"p/a.go": "package p\nfunc {"})
	r.run("dupes").want(t, gate.Error, "parse p/a.go")
	runIn(t, "dupes", "-threshold", "x").want(t, gate.Error)
	runIn(t, "dupes", "-C", t.TempDir()).want(t, gate.Error, "git")
}

func TestDupesPackageParseErrorIsError(t *testing.T) {
	a, _, _ := nearFixture(t)
	r := newRepo(t)
	r.stage(map[string]string{"p/broken.go": "package p\nfunc {"})
	r.git("commit", "-q", "--no-verify", "-m", "broken")
	r.stage(map[string]string{"p/new.go": nearImports + a})
	r.run("dupes").want(t, gate.Error, "parse p/broken.go")
}

func TestDupesRootPackage(t *testing.T) {
	a, b, _ := nearFixture(t)
	r := newRepo(t)
	r.stage(map[string]string{"old.go": nearImports + a})
	r.commit("base")
	r.stage(map[string]string{"new.go": nearImports + b})
	r.run("dupes").want(t, gate.OK, "1 candidate pairs", "example.com/m.TotalAboveZero")
	head := r.commit("head")
	r.run("dupes", "-diff", "HEAD~1.."+head).want(t, gate.OK, "1 candidate pairs")
}

func TestDupesPathModeComparesPerPackage(t *testing.T) {
	a, b, _ := nearFixture(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n")
	writeFile(t, filepath.Join(dir, "p", "a.go"), nearImports+a)
	writeFile(t, filepath.Join(dir, "q", "b.go"), strings.Replace(nearImports, "package p", "package q", 1)+b)
	runIn(t, "dupes", dir).want(t, gate.OK, "0 candidate pairs")
	writeFile(t, filepath.Join(dir, "q", "c.go"), strings.Replace(nearImports, "package p", "package q", 1)+a)
	res := runIn(t, "dupes", "-format", "tsv", dir)
	res.want(t, gate.OK, "example.com/m/q.SumPositive", "example.com/m/q.TotalAboveZero")
	res.lacks(t, "example.com/m/p.")
}
