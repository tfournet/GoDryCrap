package dupes

import (
	"go/ast"
	"go/token"
	"reflect"
)

func (n *normalizer) stmtTable() map[reflect.Type]stmtFunc {
	return map[reflect.Type]stmtFunc{
		typeOf(&ast.BlockStmt{}):  func(s ast.Stmt) *node { return n.block(s.(*ast.BlockStmt)) },
		typeOf(&ast.ExprStmt{}):   func(s ast.Stmt) *node { return list("expression_statement", n.expr(s.(*ast.ExprStmt).X, false)) },
		typeOf(&ast.SendStmt{}):   n.send,
		typeOf(&ast.IncDecStmt{}): n.incDec,
		typeOf(&ast.AssignStmt{}): n.assign,
		typeOf(&ast.GoStmt{}):     func(s ast.Stmt) *node { return list("go_statement", n.expr(s.(*ast.GoStmt).Call, false)) },
		typeOf(&ast.DeferStmt{}):  func(s ast.Stmt) *node { return list("defer_statement", n.expr(s.(*ast.DeferStmt).Call, false)) },
		typeOf(&ast.ReturnStmt{}): func(s ast.Stmt) *node { return list("return_statement", n.exprList(s.(*ast.ReturnStmt).Results)) },
		typeOf(&ast.BranchStmt{}): n.branch,
		typeOf(&ast.LabeledStmt{}): func(s ast.Stmt) *node {
			return list("labeled_statement", list("label_name"), n.stmt(s.(*ast.LabeledStmt).Stmt))
		},
		typeOf(&ast.IfStmt{}):         n.ifStmt,
		typeOf(&ast.ForStmt{}):        n.forStmt,
		typeOf(&ast.RangeStmt{}):      n.rangeStmt,
		typeOf(&ast.SwitchStmt{}):     n.switchStmt,
		typeOf(&ast.TypeSwitchStmt{}): n.typeSwitch,
		typeOf(&ast.SelectStmt{}):     n.selectStmt,
		typeOf(&ast.DeclStmt{}):       n.declStmt,
	}
}

func (n *normalizer) send(s ast.Stmt) *node {
	x := s.(*ast.SendStmt)
	return list("send_statement", n.expr(x.Chan, false), n.expr(x.Value, false))
}

func (n *normalizer) incDec(s ast.Stmt) *node {
	x := s.(*ast.IncDecStmt)
	kind := "inc_statement"
	if x.Tok == token.DEC {
		kind = "dec_statement"
	}
	return list(kind, n.expr(x.X, false), op(x.Tok))
}

func (n *normalizer) assign(s ast.Stmt) *node {
	x := s.(*ast.AssignStmt)
	kind := "assignment_statement"
	if x.Tok == token.DEFINE {
		kind = "short_var_declaration"
	}
	return list(kind, n.exprList(x.Lhs), op(x.Tok), n.exprList(x.Rhs))
}

var branchKinds = map[token.Token]string{
	token.BREAK:       "break_statement",
	token.CONTINUE:    "continue_statement",
	token.GOTO:        "goto_statement",
	token.FALLTHROUGH: "fallthrough_statement",
}

func (n *normalizer) branch(s ast.Stmt) *node {
	x := s.(*ast.BranchStmt)
	var label *node
	if x.Label != nil {
		label = list("label_name")
	}
	return list(branchKinds[x.Tok], label)
}

func (n *normalizer) ifStmt(s ast.Stmt) *node {
	x := s.(*ast.IfStmt)
	return list("if_statement", n.stmt(x.Init), n.expr(x.Cond, false), n.block(x.Body), n.stmt(x.Else))
}

func (n *normalizer) forStmt(s ast.Stmt) *node {
	x := s.(*ast.ForStmt)
	if x.Init == nil && x.Post == nil {
		return list("for_statement", n.expr(x.Cond, false), n.block(x.Body))
	}
	return list("for_statement", list("for_clause", n.stmt(x.Init), n.expr(x.Cond, false), n.stmt(x.Post)), n.block(x.Body))
}

