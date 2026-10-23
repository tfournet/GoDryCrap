package analysis

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"

	"github.com/tfournet/GoDryCrap/internal/coverlayout"
	"github.com/tfournet/GoDryCrap/internal/usererr"
)

// File is one parsed Go source file and the functions it declares.
type File struct {
	Path    string // slash-separated, relative to the repository or as given
	Package string // import path, or the package clause name without a go.mod
	// InModule is true when Package is an import path from a go.mod.
	InModule bool
	// Layout is every coverage block go cover instruments in the file,
	// package-level function literals included.
	Layout []coverlayout.Block
	Fset   *token.FileSet
	AST    *ast.File
	Funcs  []*Func
}

// DropSyntax releases the file's syntax tree and its functions' source
// text and declarations, keeping names, spans, complexity and coverage
// anchors. Change detection and dupes need what it drops.
func (f *File) DropSyntax() {
	f.AST, f.Fset = nil, nil
	for _, fn := range f.Funcs {
		fn.Decl, fn.Fset, fn.Text = nil, nil, nil
	}
}

// Func is one function or method with a body.
type Func struct {
	// ID is the identity used to match a function across the before and after
	// snapshots: package key, receiver type, and name. Several declarations
	// can share one (init functions); Changes matches those by content.
	ID string
	// Namespace and Name are crapper's join keys: the import path, or
	// importpath.Receiver for methods.
	Namespace string
	Name      string
	Receiver  string
	File      string
	// Start and End are physical lines, ignoring //line directives, as
	// coverage profiles record them.
	Start int
	End   int
	CC    int
	// Blocks are the coverage blocks go cover instruments for the function,
	// function literals inside it included.
	Blocks []coverlayout.Block
	// Body runs from the body's opening brace to just past its closing
	// brace; the function's profile blocks start inside it.
	Body [2]coverlayout.Pos
	// Text is the declaration's source, used to tell whether it changed.
	Text []byte
	Decl *ast.FuncDecl
	Fset *token.FileSet
}

// Label is the qualified display name: namespace.Name.
func (f *Func) Label() string {
	return f.Namespace + "." + f.Name
}

// ParseFile parses src and extracts its functions. It returns a nil File and
// no error for generated code. mods resolves the file's import path.
func ParseFile(filePath string, src []byte, mods Modules) (*File, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, usererr.Wrap(fmt.Errorf("parse %s: %w", filePath, err), "fix the syntax error first (in the staged mode, stage the fix too)")
	}
	if ast.IsGenerated(file) {
		return nil, nil
	}
	pkgPath, ok := mods.ImportPath(path.Dir(filePath), file.Name.Name)
	key := pkgPath
	if !ok {
		key = path.Dir(filePath) + ":" + file.Name.Name
	}
	out := &File{
		Path:     filePath,
		Package:  pkgPath,
		InModule: ok,
		Fset:     fset,
		AST:      file,
	}
	layout, err := coverlayout.File(fset, file, src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	out.Layout = layout.All
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Body == nil {
			continue
		}
		f := newFunc(fset, src, filePath, pkgPath, key, fn)
		f.Blocks = layout.Funcs[fn]
		out.Funcs = append(out.Funcs, f)
	}
	return out, nil
}

func newFunc(fset *token.FileSet, src []byte, filePath, pkgPath, key string, fn *ast.FuncDecl) *Func {
	start := fset.PositionFor(fn.Pos(), false)
	end := fset.PositionFor(fn.End(), false)
	f := &Func{
		Name:      fn.Name.Name,
		Namespace: pkgPath,
		File:      filePath,
		Start:     start.Line,
		End:       end.Line,
		CC:        Complexity(fn),
		Body:      [2]coverlayout.Pos{physical(fset, fn.Body.Lbrace), physical(fset, fn.Body.Rbrace+1)},
		Text:      bytes.Clone(src[start.Offset:end.Offset]),
		Decl:      fn,
		Fset:      fset,
	}
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		f.Receiver = ReceiverType(fn.Recv.List[0].Type)
	}
	f.ID = key + "." + f.Name
	if f.Receiver != "" {
		f.Namespace = pkgPath + "." + f.Receiver
		f.ID = key + "." + f.Receiver + "." + f.Name
	}
	return f
}

// physical is p's line and column ignoring //line directives, as go cover
// records them.
func physical(fset *token.FileSet, p token.Pos) coverlayout.Pos {
	at := fset.PositionFor(p, false)
	return coverlayout.Pos{Line: at.Line, Col: at.Column}
}

// ReceiverType is the receiver's type name without pointer or type
// arguments: Widget for (w Widget), (w *Widget) and (w *Widget[T]).
func ReceiverType(expr ast.Expr) string {
	switch t := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return ReceiverType(t.X)
	case *ast.IndexExpr:
		return ReceiverType(t.X)
	case *ast.IndexListExpr:
		return ReceiverType(t.X)
	}
	return ""
}

// Complexity is McCabe complexity as crapper counts it for Go: 1, plus one
// for each if, for, range, case clause (default included), select clause
// (default included), && and ||. Function literals nested in fn count
// toward fn.
func Complexity(fn ast.Node) int {
	cc := 1
	ast.Inspect(fn, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			cc++
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				cc++
			}
		}
		return true
	})
	return cc
}

// SortChanges orders changes by file and line.
func SortChanges(changes []Change) {
	sort.SliceStable(changes, func(i, j int) bool {
		a, b := changes[i].After, changes[j].After
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Start < b.Start
	})
}
