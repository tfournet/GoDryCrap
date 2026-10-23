package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListPathsGroupsByDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"a/a.go", "a/x.go", "a/b/z.go", "c/w.go"} {
		write(t, filepath.Join(dir, p), "package p\nfunc F() {}\n")
	}
	paths, err := ListPaths([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int
	for _, pkg := range paths.Packages {
		sizes = append(sizes, len(pkg))
	}
	if len(sizes) != 3 || sizes[0] != 2 || sizes[1] != 1 || sizes[2] != 1 {
		t.Errorf("package sizes = %v, want [2 1 1] (a, a/b, c)", sizes)
	}
}

func TestLoadPathsDropsSyntax(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package p\nfunc F() { _ = 1 }\n")
	files, err := LoadPaths([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	f := files[0]
	fn := f.Funcs[0]
	if f.AST != nil || f.Fset != nil || fn.Decl != nil || fn.Text != nil {
		t.Error("LoadPaths kept syntax trees or source text")
	}
	if fn.CC != 1 || len(fn.Blocks) != 1 || len(f.Layout) != 1 {
		t.Errorf("LoadPaths lost scoring data: %+v", fn)
	}
	paths, err := ListPaths([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := paths.Load(paths.Packages[0], true)
	if err != nil || kept[0].Funcs[0].Decl == nil {
		t.Errorf("Load(keepSyntax) dropped the declaration: %v", err)
	}
}
