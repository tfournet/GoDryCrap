package dupes

import (
	"go/ast"
	"reflect"
)

func typeOf(v any) reflect.Type { return reflect.TypeOf(v) }

func (n *normalizer) exprTable() map[reflect.Type]exprFunc {
	return map[reflect.Type]exprFunc{
		typeOf(&ast.Ident{}):          n.ident,
		typeOf(&ast.BasicLit{}):       func(ast.Expr, bool) *node { return keyword("literal") },
		typeOf(&ast.CompositeLit{}):   n.composite,
		typeOf(&ast.FuncLit{}):        n.funcLit,
		typeOf(&ast.ParenExpr{}):      func(e ast.Expr, head bool) *node { return n.expr(e.(*ast.ParenExpr).X, head) },
		typeOf(&ast.SelectorExpr{}):   n.selector,
		typeOf(&ast.IndexExpr{}):      n.index,
		typeOf(&ast.IndexListExpr{}):  n.indexList,
		typeOf(&ast.SliceExpr{}):      n.slice,
		typeOf(&ast.TypeAssertExpr{}): n.typeAssert,
		typeOf(&ast.CallExpr{}):       n.call,
		typeOf(&ast.StarExpr{}):       n.star,
		typeOf(&ast.UnaryExpr{}):      n.unary,
		typeOf(&ast.BinaryExpr{}):     n.binary,
		typeOf(&ast.KeyValueExpr{}):   n.keyValue,
		typeOf(&ast.ArrayType{}):      n.arrayType,
		typeOf(&ast.MapType{}):        n.mapType,
		typeOf(&ast.ChanType{}):       n.chanType,
		typeOf(&ast.FuncType{}):       n.funcType,
		typeOf(&ast.StructType{}):     n.structType,
		typeOf(&ast.InterfaceType{}):  n.interfaceType,
		typeOf(&ast.Ellipsis{}):       func(e ast.Expr, _ bool) *node { return n.typ(e.(*ast.Ellipsis).Elt) },
	}
}

func (n *normalizer) typeTable() map[reflect.Type]exprFunc {
	return map[reflect.Type]exprFunc{
		typeOf(&ast.Ident{}):         func(ast.Expr, bool) *node { return symbol() },
		typeOf(&ast.SelectorExpr{}):  func(ast.Expr, bool) *node { return list("qualified_type", symbol(), symbol()) },
		typeOf(&ast.StarExpr{}):      func(e ast.Expr, _ bool) *node { return list("pointer_type", called("*"), n.typ(e.(*ast.StarExpr).X)) },
		typeOf(&ast.IndexExpr{}):     func(e ast.Expr, _ bool) *node { x := e.(*ast.IndexExpr); return n.generic(x.X, []ast.Expr{x.Index}) },
		typeOf(&ast.IndexListExpr{}): func(e ast.Expr, _ bool) *node { x := e.(*ast.IndexListExpr); return n.generic(x.X, x.Indices) },
		typeOf(&ast.ParenExpr{}):     func(e ast.Expr, _ bool) *node { return list("parenthesized_type", n.typ(e.(*ast.ParenExpr).X)) },
	}
}

func (n *normalizer) generic(base ast.Expr, args []ast.Expr) *node {
	return list("generic_type", n.typ(base), n.typeArguments(args))
}

func (n *normalizer) ident(e ast.Expr, head bool) *node {
	id := e.(*ast.Ident)
	switch {
	case head:
		return called(id.Name)
	case literals[id.Name]:
		return keyword("literal")
	case id.Name == "iota":
		return list("iota")
	}
	return symbol()
}

func (n *normalizer) selector(e ast.Expr, head bool) *node {
	x := e.(*ast.SelectorExpr)
	field := symbol()
	if head {
		field = called(x.Sel.Name)
	}
	return list("selector_expression", n.expr(x.X, false), field)
}

func (n *normalizer) composite(e ast.Expr, _ bool) *node {
	x := e.(*ast.CompositeLit)
	return list("composite_literal", n.typ(x.Type), n.literalValue(x.Elts))
}

func (n *normalizer) literalValue(elts []ast.Expr) *node {
	out := make([]*node, len(elts))
	for i, e := range elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			out[i] = list("keyed_element", n.literalElement(kv.Key), n.literalElement(kv.Value))
			continue
		}
		out[i] = n.literalElement(e)
	}
	return list("literal_value", out...)
}

// literalElement wraps an element; a composite literal with its type elided
// is a bare literal_value.
func (n *normalizer) literalElement(e ast.Expr) *node {
	if c, ok := e.(*ast.CompositeLit); ok && c.Type == nil {
		return list("literal_element", n.literalValue(c.Elts))
	}
	return list("literal_element", n.expr(e, false))
}

func (n *normalizer) keyValue(e ast.Expr, _ bool) *node {
	x := e.(*ast.KeyValueExpr)
	return list("keyed_element", n.literalElement(x.Key), n.literalElement(x.Value))
}

