// Derived from Go's cmd/cover (src/cmd/cover/cover.go, Go 1.27).
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by the BSD-style license in
// LICENSE-go in this directory. Modifications are released under this
// repository's 0BSD license.

package coverlayout

import (
	"go/ast"
	"go/token"
	"reflect"
)

// control describes a control statement for statementBoundary: the parts
// searched for a function literal, in cover's order, and the body brace that
// otherwise ends the basic block.
type control func(s ast.Stmt) (parts []ast.Node, lbrace token.Pos)

var controls = map[reflect.Type]control{
	nodeType(&ast.BlockStmt{}): func(s ast.Stmt) ([]ast.Node, token.Pos) {
		return nil, s.(*ast.BlockStmt).Lbrace
	},
	nodeType(&ast.IfStmt{}): func(s ast.Stmt) ([]ast.Node, token.Pos) {
		x := s.(*ast.IfStmt)
		return []ast.Node{x.Init, x.Cond}, x.Body.Lbrace
	},
	nodeType(&ast.ForStmt{}): func(s ast.Stmt) ([]ast.Node, token.Pos) {
		x := s.(*ast.ForStmt)
		return []ast.Node{x.Init, x.Cond, x.Post}, x.Body.Lbrace
	},
	nodeType(&ast.RangeStmt{}): func(s ast.Stmt) ([]ast.Node, token.Pos) {
		x := s.(*ast.RangeStmt)
		return []ast.Node{x.X}, x.Body.Lbrace
	},
	nodeType(&ast.SwitchStmt{}): func(s ast.Stmt) ([]ast.Node, token.Pos) {
		x := s.(*ast.SwitchStmt)
		return []ast.Node{x.Init, x.Tag}, x.Body.Lbrace
	},
	nodeType(&ast.SelectStmt{}): func(s ast.Stmt) ([]ast.Node, token.Pos) {
		return nil, s.(*ast.SelectStmt).Body.Lbrace
	},
	nodeType(&ast.TypeSwitchStmt{}): func(s ast.Stmt) ([]ast.Node, token.Pos) {
		x := s.(*ast.TypeSwitchStmt)
		return []ast.Node{x.Init}, x.Body.Lbrace
	},
}

// statementBoundary is cover's: where s ends the current basic block. A
// control statement ends it at its body's brace, or at the first function
// literal in its header; any other statement at its first function
// literal's body, or at its end.
func statementBoundary(s ast.Stmt) token.Pos {
	if label, ok := s.(*ast.LabeledStmt); ok {
		return statementBoundary(label.Stmt)
	}
	if c, ok := controls[nodeType(s)]; ok {
		parts, lbrace := c(s)
		for _, part := range parts {
			if pos, found := funcLiteral(part); found {
				return pos
			}
		}
		return lbrace
	}
	if pos, found := funcLiteral(s); found {
		return pos
	}
	return s.End()
}

// enders are the statements that always end a basic block.
var enders = map[reflect.Type]bool{
	nodeType(&ast.BlockStmt{}):      true,
	nodeType(&ast.BranchStmt{}):     true,
	nodeType(&ast.ForStmt{}):        true,
	nodeType(&ast.IfStmt{}):         true,
	nodeType(&ast.LabeledStmt{}):    true,
	nodeType(&ast.RangeStmt{}):      true,
	nodeType(&ast.SwitchStmt{}):     true,
	nodeType(&ast.SelectStmt{}):     true,
	nodeType(&ast.TypeSwitchStmt{}): true,
}

// endsBasicSourceBlock is cover's: control flow, a call to panic with one
// argument, or a statement holding a function literal.
func endsBasicSourceBlock(s ast.Stmt) bool {
	if enders[nodeType(s)] || isPanic(s) {
		return true
	}
	_, found := funcLiteral(s)
	return found
}

func isPanic(s ast.Stmt) bool {
	e, ok := s.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := e.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "panic" && len(call.Args) == 1
}

// isControl reports a control statement that, labeled, cannot be split
// from its label.
func isControl(s ast.Stmt) bool {
	switch s.(type) {
	case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.SelectStmt, *ast.TypeSwitchStmt:
		return true
	}
	return false
}

// funcLiteral is cover's hasFuncLiteral: the body brace of the first
// function literal in n.
func funcLiteral(n ast.Node) (token.Pos, bool) {
	if n == nil || reflect.ValueOf(n).IsNil() {
		return token.NoPos, false
	}
	var found token.Pos
	ast.Inspect(n, func(x ast.Node) bool {
		if found.IsValid() {
			return false
		}
		if lit, ok := x.(*ast.FuncLit); ok {
			found = lit.Body.Lbrace
			return false
		}
		return true
	})
	return found, found.IsValid()
}
