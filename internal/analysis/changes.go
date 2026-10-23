package analysis

import "bytes"

// Change is one function the diff touched. Before is nil for a new
// function, including a renamed one.
type Change struct {
	Before *Func
	After  *Func
}

// Changes lists the after-snapshot functions that are new, whose source
// differs from before, or that moved to another file. A function that was
// only deleted yields nothing.
//
// Functions are matched by identity: package, receiver and name. When
// several declarations share an identity (init functions, say), they are
// matched by content rather than by position, so adding or removing one
// leaves the others' history intact.
func Changes(before, after []*File) []Change {
	old := byIdentity(before)
	var out []Change
	for id, fns := range byIdentity(after) {
		out = append(out, match(old[id], fns)...)
	}
	SortChanges(out)
	return out
}

func byIdentity(files []*File) map[string][]*Func {
	out := map[string][]*Func{}
	for _, file := range files {
		for _, fn := range file.Funcs {
			out[fn.ID] = append(out[fn.ID], fn)
		}
	}
	return out
}

// match pairs the before and after declarations of one identity. A single
// declaration on each side is the same function, edited or moved. With
// repeats, a declaration with the same text in the same file is unchanged;
// the same text in another file has moved; and a declaration that is the
// only one left on both sides, in its file or overall, was edited. The rest
// are new.
func match(before, after []*Func) []Change {
	if len(before) == 1 && len(after) == 1 {
		return changed(before[0], after[0])
	}
	m := &matcher{before: before, used: make([]bool, len(before))}
	rest := m.pass(after, func(b, a *Func) bool { return b.File == a.File && bytes.Equal(b.Text, a.Text) }, true)
	rest = m.pass(rest, func(b, a *Func) bool { return bytes.Equal(b.Text, a.Text) }, false)
	return m.leftovers(rest)
}

func changed(before, after *Func) []Change {
	if before.File == after.File && bytes.Equal(before.Text, after.Text) {
		return nil
	}
	return []Change{{Before: before, After: after}}
}

type matcher struct {
	before []*Func
	used   []bool
	out    []Change
}

// pass pairs each declaration in after with the first unused before for
// which same holds, and returns those left. Pairs are reported unless
// unchanged is set.
func (m *matcher) pass(after []*Func, same func(b, a *Func) bool, unchanged bool) []*Func {
	var rest []*Func
	for _, a := range after {
		i := m.find(func(b *Func) bool { return same(b, a) })
		if i < 0 {
			rest = append(rest, a)
			continue
		}
		m.used[i] = true
		if !unchanged {
			m.out = append(m.out, Change{Before: m.before[i], After: a})
		}
	}
	return rest
}

func (m *matcher) find(ok func(*Func) bool) int {
	for i, b := range m.before {
		if !m.used[i] && ok(b) {
			return i
		}
	}
	return -1
}

// leftovers pairs a declaration that is alone among the unmatched on both
// sides, first within its file and then overall, as an edit; the others are
// new.
func (m *matcher) leftovers(rest []*Func) []Change {
	for _, a := range rest {
		var prev *Func
		if i, ok := m.sole(rest, func(f *Func) bool { return f.File == a.File }); ok {
			prev = m.before[i]
		} else if i, ok := m.sole(rest, func(*Func) bool { return true }); ok {
			prev = m.before[i]
		}
		m.out = append(m.out, Change{Before: prev, After: a})
	}
	return m.out
}

// sole reports the one unused before declaration in scope, when rest also
// has exactly one declaration in scope.
func (m *matcher) sole(rest []*Func, scope func(*Func) bool) (int, bool) {
	afters := 0
	for _, a := range rest {
		if scope(a) {
			afters++
		}
	}
	found, n := -1, 0
	for i, b := range m.before {
		if !m.used[i] && scope(b) {
			found, n = i, n+1
		}
	}
	return found, afters == 1 && n == 1
}
