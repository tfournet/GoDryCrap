package coverlayout

import (
	"go/ast"
	"go/scanner"
	"go/token"
	"sort"
)

// addCounters is cover's addCounters: the blocks at the top level of list,
// which runs from pos to blockEnd.
func (l *layout) addCounters(pos, insertPos, blockEnd token.Pos, list []ast.Stmt, extendToClosingBrace bool) {
	if len(list) == 0 {
		r := l.codeRanges(insertPos, blockEnd)[0]
		l.newCounter(r.pos, r.end, 0)
		return
	}
	list = append([]ast.Stmt(nil), list...)
	for {
		var last int
		var end token.Pos
		list, last, end, extendToClosingBrace = basicBlock(list, blockEnd, extendToClosingBrace)
		if extendToClosingBrace {
			end = blockEnd
		}
		if pos != end {
			for _, r := range mergeRangesWithinStatements(l.codeRanges(pos, end), list[:last]) {
				l.newCounter(r.pos, r.end, r.statements)
			}
		}
		list = list[last:]
		if len(list) == 0 {
			return
		}
		pos = list[0].Pos()
	}
}

// basicBlock finds the statement that ends the first basic block of list.
// It returns the list (with a non-control labeled statement split into the
// label and its statement, as cover does), the index after that statement,
// the block's end, and whether the block may still extend to the closing
// brace.
func basicBlock(list []ast.Stmt, blockEnd token.Pos, extend bool) ([]ast.Stmt, int, token.Pos, bool) {
	end := blockEnd
	for last, stmt := range list {
		end = statementBoundary(stmt)
		if !endsBasicSourceBlock(stmt) {
			continue
		}
		if label, ok := stmt.(*ast.LabeledStmt); ok && !isControl(label.Stmt) {
			split := *label
			split.Stmt = &ast.EmptyStmt{Semicolon: label.Stmt.Pos(), Implicit: true}
			end = label.Pos()
			list = append(list[:last:last], append([]ast.Stmt{&split, label.Stmt}, list[last+1:]...)...)
		}
		return list, last + 1, end, false
	}
	return list, len(list), end, extend
}

// codeRange is a span of executable code.
type codeRange struct {
	pos, end   token.Pos
	statements int
}

// codeRanges is cover's codeRanges: the parts of start..end that hold code,
// split at blank and comment-only lines; one empty range at start when
// there is no code.
func (l *layout) codeRanges(start, end token.Pos) []codeRange {
	startOffset := l.offset(start)
	src := l.src[startOffset:l.offset(end)]
	orig := l.fset.File(start)
	scanFile := token.NewFileSet().AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(scanFile, src, nil, 0)
	at := func(p token.Pos) token.Pos { return orig.Pos(startOffset + scanFile.Offset(p)) }
	var ranges []codeRange
	var codeStart token.Pos
	prevEndLine := 0
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if skipToken(tok, lit) {
			continue
		}
		startLine, endLine := tokenLines(scanFile, pos, tok, lit)
		if prevEndLine == 0 {
			codeStart = at(pos)
		} else if startLine > prevEndLine+1 {
			ranges = append(ranges, codeRange{pos: codeStart, end: at(scanFile.LineStart(prevEndLine + 1))})
			codeStart = at(pos)
		}
		prevEndLine = max(prevEndLine, endLine)
	}
	return closeRanges(ranges, scanFile, codeStart, prevEndLine, at, start, end)
}

// skipToken reports braces and automatic semicolons, which are not code.
func skipToken(tok token.Token, lit string) bool {
	return tok == token.LBRACE || tok == token.RBRACE || tok == token.SEMICOLON && lit == "\n"
}

// tokenLines is the first and last line of a token; only string literals
// span lines.
func tokenLines(f *token.File, pos token.Pos, tok token.Token, lit string) (int, int) {
	startLine := f.PositionFor(pos, false).Line
	if tok != token.STRING {
		return startLine, startLine
	}
	return startLine, f.PositionFor(pos+token.Pos(len(lit)), false).Line
}

func closeRanges(ranges []codeRange, f *token.File, codeStart token.Pos, prevEndLine int, at func(token.Pos) token.Pos, start, end token.Pos) []codeRange {
	switch {
	case prevEndLine == 0:
	case prevEndLine < f.LineCount():
		ranges = append(ranges, codeRange{pos: codeStart, end: at(f.LineStart(prevEndLine + 1))})
	default:
		ranges = append(ranges, codeRange{pos: codeStart, end: end})
	}
	if len(ranges) == 0 {
		return []codeRange{{pos: start, end: start}}
	}
	return ranges
}

// firstAtOrAfter is the index of the first statement starting at or after p.
func firstAtOrAfter(stmts []ast.Stmt, p token.Pos) int {
	return sort.Search(len(stmts), func(i int) bool { return stmts[i].Pos() >= p })
}

// insideStatement reports whether pos falls strictly inside a statement.
func insideStatement(pos token.Pos, stmts []ast.Stmt) bool {
	i := firstAtOrAfter(stmts, pos)
	return i > 0 && pos < stmts[i-1].End()
}

// mergeRangesWithinStatements is cover's: a range that starts inside a
// statement joins the previous range, and each range counts the statements
// that start in it.
func mergeRangesWithinStatements(ranges []codeRange, stmts []ast.Stmt) []codeRange {
	merged := make([]codeRange, 0, len(ranges))
	for _, r := range ranges {
		r.statements = firstAtOrAfter(stmts, r.end) - firstAtOrAfter(stmts, r.pos)
		if len(merged) > 0 && insideStatement(r.pos, stmts) {
			prev := &merged[len(merged)-1]
			prev.end = r.end
			prev.statements += r.statements
			continue
		}
		merged = append(merged, r)
	}
	return merged
}
