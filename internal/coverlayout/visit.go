package coverlayout

import (
	"go/ast"
	"reflect"
)

type visitFunc func(l *layout, n ast.Node) ast.Visitor

func nodeType(n ast.Node) reflect.Type { return reflect.TypeOf(n) }

// visitors are cover's Visit cases; other nodes are walked into.
var visitors map[reflect.Type]visitFunc

func init() {
	visitors = map[reflect.Type]visitFunc{
		nodeType(&ast.BlockStmt{}):      visitBlock,
		nodeType(&ast.IfStmt{}):         visitIf,
		nodeType(&ast.SelectStmt{}):     visitSelect,
		nodeType(&ast.SwitchStmt{}):     visitSwitch,
		nodeType(&ast.TypeSwitchStmt{}): visitTypeSwitch,
		nodeType(&ast.FuncDecl{}):       visitFuncDecl,
		nodeType(&ast.FuncLit{}):        visitFuncLit,
	}
}

// visitBlock adds a block per case clause of a switch or select body, and
// otherwise the blocks of the statement list.
func visitBlock(l *layout, node ast.Node) ast.Visitor {
	n := node.(*ast.BlockStmt)
	if len(n.List) > 0 {
		switch n.List[0].(type) {
		case *ast.CaseClause:
			for _, s := range n.List {
				c := s.(*ast.CaseClause)
				l.addCounters(c.Colon+1, c.Colon+1, c.End(), c.Body, false)
			}
			return l
		case *ast.CommClause:
			for _, s := range n.List {
				c := s.(*ast.CommClause)
				l.addCounters(c.Colon+1, c.Colon+1, c.End(), c.Body, false)
			}
			return l
		}
	}
	l.addCounters(n.Lbrace, n.Lbrace+1, n.Rbrace+1, n.List, true)
	return l
}

// visitIf walks the parts of an if. cover wraps an else in a block that
// opens just after the else keyword, so an else-if gets a counter; the walk
// here uses a copy with that position and leaves the tree unchanged.
func visitIf(l *layout, node ast.Node) ast.Visitor {
	n := node.(*ast.IfStmt)
	if n.Init != nil {
		ast.Walk(l, n.Init)
	}
	ast.Walk(l, n.Cond)
	ast.Walk(l, n.Body)
	if n.Else == nil {
		return nil
	}
	elseOffset := l.findText(n.Body.End(), "else")
	if elseOffset < 0 {
		l.err = errLostElse
		return nil
	}
	pos := l.fset.File(n.Body.End()).Pos(elseOffset + 4)
	block := &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{n.Else}, Rbrace: n.Else.End()}
	if b, ok := n.Else.(*ast.BlockStmt); ok {
		copied := *b
		copied.Lbrace = pos
		block = &copied
	}
	ast.Walk(l, block)
	return nil
}

func visitSelect(l *layout, node ast.Node) ast.Visitor {
	if n := node.(*ast.SelectStmt); n.Body == nil || len(n.Body.List) == 0 {
		return nil
	}
	return l
}

func visitSwitch(l *layout, node ast.Node) ast.Visitor {
	n := node.(*ast.SwitchStmt)
	if n.Body != nil && len(n.Body.List) > 0 {
		return l
	}
	if n.Init != nil {
		ast.Walk(l, n.Init)
	}
	if n.Tag != nil {
		ast.Walk(l, n.Tag)
	}
	return nil
}

func visitTypeSwitch(l *layout, node ast.Node) ast.Visitor {
	n := node.(*ast.TypeSwitchStmt)
	if n.Body != nil && len(n.Body.List) > 0 {
		return l
	}
	if n.Init != nil {
		ast.Walk(l, n.Init)
	}
	ast.Walk(l, n.Assign)
	return nil
}

// visitFuncDecl collects one function's blocks. Functions named _ and
// declarations without a body are not instrumented.
func visitFuncDecl(l *layout, node ast.Node) ast.Visitor {
	n := node.(*ast.FuncDecl)
	if n.Name.Name == "_" || n.Body == nil {
		return nil
	}
	blocks := []Block{}
	l.cur = &blocks
	ast.Walk(l, n.Body)
	l.cur = nil
	l.funcs[n] = blocks
	return nil
}

// visitFuncLit counts a literal inside a function with that function; a
// package-level literal is a function of its own.
func visitFuncLit(l *layout, node ast.Node) ast.Visitor {
	if l.cur != nil {
		return l
	}
	blocks := []Block{}
	l.cur = &blocks
	ast.Walk(l, node.(*ast.FuncLit).Body)
	l.cur = nil
	return nil
}