func (n *normalizer) rangeStmt(s ast.Stmt) *node {
	x := s.(*ast.RangeStmt)
	var left []ast.Expr
	for _, e := range []ast.Expr{x.Key, x.Value} {
		if e != nil {
			left = append(left, e)
		}
	}
	var tok *node
	if len(left) > 0 {
		tok = op(x.Tok)
	}
	return list("for_statement", list("range_clause", n.exprList(left), tok, n.expr(x.X, false)), n.block(x.Body))
}

func (n *normalizer) switchStmt(s ast.Stmt) *node {
	x := s.(*ast.SwitchStmt)
	kids := []*node{n.stmt(x.Init), n.expr(x.Tag, false)}
	for _, c := range x.Body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil {
			kids = append(kids, list("default_case", n.stmtList(cc.Body)))
			continue
		}
		kids = append(kids, list("expression_case", n.exprList(cc.List), n.stmtList(cc.Body)))
	}
	return list("expression_switch_statement", kids...)
}

func (n *normalizer) typeSwitch(s ast.Stmt) *node {
	x := s.(*ast.TypeSwitchStmt)
	kids := append([]*node{n.stmt(x.Init)}, n.typeSwitchGuard(x.Assign)...)
	for _, c := range x.Body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil {
			kids = append(kids, list("default_case", n.stmtList(cc.Body)))
			continue
		}
		types := make([]*node, len(cc.List))
		for i, t := range cc.List {
			types[i] = n.typ(t)
		}
		kids = append(kids, list("type_case", append(types, n.stmtList(cc.Body))...))
	}
	return list("type_switch_statement", kids...)
}

// typeSwitchGuard is `v := x.(type)` as alias, :=, value; or `x.(type)` as
// the value alone.
func (n *normalizer) typeSwitchGuard(s ast.Stmt) []*node {
	if a, ok := s.(*ast.AssignStmt); ok {
		value := a.Rhs[0].(*ast.TypeAssertExpr).X
		return []*node{n.exprList(a.Lhs), op(a.Tok), n.expr(value, false)}
	}
	return []*node{n.expr(s.(*ast.ExprStmt).X.(*ast.TypeAssertExpr).X, false)}
}

func (n *normalizer) selectStmt(s ast.Stmt) *node {
	x := s.(*ast.SelectStmt)
	kids := make([]*node, len(x.Body.List))
	for i, c := range x.Body.List {
		cc := c.(*ast.CommClause)
		if cc.Comm == nil {
			kids[i] = list("default_case", n.stmtList(cc.Body))
			continue
		}
		kids[i] = list("communication_case", n.comm(cc.Comm), n.stmtList(cc.Body))
	}
	return list("select_statement", kids...)
}

// comm is a select case's send or receive.
func (n *normalizer) comm(s ast.Stmt) *node {
	switch x := s.(type) {
	case *ast.ExprStmt:
		return list("receive_statement", n.expr(x.X, false))
	case *ast.AssignStmt:
		return list("receive_statement", n.exprList(x.Lhs), op(x.Tok), n.expr(x.Rhs[0], false))
	}
	return n.stmt(s)
}

var declKinds = map[token.Token][2]string{
	token.VAR:   {"var_declaration", "var_spec"},
	token.CONST: {"const_declaration", "const_spec"},
	token.TYPE:  {"type_declaration", "type_spec"},
}

func (n *normalizer) declStmt(s ast.Stmt) *node {
	d := s.(*ast.DeclStmt).Decl.(*ast.GenDecl)
	kinds := declKinds[d.Tok]
	specs := make([]*node, len(d.Specs))
	for i, spec := range d.Specs {
		specs[i] = n.spec(kinds[1], spec)
	}
	return list(kinds[0], specs...)
}

func (n *normalizer) spec(kind string, spec ast.Spec) *node {
	if t, ok := spec.(*ast.TypeSpec); ok {
		if t.Assign.IsValid() {
			return list("type_alias", symbol(), n.typeParams(t.TypeParams), called("="), n.typ(t.Type))
		}
		return list(kind, symbol(), n.typeParams(t.TypeParams), n.typ(t.Type))
	}
	v := spec.(*ast.ValueSpec)
	var assign *node
	if len(v.Values) > 0 {
		assign = called("=")
	}
	return list(kind, append(symbols(v.Names), n.typ(v.Type), assign, n.exprList(v.Values))...)
}
