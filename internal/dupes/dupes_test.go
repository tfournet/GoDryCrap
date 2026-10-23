package dupes

import (
	"go/ast"
	"os"
	"strings"
	"testing"

	"github.com/tfournet/GoDryCrap/internal/analysis"
)

// pr prints a normalized tree the way dryer's shape.pr does.
func pr(n *node) string {
	if !n.isList() {
		return n.atom
	}
	parts := make([]string, len(n.kids))
	for i, k := range n.kids {
		parts[i] = pr(k)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func parseFile(t *testing.T, path string, src []byte) *analysis.File {
	t.Helper()
	file, err := analysis.ParseFile(path, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func fixture(t *testing.T, name string) *analysis.File {
	t.Helper()
	src, err := os.ReadFile("../../testdata/dupes/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return parseFile(t, name, src)
}

func normalizeBody(t *testing.T, body string) string {
	t.Helper()
	file := parseFile(t, "a.go", []byte("package p\nfunc F(x int) {\n"+body+"\n}\n"))
	return pr(newNormalizer().Func(file.Funcs[0].Decl))
}

func TestNormalizeShapes(t *testing.T) {
	cases := map[string]string{
		`fmt.Println(x)`: `[:function_declaration :symbol [:parameter_list [:parameter_declaration :symbol :symbol]] [:block [:statement_list [:expression_statement [:call_expression [:selector_expression :symbol [:symbol "Println"]] [:argument_list :symbol]]]]]]`,
		`y := x + 1`:     `[:function_declaration :symbol [:parameter_list [:parameter_declaration :symbol :symbol]] [:block [:statement_list [:short_var_declaration [:expression_list :symbol] [:symbol ":="] [:expression_list [:binary_expression :symbol [:symbol "+"] :literal]]]]]]`,
		`_ = a.b`:        `[:function_declaration :symbol [:parameter_list [:parameter_declaration :symbol :symbol]] [:block [:statement_list [:assignment_statement [:expression_list :symbol] [:symbol "="] [:expression_list [:selector_expression :symbol :symbol]]]]]]`,
		``:               `[:function_declaration :symbol [:parameter_list [:parameter_declaration :symbol :symbol]] [:block]]`,
		`(f)(nil)`:       `[:function_declaration :symbol [:parameter_list [:parameter_declaration :symbol :symbol]] [:block [:statement_list [:expression_statement [:call_expression [:symbol "f"] [:argument_list :literal]]]]]]`,
		`<-ch`:           `[:function_declaration :symbol [:parameter_list [:parameter_declaration :symbol :symbol]] [:block [:statement_list [:expression_statement [:unary_expression :symbol]]]]]`,
	}
	for body, want := range cases {
		if got := normalizeBody(t, body); got != want {
			t.Errorf("%s:\n got %s\nwant %s", body, got, want)
		}
	}
}

func TestNormalizeMethod(t *testing.T) {
	file := parseFile(t, "a.go", []byte("package p\nfunc (r *T) M() (int, error) { return 0, nil }\n"))
	got := pr(newNormalizer().Func(file.Funcs[0].Decl))
	want := `[:method_declaration [:parameter_list [:parameter_declaration :symbol [:pointer_type [:symbol "*"] :symbol]]] :symbol [:parameter_list] [:parameter_list [:parameter_declaration :symbol] [:parameter_declaration :symbol]] [:block [:statement_list [:return_statement [:expression_list :literal :literal]]]]]`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestNormalizeEverySyntax walks a fixture with every statement and
// expression kind and checks that no supported node is dropped.
func TestNormalizeEverySyntax(t *testing.T) {
	file := fixture(t, "syntax.go")
	n := newNormalizer()
	printed := pr(n.Func(file.Funcs[0].Decl))
	ast.Inspect(file.Funcs[0].Decl, func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.Ellipsis, *ast.CaseClause, *ast.CommClause:
			// Normalized by the enclosing array type, switch or select.
		case ast.Expr:
			if n.expr(v, false) == nil {
				t.Errorf("expression %T normalized to nil", v)
			}
		case *ast.EmptyStmt:
		case ast.Stmt:
			if n.stmt(v) == nil {
				t.Errorf("statement %T normalized to nil", v)
			}
		}
		return true
	})
	for _, kind := range []string{
		":const_declaration", ":var_declaration", ":type_declaration", ":type_alias",
		":implicit_length_array_type", ":generic_type", ":keyed_element", ":if_statement",
		":for_clause", ":range_clause", ":labeled_statement", ":goto_statement",
		":fallthrough_statement", ":expression_switch_statement", ":type_switch_statement",
		":type_case", ":default_case", ":select_statement", ":communication_case",
		":receive_statement", ":send_statement", ":go_statement", ":defer_statement",
		":slice_expression", ":type_assertion_expression", ":variadic_argument",
		":function_type", ":channel_type", ":struct_type", ":interface_type", ":iota",
		":dec_statement", ":inc_statement", `[:symbol "<<="]`, ":type_parameter_list",
		":variadic_parameter_declaration", ":map_type", ":qualified_type",
	} {
		if !strings.Contains(printed, kind) {
			t.Errorf("normalized tree lacks %s", kind)
		}
	}
	if strings.Contains(printed, "&^") {
		t.Error("&^= is kept; dryer drops operators it does not list")
	}
	types := parseFile(t, "b.go", []byte("package p\nfunc G(p struct{ a int `t` }, i interface{ M(); fmt.Stringer }, f (func())) {}\n"))
	got := pr(n.Func(types.Funcs[0].Decl))
	for _, kind := range []string{":field_declaration", ":method_elem", ":type_elem", ":parenthesized_type"} {
		if !strings.Contains(got, kind) {
			t.Errorf("%s lacks %s", got, kind)
		}
	}
}

func TestNearCopyFoundAndDissimilarIgnored(t *testing.T) {
	file := fixture(t, "near.go")
	entries := Entries([]*analysis.File{file}, DefaultMinLines, DefaultMinNodes)
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Func.Name] = true
	}
	if names["Tiny"] || !names["JoinUpper"] {
		t.Errorf("entries = %v, want Tiny filtered out and JoinUpper kept", names)
	}
	pairs := AllPairs(entries, DefaultThreshold)
	if len(pairs) != 1 {
		t.Fatalf("pairs = %d, want 1", len(pairs))
	}
	p := pairs[0]
	if p.Left.Func.Name != "SumPositive" || p.Right.Func.Name != "TotalAboveZero" || p.Score != 1 {
		t.Errorf("pair = %s %s %v, want SumPositive TotalAboveZero 1", p.Left.Func.Name, p.Right.Func.Name, p.Score)
	}
	for _, other := range AllPairs(entries, 0) {
		if other.Left.Func.Name == "JoinUpper" || other.Right.Func.Name == "JoinUpper" {
			if other.Score >= DefaultThreshold {
				t.Errorf("JoinUpper scored %v", other.Score)
			}
		}
	}
}

func TestAgainstReportsEachPairOnce(t *testing.T) {
	entries := Entries([]*analysis.File{fixture(t, "near.go")}, DefaultMinLines, DefaultMinNodes)
	both := Against(entries[:2], entries, 0)
	if len(both) != 3 {
		t.Errorf("pairs = %d, want 3 (a-b, a-c, b-c)", len(both))
	}
	one := Against(entries[2:], entries, 0)
	if len(one) != 2 {
		t.Errorf("pairs = %d, want 2", len(one))
	}
}

func TestJaccard(t *testing.T) {
	cases := []struct {
		a, b []uint64
		want float64
	}{
		{nil, nil, 0},
		{[]uint64{1, 2}, []uint64{1, 2}, 1},
		{[]uint64{1, 2, 3}, []uint64{2, 3, 4}, 0.5},
		{[]uint64{1}, []uint64{2}, 0},
	}
	for _, tc := range cases {
		if got := Jaccard(tc.a, tc.b); got != tc.want {
			t.Errorf("Jaccard(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSizePrefilterIsExact(t *testing.T) {
	a := &Entry{prints: []uint64{1, 2, 3, 4, 5}}
	b := &Entry{prints: []uint64{1, 2, 3, 4}}
	if got := score(a, b, 0.8); got != 0.8 {
		t.Errorf("score = %v, want 0.8", got)
	}
	if got := score(a, b, 0.81); got != -1 {
		t.Errorf("score = %v, want -1 (cannot reach 0.81)", got)
	}
	if got := score(&Entry{}, &Entry{}, 0.5); got != -1 {
		t.Errorf("empty score = %v", got)
	}
}

func TestNodeCountMatchesDryer(t *testing.T) {
	file := parseFile(t, "a.go", []byte("package p\nfunc F() {}\n"))
	e := newEntry(newNormalizer(), file.Funcs[0])
	// [:function_declaration :symbol [:parameter_list] [:block]]: 3 lists, 4 atoms.
	if e.Nodes != 7 {
		t.Errorf("nodes = %d, want 7", e.Nodes)
	}
	// Distinct prints: the 3 lists and the atoms :function_declaration,
	// :symbol, :parameter_list, :block.
	if len(e.prints) != 7 {
		t.Errorf("fingerprints = %d, want 7", len(e.prints))
	}
}

func prints(t *testing.T, body string) []uint64 {
	t.Helper()
	file := parseFile(t, "a.go", []byte("package p\nfunc F(x int) {\n"+body+"\n}\n"))
	return newEntry(newNormalizer(), file.Funcs[0]).prints
}

func TestGenericCalleeNamesKept(t *testing.T) {
	same := func(a, b string) bool { return Jaccard(prints(t, a), prints(t, b)) == 1 }
	cases := []struct {
		a, b string
		same bool
	}{
		{`y := Map[int](x, 1)`, `z := Map[int](x, 2)`, true},
		{`y := Map[int](x)`, `y := Filter[int](x)`, false},
		{`y := Pair[int, string](x)`, `y := Swap[int, string](x)`, false},
		{`y := slices.Map[int](x)`, `y := slices.Filter[int](x)`, false},
		{`y := lo.Map[int, string](x)`, `y := lo.Keys[int, string](x)`, false},
		{`y := (Map[int])(x)`, `y := (Filter[int])(x)`, false},
	}
	for _, tc := range cases {
		if got := same(tc.a, tc.b); got != tc.same {
			t.Errorf("%s vs %s: identical = %v, want %v", tc.a, tc.b, got, tc.same)
		}
	}
	got := normalizeBody(t, `Map[int](x)`)
	want := `[:call_expression [:symbol "Map"] [:type_arguments [:type_elem :symbol]] [:argument_list :symbol]]`
	if !strings.Contains(got, want) {
		t.Errorf("got %s\nwant it to contain %s", got, want)
	}
}
