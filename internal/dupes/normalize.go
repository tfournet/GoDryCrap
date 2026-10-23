// Package dupes finds structurally similar functions the way dryer does:
// normalize each function, fingerprint every subtree, and score pairs by
// Jaccard similarity of the fingerprint sets.
package dupes

import (
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
)

// node is one element of a normalized tree: an atom (a keyword such as
// ":block" or ":symbol", or a quoted name such as "\"Println\"") or a list.
// It mirrors the tree dryer builds from tree-sitter, which prints like a
// Clojure vector of keywords and strings.
type node struct {
	atom string
	kids []*node
}

func (n *node) isList() bool { return n.atom == "" }

func keyword(k string) *node { return &node{atom: ":" + k} }

func name(s string) *node { return &node{atom: strconv.Quote(s)} }

// list is [:kind kids...], dropping nil kids the way dryer drops
// punctuation and keywords.
func list(kind string, kids ...*node) *node {
	n := &node{kids: make([]*node, 0, len(kids)+1)}
	n.kids = append(n.kids, keyword(kind))
	for _, k := range kids {
		if k != nil {
			n.kids = append(n.kids, k)
		}
	}
	return n
}

// symbol is a local, field or type name: its spelling is dropped.
func symbol() *node { return keyword("symbol") }

// called keeps the spelling of a name in call position: [:symbol "name"].
func called(s string) *node { return list("symbol", name(s)) }

// operators are the tokens dryer keeps, as [:symbol "op"]. Others, such as
// <- and &^, are dropped like punctuation.
var operators = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true,
	"==": true, "!=": true, "<": true, ">": true, "<=": true, ">=": true,
	"&&": true, "||": true, "&": true, "|": true, "^": true, "<<": true, ">>": true,
	"+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "&=": true, "|=": true,
	"^=": true, "<<=": true, ">>=": true, "!": true, "++": true, "--": true,
	"=": true, ":=": true,
}

func op(tok token.Token) *node {
	if !operators[tok.String()] {
		return nil
	}
	return called(tok.String())
}

// literals are the predeclared identifiers tree-sitter parses as literal
// nodes rather than identifiers.
var literals = map[string]bool{"nil": true, "true": true, "false": true}

type exprFunc func(e ast.Expr, head bool) *node

type stmtFunc func(s ast.Stmt) *node

// normalizer holds the dispatch tables. exprs serve expression positions;
// types override them in type positions, where tree-sitter has distinct
// node kinds (qualified_type, pointer_type, generic_type).
type normalizer struct {
	exprs map[reflect.Type]exprFunc
	types map[reflect.Type]exprFunc
	stmts map[reflect.Type]stmtFunc
}

func newNormalizer() *normalizer {
	n := &normalizer{}
	n.exprs = n.exprTable()
	n.types = n.typeTable()
	n.stmts = n.stmtTable()
	return n
}

// expr normalizes e. head is true when e is the callee of a call, so its
// name is kept.
func (n *normalizer) expr(e ast.Expr, head bool) *node {
	if e == nil || reflect.ValueOf(e).IsNil() {
		return nil
	}
	if f, ok := n.exprs[reflect.TypeOf(e)]; ok {
		return f(e, head)
	}
	return nil
}

// typ normalizes e in a type position.
func (n *normalizer) typ(e ast.Expr) *node {
	if e == nil || reflect.ValueOf(e).IsNil() {
		return nil
	}
	if f, ok := n.types[reflect.TypeOf(e)]; ok {
		return f(e, false)
	}
	return n.expr(e, false)
}

func (n *normalizer) stmt(s ast.Stmt) *node {
	if s == nil || reflect.ValueOf(s).IsNil() {
		return nil
	}
	if f, ok := n.stmts[reflect.TypeOf(s)]; ok {
		return f(s)
	}
	return nil
}

func (n *normalizer) exprNodes(es []ast.Expr) []*node {
	out := make([]*node, len(es))
	for i, e := range es {
		out[i] = n.expr(e, false)
	}
	return out
}

func (n *normalizer) exprList(es []ast.Expr) *node {
	if len(es) == 0 {
		return nil
	}
	return list("expression_list", n.exprNodes(es)...)
}

func (n *normalizer) stmtList(ss []ast.Stmt) *node {
	if len(ss) == 0 {
		return nil
	}
	out := make([]*node, len(ss))
	for i, s := range ss {
		out[i] = n.stmt(s)
	}
	return list("statement_list", out...)
}

func (n *normalizer) block(b *ast.BlockStmt) *node {
	if b == nil {
		return nil
	}
	return list("block", n.stmtList(b.List))
}

// Func normalizes a function or method declaration.
func (n *normalizer) Func(fn *ast.FuncDecl) *node {
	if fn.Recv != nil {
		return list("method_declaration", n.params(fn.Recv), symbol(), n.params(fn.Type.Params), n.result(fn.Type.Results), n.block(fn.Body))
	}
	return list("function_declaration", symbol(), n.typeParams(fn.Type.TypeParams), n.params(fn.Type.Params), n.result(fn.Type.Results), n.block(fn.Body))
}

func symbols(idents []*ast.Ident) []*node {
	out := make([]*node, len(idents))
	for i := range idents {
		out[i] = symbol()
	}
	return out
}

func (n *normalizer) params(fl *ast.FieldList) *node {
	if fl == nil {
		return list("parameter_list")
	}
	out := make([]*node, len(fl.List))
	for i, f := range fl.List {
		kind, t := "parameter_declaration", f.Type
		if e, ok := t.(*ast.Ellipsis); ok {
			kind, t = "variadic_parameter_declaration", e.Elt
		}
		out[i] = list(kind, append(symbols(f.Names), n.typ(t))...)
	}
	return list("parameter_list", out...)
}

func (n *normalizer) result(fl *ast.FieldList) *node {
	if fl == nil || len(fl.List) == 0 {
		return nil
	}
	if len(fl.List) == 1 && len(fl.List[0].Names) == 0 {
		return n.typ(fl.List[0].Type)
	}
	return n.params(fl)
}

func (n *normalizer) typeParams(fl *ast.FieldList) *node {
	if fl == nil {
		return nil
	}
	out := make([]*node, len(fl.List))
	for i, f := range fl.List {
		out[i] = list("type_parameter_declaration", append(symbols(f.Names), list("type_constraint", n.typ(f.Type)))...)
	}
	return list("type_parameter_list", out...)
}
