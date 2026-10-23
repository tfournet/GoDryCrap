package coverlayout

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

func parse(t *testing.T, name string, src []byte) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	return fset, f
}

func format(b Block) string {
	return fmt.Sprintf("%d.%d,%d.%d %d", b.Start.Line, b.Start.Col, b.End.Line, b.End.Col, b.Statements)
}

// TestMatchesGoCover compares the computed layout of testdata/coverlayout
// with the profile go1.27.3 wrote for it.
func TestMatchesGoCover(t *testing.T) {
	src, err := os.ReadFile("../../testdata/coverlayout/edge/edge.go")
	if err != nil {
		t.Fatal(err)
	}
	fset, file := parse(t, "edge.go", src)
	layout, err := File(fset, file, src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range layout.All {
		got = append(got, format(b))
	}
	want := profileBlocks(t, "../../testdata/coverlayout/cover.out", "example.com/layout/edge/edge.go")
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("computed blocks differ from go cover's\n got %d: %v\nwant %d: %v", len(got), got, len(want), want)
	}
}

func profileBlocks(t *testing.T, path, key string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		rest, ok := strings.CutPrefix(sc.Text(), key+":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		out = append(out, fields[0]+" "+fields[1])
	}
	sort.Strings(out)
	return out
}

func TestFuncsGroupBlocks(t *testing.T) {
	src := []byte("package p\n\nvar G = func() int { return 1 }\n\nfunc F(x int) int {\n\tf := func() {\n\t\tx++\n\t}\n\tf()\n\treturn x\n}\n\nfunc _() { println() }\n\nfunc Declared()\n")
	fset, file := parse(t, "a.go", src)
	layout, err := File(fset, file, src)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if d, ok := d.(*ast.FuncDecl); ok && d.Name.Name == "F" {
			fn = d
		}
	}
	if got := len(layout.Funcs[fn]); got != 3 {
		t.Errorf("F has %d blocks, want 3 (its literal's counted with it)", got)
	}
	if len(layout.Funcs) != 1 || len(layout.All) != 4 {
		t.Errorf("funcs = %d, all = %d; want only F, and G's literal in All", len(layout.Funcs), len(layout.All))
	}
}

func TestLayoutLeavesTreeUnchanged(t *testing.T) {
	src := []byte("package p\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t} else if x < 0 {\n\t\treturn -1\n\t} else {\n\t\treturn 0\n\t}\n}\n")
	fset, file := parse(t, "a.go", src)
	var before, after strings.Builder
	printer.Fprint(&before, fset, file)
	if _, err := File(fset, file, src); err != nil {
		t.Fatal(err)
	}
	printer.Fprint(&after, fset, file)
	if before.String() != after.String() {
		t.Errorf("File changed the tree:\n%s", after.String())
	}
}

func TestDedupMovesRepeatedSpan(t *testing.T) {
	l := &layout{seen: map[[2]Pos]bool{}}
	blocks := []Block{}
	l.cur = &blocks
	fset := token.NewFileSet()
	f := fset.AddFile("a.go", -1, 10)
	f.SetLines([]int{0})
	l.fset = fset
	l.newCounter(f.Pos(2), f.Pos(2), 0)
	l.newCounter(f.Pos(2), f.Pos(2), 0)
	if blocks[1].End.Col != blocks[0].End.Col+1 {
		t.Errorf("blocks = %v, want the repeat's end moved one column", blocks)
	}
}

func TestFindText(t *testing.T) {
	src := []byte("x /* else */ // else\n else")
	fset := token.NewFileSet()
	f := fset.AddFile("a.go", -1, len(src))
	l := &layout{fset: fset, src: src}
	if got := l.findText(f.Pos(0), "else"); got != len(src)-4 {
		t.Errorf("findText = %d, want %d (comments skipped)", got, len(src)-4)
	}
	if got := l.findText(f.Pos(0), "nope"); got != -1 {
		t.Errorf("findText = %d, want -1", got)
	}
	open := []byte("x /* else")
	fset = token.NewFileSet()
	g := fset.AddFile("b.go", -1, len(open))
	l = &layout{fset: fset, src: open}
	if got := l.findText(g.Pos(0), "else"); got != 0 {
		t.Errorf("findText in an open comment = %d, want 0 (cover's result)", got)
	}
}
