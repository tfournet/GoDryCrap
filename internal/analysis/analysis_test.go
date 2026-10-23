package analysis

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func parse(t *testing.T, path, src string, mods Modules) *File {
	t.Helper()
	file, err := ParseFile(path, []byte(src), mods)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestComplexityCountsCrapperDecisions(t *testing.T) {
	cases := map[string]struct {
		body string
		want int
	}{
		"straight":     {`return`, 1},
		"if else if":   {`if a { } else if b { } else { }`, 3},
		"for":          {`for a { }`, 2},
		"range":        {`for range 3 { }`, 2},
		"and or":       {`_ = a && b || a`, 3},
		"switch":       {`switch { case a: case b: default: }`, 4},
		"type switch":  {`var x any; switch x.(type) { case int, string: default: }`, 3},
		"select":       {`var c chan int; select { case <-c: default: }`, 3},
		"func literal": {`f := func() { if a { } }; f()`, 2},
		"bitwise":      {`_ = 1 & 2 | 3`, 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			file := parse(t, "a.go", "package p\nfunc F(a, b bool) {\n"+tc.body+"\n}\n", nil)
			if got := file.Funcs[0].CC; got != tc.want {
				t.Errorf("CC = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestThresholdFixtures(t *testing.T) {
	for name, want := range map[string]int{"CC9": 9, "CC10": 10, "CC11": 11} {
		src, err := os.ReadFile("../../testdata/complexity/" + strings.ToLower(name) + ".go")
		if err != nil {
			t.Fatal(err)
		}
		fn := parse(t, "a.go", string(src), nil).Funcs[0]
		if fn.Name != name || fn.CC != want {
			t.Errorf("%s: CC = %d, want %d", fn.Name, fn.CC, want)
		}
	}
}

func TestReceiversAndNamespaces(t *testing.T) {
	src, err := os.ReadFile("../../testdata/receivers/receivers.go")
	if err != nil {
		t.Fatal(err)
	}
	mods := Modules{".": "example.com/m"}
	file := parse(t, "pkg/receivers.go", string(src), mods)
	want := []struct{ ns, name, id string }{
		{"example.com/m/pkg.Value", "Get", "example.com/m/pkg.Value.Get"},
		{"example.com/m/pkg.Value", "Set", "example.com/m/pkg.Value.Set"},
		{"example.com/m/pkg.Stack", "Push", "example.com/m/pkg.Stack.Push"},
		{"example.com/m/pkg.Pair", "Key", "example.com/m/pkg.Pair.Key"},
		{"example.com/m/pkg.Value", "Kind", "example.com/m/pkg.Value.Kind"},
		{"example.com/m/pkg", "init", "example.com/m/pkg.init"},
		{"example.com/m/pkg", "init", "example.com/m/pkg.init"},
	}
	if len(file.Funcs) != len(want) {
		t.Fatalf("got %d functions, want %d (bodyless declarations are skipped)", len(file.Funcs), len(want))
	}
	for i, w := range want {
		fn := file.Funcs[i]
		if fn.Namespace != w.ns || fn.Name != w.name || fn.ID != w.id {
			t.Errorf("func %d = %s %s %s, want %s %s %s", i, fn.Namespace, fn.Name, fn.ID, w.ns, w.name, w.id)
		}
	}
}

func TestParenthesizedReceiver(t *testing.T) {
	file := parse(t, "a.go", "package p\ntype T struct{}\nfunc (t (*T)) M() {}\n", nil)
	if got := file.Funcs[0].Namespace; got != "p.T" {
		t.Errorf("namespace = %q, want p.T", got)
	}
}

func TestGeneratedHeaderSkipped(t *testing.T) {
	src, err := os.ReadFile("../../testdata/generated/header.go")
	if err != nil {
		t.Fatal(err)
	}
	if file := parse(t, "header.go", string(src), nil); file != nil {
		t.Errorf("generated file was parsed into %d functions", len(file.Funcs))
	}
}

func TestParseError(t *testing.T) {
	if _, err := ParseFile("bad.go", []byte("package p\nfunc {"), nil); err == nil {
		t.Error("no error for invalid source")
	}
}

func TestSkipPath(t *testing.T) {
	cases := map[string]bool{
		"a.go":              false,
		"pkg/a.go":          false,
		"a_test.go":         true,
		"README.md":         true,
		"ent/schema/a.go":   true,
		"pkg/gen/a.go":      true,
		"vendor/x/a.go":     true,
		"pkg/testdata/a.go": true,
		"pkg/agent/a.go":    false,
		"pkg/generic.go":    false,
	}
	for path, want := range cases {
		if got := SkipPath(path); got != want {
			t.Errorf("SkipPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestModulePath(t *testing.T) {
	cases := map[string]string{
		"module example.com/m\n\ngo 1.27\n":     "example.com/m",
		"// c\nmodule \"example.com/q\" // x\n": "example.com/q",
		"go 1.27\n":                             "",
		"modules x\n":                           "",
	}
	for src, want := range cases {
		if got := ModulePath([]byte(src)); got != want {
			t.Errorf("ModulePath(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestImportPath(t *testing.T) {
	mods := Modules{".": "example.com/root", "tools": "example.com/tools", "/abs/mod": "example.com/abs"}
	cases := []struct {
		dir, want string
		ok        bool
	}{
		{".", "example.com/root", true},
		{"a/b", "example.com/root/a/b", true},
		{"tools", "example.com/tools", true},
		{"tools/x", "example.com/tools/x", true},
		{"/abs/mod/p", "example.com/abs/p", true},
		{"/elsewhere", "pkgname", false},
	}
	for _, tc := range cases {
		got, ok := mods.ImportPath(tc.dir, "pkgname")
		if got != tc.want || ok != tc.ok {
			t.Errorf("ImportPath(%q) = %q %v, want %q %v", tc.dir, got, ok, tc.want, tc.ok)
		}
	}
}

func TestChanges(t *testing.T) {
	mods := Modules{".": "m"}
	before := []*File{
		parse(t, "a.go", "package p\nfunc Same() {}\nfunc Edited() {}\nfunc Moved() {}\nfunc Old() {}\nfunc Gone() {}\n", mods),
	}
	after := []*File{
		parse(t, "a.go", "package p\nfunc Same() {}\nfunc Edited() { _ = 1 }\nfunc New() {}\n", mods),
		parse(t, "b.go", "package p\nfunc Moved() {}\n", mods),
	}
	got := map[string]bool{}
	for _, c := range Changes(before, after) {
		got[c.After.Name] = c.Before != nil
	}
	want := map[string]bool{"Edited": true, "New": false, "Moved": true}
	if len(got) != len(want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	for name, hasBefore := range want {
		if v, ok := got[name]; !ok || v != hasBefore {
			t.Errorf("%s: reported %v with before %v, want before %v", name, ok, v, hasBefore)
		}
	}
}

// initFn is an init function with n-1 if statements (complexity n); tag
// makes its text distinct.
func initFn(n, tag int) string {
	src := "func init() {\n\tx := " + strconv.Itoa(tag) + "\n"
	for i := 1; i < n; i++ {
		src += "\tif x > 0 {\n\t\tx--\n\t}\n"
	}
	return src + "\t_ = x\n}\n"
}

type changeView struct {
	file       string
	cc         int
	beforeFile string
}

func changes(t *testing.T, before, after map[string]string) []changeView {
	t.Helper()
	mods := Modules{".": "m"}
	load := func(files map[string]string) []*File {
		var out []*File
		for path, src := range files {
			out = append(out, parse(t, path, src, mods))
		}
		return out
	}
	var out []changeView
	for _, c := range Changes(load(before), load(after)) {
		v := changeView{file: c.After.File, cc: c.After.CC}
		if c.Before != nil {
			v.beforeFile = c.Before.File
		}
		out = append(out, v)
	}
	return out
}

func TestRepeatedInitsMatchByContent(t *testing.T) {
	survivor, other, added := initFn(12, 1), initFn(2, 2), initFn(3, 3)
	cases := []struct {
		name          string
		before, after map[string]string
		want          []changeView
	}{
		{"delete the second", map[string]string{"a.go": "package p\n" + survivor + other}, map[string]string{"a.go": "package p\n" + survivor}, nil},
		{"delete the first", map[string]string{"a.go": "package p\n" + other + survivor}, map[string]string{"a.go": "package p\n" + survivor}, nil},
		{"insert before", map[string]string{"a.go": "package p\n" + survivor}, map[string]string{"a.go": "package p\n" + added + survivor}, []changeView{{"a.go", 3, ""}}},
		{"insert after", map[string]string{"a.go": "package p\n" + survivor + other}, map[string]string{"a.go": "package p\n" + survivor + other + added}, []changeView{{"a.go", 3, ""}}},
		{"reorder", map[string]string{"a.go": "package p\n" + survivor + other}, map[string]string{"a.go": "package p\n" + other + survivor}, nil},
		{"edit one", map[string]string{"a.go": "package p\n" + survivor + other}, map[string]string{"a.go": "package p\n" + survivor + initFn(4, 2)}, []changeView{{"a.go", 4, "a.go"}}},
		{"move one", map[string]string{"a.go": "package p\n" + survivor + other}, map[string]string{"a.go": "package p\n" + survivor, "b.go": "package p\n" + other}, []changeView{{"b.go", 2, "a.go"}}},
		{"edit and move the only leftover", map[string]string{"a.go": "package p\n" + survivor + other}, map[string]string{"a.go": "package p\n" + survivor, "b.go": "package p\n" + initFn(5, 9)}, []changeView{{"b.go", 5, "a.go"}}},
		{"two edits are ambiguous", map[string]string{"a.go": "package p\n" + survivor + other}, map[string]string{"a.go": "package p\n" + initFn(6, 7) + initFn(7, 8)}, []changeView{{"a.go", 6, ""}, {"a.go", 7, ""}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := changes(t, tc.before, tc.after)
			if len(got) != len(tc.want) {
				t.Fatalf("changes = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("change %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