func (n *normalizer) funcLit(e ast.Expr, _ bool) *node {
	x := e.(*ast.FuncLit)
	return list("func_literal", n.params(x.Type.Params), n.result(x.Type.Results), n.block(x.Body))
}

func (n *normalizer) index(e ast.Expr, head bool) *node {
	x := e.(*ast.IndexExpr)
	return list("index_expression", n.expr(x.X, head), n.expr(x.Index, false))
}

func (n *normalizer) indexList(e ast.Expr, head bool) *node {
	x := e.(*ast.IndexListExpr)
	return list("index_expression", append([]*node{n.expr(x.X, head)}, n.exprNodes(x.Indices)...)...)
}

func (n *normalizer) slice(e ast.Expr, _ bool) *node {
	x := e.(*ast.SliceExpr)
	return list("slice_expression", n.expr(x.X, false), n.expr(x.Low, false), n.expr(x.High, false), n.expr(x.Max, false))
}

func (n *normalizer) typeAssert(e ast.Expr, _ bool) *node {
	x := e.(*ast.TypeAssertExpr)
	return list("type_assertion_expression", n.expr(x.X, false), n.typ(x.Type))
}

// call keeps the callee's name: fmt.Println(x) is
// [:call_expression [:selector_expression :symbol [:symbol "Println"]] [:argument_list :symbol]].
// An indexed callee is a generic instantiation, as tree-sitter-go parses
// F[int](x): the name F stays and the indices become type arguments.
func (n *normalizer) call(e ast.Expr, _ bool) *node {
	x := e.(*ast.CallExpr)
	args := n.exprNodes(x.Args)
	if x.Ellipsis.IsValid() && len(args) > 0 {
		args[len(args)-1] = list("variadic_argument", args[len(args)-1])
	}
	callee, typeArgs := n.callee(x.Fun)
	return list("call_expression", callee, typeArgs, list("argument_list", args...))
}

func (n *normalizer) callee(fun ast.Expr) (callee, typeArgs *node) {
	switch f := ast.Unparen(fun).(type) {
	case *ast.IndexExpr:
		return n.expr(f.X, true), n.typeArguments([]ast.Expr{f.Index})
	case *ast.IndexListExpr:
		return n.expr(f.X, true), n.typeArguments(f.Indices)
	}
	return n.expr(fun, true), nil
}

func (n *normalizer) typeArguments(args []ast.Expr) *node {
	elems := make([]*node, len(args))
	for i, a := range args {
		elems[i] = list("type_elem", n.typ(a))
	}
	return list("type_arguments", elems...)
}

func (n *normalizer) star(e ast.Expr, _ bool) *node {
	return list("unary_expression", called("*"), n.expr(e.(*ast.StarExpr).X, false))
}

func (n *normalizer) unary(e ast.Expr, _ bool) *node {
	x := e.(*ast.UnaryExpr)
	return list("unary_expression", op(x.Op), n.expr(x.X, false))
}

func (n *normalizer) binary(e ast.Expr, _ bool) *node {
	x := e.(*ast.BinaryExpr)
	return list("binary_expression", n.expr(x.X, false), op(x.Op), n.expr(x.Y, false))
}

func (n *normalizer) arrayType(e ast.Expr, _ bool) *node {
	x := e.(*ast.ArrayType)
	if x.Len == nil {
		return list("slice_type", n.typ(x.Elt))
	}
	if _, ok := x.Len.(*ast.Ellipsis); ok {
		return list("implicit_length_array_type", n.typ(x.Elt))
	}
	return list("array_type", n.expr(x.Len, false), n.typ(x.Elt))
}

func (n *normalizer) mapType(e ast.Expr, _ bool) *node {
	x := e.(*ast.MapType)
	return list("map_type", n.typ(x.Key), n.typ(x.Value))
}

func (n *normalizer) chanType(e ast.Expr, _ bool) *node {
	return list("channel_type", n.typ(e.(*ast.ChanType).Value))
}

func (n *normalizer) funcType(e ast.Expr, _ bool) *node {
	x := e.(*ast.FuncType)
	return list("function_type", n.params(x.Params), n.result(x.Results))
}

func (n *normalizer) structType(e ast.Expr, _ bool) *node {
	fields := e.(*ast.StructType).Fields.List
	out := make([]*node, len(fields))
	for i, f := range fields {
		var tag *node
		if f.Tag != nil {
			tag = keyword("literal")
		}
		out[i] = list("field_declaration", append(symbols(f.Names), n.typ(f.Type), tag)...)
	}
	return list("struct_type", list("field_declaration_list", out...))
}

func (n *normalizer) interfaceType(e ast.Expr, _ bool) *node {
	methods := e.(*ast.InterfaceType).Methods.List
	out := make([]*node, len(methods))
	for i, f := range methods {
		if ft, ok := f.Type.(*ast.FuncType); ok && len(f.Names) > 0 {
			out[i] = list("method_elem", symbol(), n.params(ft.Params), n.result(ft.Results))
			continue
		}
		out[i] = list("type_elem", n.typ(f.Type))
	}
	return list("interface_type", out...)
}
