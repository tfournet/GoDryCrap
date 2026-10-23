package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tfournet/GoDryCrap/internal/gate"
)

// test_import_path_comes_from_go_mod, from crapper's tests/test_go.py: the
// namespace is the go.mod module path plus the package directory.
func TestCrapperImportPathComesFromGoMod(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "crapper", "demo")
	res := runIn(t, "complexity", "-format", "tsv", dir)
	res.want(t, gate.OK, "ok\t3\tnew\tgithub.com/acme/demo/internal/game.Board.Place\t")
	if got := len(strings.Split(strings.TrimSpace(res.stdout), "\n")); got != 2 {
		t.Errorf("rows = %d, want a header and one function\n%s", got, res.stdout)
	}
}

// TestCrapperCRAPOnSameProfile scores crapper's Go cases with the profile
// in testdata/crapper/coverage. The expected rows are crapper's output on
// that profile (crapper --use-existing-coverage; see docs/parity.md).
func TestCrapperCRAPOnSameProfile(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "crapper", "coverage")
	res := runIn(t, "crap", "-format", "json", "-coverprofile", filepath.Join(dir, "coverage.out"), dir)
	res.want(t, gate.OK)
	rows := crapRows(t, res)
	want := map[string]struct {
		namespace string
		cc        int
		coverage  float64
		crap      float64
	}{
		"Branchy": {"github.com/acme/demo/sample", 5, 80, 5.2},
		"Place":   {"github.com/acme/demo/internal/game.Board", 3, 100, 3},
		"Run":     {"github.com/acme/demo/sample.Widget", 3, 100, 3},
		"Simple":  {"github.com/acme/demo/sample", 1, 0, 2},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v, want crapper's four", rows)
	}
	for name, w := range want {
		got := rows[name]
		if got.Namespace != w.namespace || got.CC != w.cc || got.Coverage != w.coverage || got.CRAP != w.crap {
			t.Errorf("%s = %+v, want crapper's %+v", name, got, w)
		}
	}
}
