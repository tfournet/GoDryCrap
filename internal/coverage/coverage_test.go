package coverage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tfournet/GoDryCrap/internal/analysis"
)

func TestCRAP(t *testing.T) {
	cases := []struct {
		cc   int
		pct  float64
		want float64
	}{
		{1, 100, 1},
		{1, 0, 2},
		{9, 100, 9},
		{3, 0, 12},
		{4, 50, 6},
		{10, 75, 11.5625},
	}
	for _, tc := range cases {
		if got := CRAP(tc.cc, tc.pct); got != tc.want {
			t.Errorf("CRAP(%d, %v) = %v, want %v", tc.cc, tc.pct, got, tc.want)
		}
	}
}

func TestRound4(t *testing.T) {
	cases := map[float64]float64{100.0 / 3: 33.3333, 2.0 / 3: 0.6667, 1.00005: 1.0001, 5.66665: 5.6666, 12: 12}
	for in, want := range cases {
		if got := Round4(in); got != want {
			t.Errorf("Round4(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestPercentOverlapsByLine(t *testing.T) {
	blocks := []Block{
		{StartLine: 1, EndLine: 3, Statements: 2, Count: 1},
		{StartLine: 3, EndLine: 6, Statements: 1, Count: 0},
		{StartLine: 7, EndLine: 9, Statements: 4, Count: 5},
	}
	cases := []struct {
		start, end int
		want       float64
	}{
		{1, 2, 100},
		{3, 5, 100.0 * 2 / 3},
		{4, 6, 0},
		{10, 12, 0},
		{1, 9, 100.0 * 6 / 7},
	}
	for _, tc := range cases {
		if got := Percent(blocks, tc.start, tc.end); got != tc.want {
			t.Errorf("Percent(%d, %d) = %v, want %v", tc.start, tc.end, got, tc.want)
		}
	}
}

func load(t *testing.T, contents ...string) (*Profile, error) {
	t.Helper()
	var paths []string
	for i, c := range contents {
		p := filepath.Join(t.TempDir(), "p"+string(rune('a'+i)))
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return Load(paths)
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(nil); err == nil {
		t.Error("no error without a profile")
	}
	if _, err := load(t, "mode: set\n"); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty profile: %v", err)
	}
	if _, err := load(t, "mode: set\na.go:1.1,2.1 1 1\n", ""); !errors.Is(err, ErrEmpty) {
		t.Errorf("second profile empty: %v", err)
	}
	if _, err := load(t, "mode: set\n:1.1,2.1 1 1\n"); err == nil {
		t.Error("no error for a record without a file name")
	}
}

func TestLoadMergesHighestCount(t *testing.T) {
	p, err := load(t, "mode: count\nm/a.go:1.1,2.1 1 0\nm/a.go:3.1,4.1 1 2\n", "mode: count\nm/a.go:1.1,2.1 1 3\nm/a.go:3.1,4.1 1 0\n")
	if err != nil {
		t.Fatal(err)
	}
	blocks := p.Files["m/a.go"]
	if len(blocks) != 2 || blocks[0].Count != 3 || blocks[1].Count != 2 {
		t.Errorf("blocks = %+v", blocks)
	}
}

func TestBlocksKeys(t *testing.T) {
	p := &Profile{Files: map[string][]Block{
		"example.com/m/p/a.go":        {{EndLine: 1}},
		"other.com/x/q/b.go":          {{EndLine: 2}},
		"one.com/r/c.go":              {{EndLine: 3}},
		"two.com/r/c.go":              {{EndLine: 4}},
		"github.com/long/a/b/main.go": {{EndLine: 5}},
	}}
	cases := []struct {
		file analysis.File
		want int
		ok   bool
	}{
		{analysis.File{Path: "p/a.go", Package: "example.com/m/p", InModule: true}, 1, true},
		{analysis.File{Path: "q/b.go", Package: "q"}, 2, true},
		{analysis.File{Path: "../q/b.go", Package: "q"}, 2, true},
		{analysis.File{Path: "r/c.go", Package: "r"}, 0, false},
		{analysis.File{Path: "b/main.go", Package: "main"}, 5, true},
		{analysis.File{Path: "z/main.go", Package: "main"}, 0, false},
	}
	for _, tc := range cases {
		blocks, ok := p.Blocks(&tc.file)
		if ok != tc.ok || ok && blocks[0].EndLine != tc.want {
			t.Errorf("Blocks(%s) = %v %v, want %d %v", tc.file.Path, blocks, ok, tc.want, tc.ok)
		}
	}
}

func TestLoadRejectsMalformedProfiles(t *testing.T) {
	cases := map[string]string{
		"no header":              "m/a.go:1.1,2.1 1 1\n",
		"unknown mode":           "mode: sometimes\nm/a.go:1.1,2.1 1 1\n",
		"zero line":              "mode: set\nm/a.go:0.0,0.0 1 1\n",
		"zero column":            "mode: set\nm/a.go:1.0,2.1 1 1\n",
		"reversed lines":         "mode: set\nm/a.go:5.1,2.1 1 1\n",
		"reversed columns":       "mode: set\nm/a.go:2.9,2.1 1 1\n",
		"negative statements":    "mode: set\nm/a.go:1.1,2.1 -1 1\n",
		"negative count":         "mode: set\nm/a.go:1.1,2.1 1 -1\n",
		"trailing garbage":       "mode: set\nm/a.go:1.1,2.1 1 1 extra\n",
		"span garbage":           "mode: set\nm/a.go:1.1x,2.1 1 1\n",
		"no file":                "mode: set\n:1.1,2.1 1 1\n",
		"overflow":               "mode: set\nm/a.go:1.1,2.1 1 99999999999999999999\n",
		"inconsistent duplicate": "mode: set\nm/a.go:1.1,2.1 1 1\nm/a.go:1.1,2.1 2 1\n",
	}
	for name, content := range cases {
		if _, err := load(t, content); err == nil || errors.Is(err, ErrEmpty) {
			t.Errorf("%s: err = %v, want a parse error", name, err)
		}
	}
}

func TestLoadAcceptsWellFormedProfiles(t *testing.T) {
	content := "mode: atomic\r\nm/a.go:3.15,3.15 0 0\r\n\nmode: atomic\nm/a.go:6.2,6.11 1 4\n"
	p, err := load(t, content)
	if err != nil {
		t.Fatal(err)
	}
	if blocks := p.Files["m/a.go"]; len(blocks) != 2 || blocks[0].Statements != 0 || blocks[1].Count != 4 {
		t.Errorf("blocks = %+v", blocks)
	}
}

func TestModuleFileNeverFallsBackToAnotherImportPath(t *testing.T) {
	p := &Profile{Files: map[string][]Block{
		"other.com/x/p/a.go":   {{EndLine: 1}},
		"example.com/m/p/a.go": {{EndLine: 2}},
	}}
	root := analysis.File{Path: "a.go", Package: "example.com/m", InModule: true}
	if blocks, ok := p.Blocks(&root); ok {
		t.Errorf("root a.go took a nested package's blocks: %v", blocks)
	}
	wrong := analysis.File{Path: "p/a.go", Package: "example.com/q/p", InModule: true}
	if blocks, ok := p.Blocks(&wrong); ok {
		t.Errorf("p/a.go in module example.com/q took another module's blocks: %v", blocks)
	}
}

func TestLongerSourceOwnsSuffixMatch(t *testing.T) {
	p := &Profile{Files: map[string][]Block{"x/p/a.go": {{EndLine: 1}}}}
	short := &analysis.File{Path: "a.go", Package: "main"}
	long := &analysis.File{Path: "p/a.go", Package: "p"}
	if _, ok := p.Blocks(short); !ok {
		t.Error("unbound a.go should match x/p/a.go by suffix")
	}
	p.Bind([]*analysis.File{short, long})
	if _, ok := p.Blocks(short); ok {
		t.Error("a.go took x/p/a.go although p/a.go is also scored")
	}
	if _, ok := p.Blocks(long); !ok {
		t.Error("p/a.go lost its own key")
	}
}
