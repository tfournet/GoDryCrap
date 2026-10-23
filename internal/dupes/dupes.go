package dupes

import (
	"encoding/binary"
	"hash/fnv"
	"slices"
	"sort"
	"strconv"

	"github.com/tfournet/GoDryCrap/internal/analysis"
)

// Defaults. MinLines and MinNodes are dryer's; Threshold is 0.8
// (dryer defaults to 0.82).
const (
	DefaultThreshold = 0.8
	DefaultMinLines  = 4
	DefaultMinNodes  = 20
)

// Entry is one function ready to compare.
type Entry struct {
	Func  *analysis.Func
	Nodes int
	// prints is the sorted set of fingerprints: one per distinct subtree
	// and per distinct atom, each a hash of its printed form.
	prints []uint64
}

// Pair is two functions whose similarity reached the threshold.
type Pair struct {
	Score       float64
	Left, Right *Entry
}

// Entries normalizes each function in files and keeps those with at least
// minLines source lines and minNodes normalized nodes, as dryer does.
func Entries(files []*analysis.File, minLines, minNodes int) []*Entry {
	n := newNormalizer()
	var out []*Entry
	for _, file := range files {
		for _, fn := range file.Funcs {
			e := newEntry(n, fn)
			if fn.End-fn.Start+1 >= minLines && e.Nodes >= minNodes {
				out = append(out, e)
			}
		}
	}
	return out
}

func newEntry(n *normalizer, fn *analysis.Func) *Entry {
	f := &fingerprinter{set: map[uint64]struct{}{}}
	f.hash(n.Func(fn.Decl))
	prints := make([]uint64, 0, len(f.set))
	for h := range f.set {
		prints = append(prints, h)
	}
	slices.Sort(prints)
	return &Entry{Func: fn, Nodes: f.nodes, prints: prints}
}

// fingerprinter hashes a normalized tree bottom up. Equal subtrees hash
// equally, so the set of hashes stands in for dryer's set of printed
// subtrees; atoms and lists are tagged so they cannot collide.
type fingerprinter struct {
	set   map[uint64]struct{}
	nodes int
}

func (f *fingerprinter) hash(n *node) uint64 {
	f.nodes++
	h := fnv.New64a()
	if !n.isList() {
		h.Write([]byte{'a'})
		h.Write([]byte(n.atom))
	} else {
		h.Write([]byte{'l'})
		var buf [8]byte
		for _, kid := range n.kids {
			binary.LittleEndian.PutUint64(buf[:], f.hash(kid))
			h.Write(buf[:])
		}
	}
	sum := h.Sum64()
	f.set[sum] = struct{}{}
	return sum
}

// Jaccard is |a ∩ b| / |a ∪ b| over sorted sets; 0 when both are empty.
func Jaccard(a, b []uint64) float64 {
	shared := 0
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] == b[j]:
			shared++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	union := len(a) + len(b) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

// score is the pair's Jaccard similarity, or -1 when the set sizes alone
// rule out reaching threshold (the score is at most small/large).
func score(a, b *Entry, threshold float64) float64 {
	small, large := len(a.prints), len(b.prints)
	if small > large {
		small, large = large, small
	}
	if large == 0 || float64(small)/float64(large) < threshold {
		return -1
	}
	return Jaccard(a.prints, b.prints)
}

func sameSpan(a, b *Entry) bool { return spanOf(a) == spanOf(b) }

// AllPairs compares every entry with every other, as dryer does.
func AllPairs(entries []*Entry, threshold float64) []Pair {
	var out []Pair
	for i, left := range entries {
		for _, right := range entries[i+1:] {
			out = appendPair(out, left, right, threshold)
		}
	}
	Sort(out)
	return out
}

// Against compares each entry in changed with every entry in pool, reporting
// each unordered pair once. Entries present in both lists are recognized by
// span.
func Against(changed, pool []*Entry, threshold float64) []Pair {
	order := map[span]int{}
	for i, e := range changed {
		order[spanOf(e)] = i + 1
	}
	var out []Pair
	for i, left := range changed {
		for _, right := range pool {
			if j := order[spanOf(right)]; j > 0 && j-1 <= i {
				continue
			}
			out = appendPair(out, left, right, threshold)
		}
	}
	Sort(out)
	return out
}

type span struct {
	file       string
	start, end int
}

func spanOf(e *Entry) span { return span{e.Func.File, e.Func.Start, e.Func.End} }

func appendPair(out []Pair, left, right *Entry, threshold float64) []Pair {
	if sameSpan(left, right) {
		return out
	}
	if s := score(left, right, threshold); s >= threshold {
		out = append(out, Pair{Score: s, Left: left, Right: right})
	}
	return out
}

// Sort orders pairs by score, highest first, then by location.
func Sort(pairs []Pair) {
	sort.SliceStable(pairs, func(i, j int) bool {
		a, b := pairs[i], pairs[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Left.Func.File != b.Left.Func.File {
			return a.Left.Func.File < b.Left.Func.File
		}
		return a.Left.Func.Start < b.Left.Func.Start
	})
}

// Round rounds a score to four decimal places.
func Round(v float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 4, 64), 64)
	return r
}
