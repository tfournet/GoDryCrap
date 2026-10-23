// Package coverlayout computes the coverage blocks go cover instruments in a
// Go source file: the spans and statement counts a go test -coverprofile
// profile reports. It ports the block rules of cmd/cover in Go 1.27
// (cover.go: Visit, addCounters, codeRanges, mergeRangesWithinStatements,
// statementBoundary, endsBasicSourceBlock and dedup), so a profile can be
// checked block for block against the source it is said to describe.
package coverlayout

import (
	"bytes"
	"errors"
	"go/ast"
	"go/token"
)

// Pos is a 1-based line and byte column, ignoring //line directives, as go
// cover records them.
type Pos struct {
	Line, Col int
}

// Before reports whether p comes strictly before q.
func (p Pos) Before(q Pos) bool {
	return p.Line < q.Line || p.Line == q.Line && p.Col < q.Col
}

// Block is one coverage block: a span and its statement count.
type Block struct {
	Start, End Pos
	Statements int
}

// Layout is a file's blocks: those of each function declaration (function
// literals inside it included, as go cover counts them), and every block in
// the file, package-level function literals included.
type Layout struct {
	Funcs map[*ast.FuncDecl][]Block
	All   []Block
}

// File computes the layout of file, parsed from src with fset.
func File(fset *token.FileSet, file *ast.File, src []byte) (*Layout, error) {
	l := &layout{
		fset:  fset,
		src:   src,
		seen:  map[[2]Pos]bool{},
		funcs: map[*ast.FuncDecl][]Block{},
	}
	ast.Walk(l, file)
	if l.err != nil {
		return nil, l.err
	}
	return &Layout{Funcs: l.funcs, All: l.all}, nil
}

type layout struct {
	fset  *token.FileSet
	src   []byte
	seen  map[[2]Pos]bool
	funcs map[*ast.FuncDecl][]Block
	all   []Block
	// cur collects the blocks of the function being walked; nil outside one.
	cur *[]Block
	err error
}

func (l *layout) position(p token.Pos) token.Position {
	return l.fset.PositionFor(p, false)
}

func (l *layout) offset(p token.Pos) int {
	return l.position(p).Offset
}

// newCounter records a block, moving its end one column right while an
// identical span is already recorded (cover's dedup).
func (l *layout) newCounter(start, end token.Pos, statements int) {
	s, e := l.position(start), l.position(end)
	key := [2]Pos{{s.Line, s.Column}, {e.Line, e.Column}}
	for l.seen[key] {
		key[1].Col++
	}
	l.seen[key] = true
	b := Block{Start: key[0], End: key[1], Statements: statements}
	*l.cur = append(*l.cur, b)
	l.all = append(l.all, b)
}

// Visit follows cover's File.Visit.
func (l *layout) Visit(node ast.Node) ast.Visitor {
	if l.err != nil {
		return nil
	}
	if visit, ok := visitors[nodeType(node)]; ok {
		return visit(l, node)
	}
	return l
}

var errLostElse = errors.New("coverlayout: no else keyword after an if body")

// findText is cover's findText: the offset of text at or after pos,
// skipping comments.
func (l *layout) findText(pos token.Pos, text string) int {
	b := []byte(text)
	s := l.src
	for i := l.offset(pos); i < len(s); {
		switch {
		case bytes.HasPrefix(s[i:], b):
			return i
		case bytes.HasPrefix(s[i:], []byte("//")):
			i = skipLine(s, i)
		case bytes.HasPrefix(s[i:], []byte("/*")):
			if i = skipBlockComment(s, i); i < 0 {
				return 0
			}
		default:
			i++
		}
	}
	return -1
}

func skipLine(s []byte, i int) int {
	for i < len(s) && s[i] != '\n' {
		i++
	}
	return i
}

// skipBlockComment returns the offset after the comment starting at i, or
// -1 when it is not closed.
func skipBlockComment(s []byte, i int) int {
	end := bytes.Index(s[i+2:], []byte("*/"))
	if end < 0 {
		return -1
	}
	return i + 2 + end + 2
}
