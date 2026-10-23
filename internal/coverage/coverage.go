// Package coverage reads existing Go coverage profiles (go test
// -coverprofile) and scores functions with them. It never runs tests.
package coverage

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/tfournet/GoDryCrap/internal/analysis"
	"github.com/tfournet/GoDryCrap/internal/coverlayout"
)

// Block is one profile record: a source span, its statement count, and its
// hit count.
type Block struct {
	StartLine, StartCol int
	EndLine, EndCol     int
	Statements          int
	Count               int
}

// Profile is the blocks of one or more profiles, keyed by the profile's file
// name (an import path plus file name, for go test output).
type Profile struct {
	Files map[string][]Block
	// sources are the path segments of the files being scored, so a suffix
	// match cannot give one of them a longer one's blocks.
	sources [][]string
}

// ErrEmpty is wrapped by Load when a profile has no blocks.
var ErrEmpty = errors.New("coverage profile has no blocks")

// Load reads and merges the profiles at paths. A missing file, an empty
// profile, a missing or unknown mode header, and a malformed record are all
// errors. Records with the same span become one block that keeps the
// highest count. go tool cover sums the counts instead (or ORs them in set
// mode); both agree on whether a block was hit, which is all coverage here
// uses, so percentages are the same, but the merged counts are not.
func Load(paths []string) (*Profile, error) {
	if len(paths) == 0 {
		return nil, errors.New("no coverage profile given (-coverprofile is required)")
	}
	p := &Profile{Files: map[string][]Block{}}
	index := map[string]map[[4]int]int{}
	for _, name := range paths {
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("coverage profile: %w", err)
		}
		n, err := p.parse(data, index)
		if err != nil {
			return nil, fmt.Errorf("coverage profile %s: %w", name, err)
		}
		if n == 0 {
			return nil, fmt.Errorf("%w: %s", ErrEmpty, name)
		}
	}
	return p, nil
}

// Bind records the files being scored. A file outside any module matches a
// profile key by path suffix, and Bind lets that match skip a key that a
// longer bound path also ends (crapper's longer-source-owns rule): with
// a.go and p/a.go both scored, x/p/a.go belongs to p/a.go.
func (p *Profile) Bind(files []*analysis.File) {
	p.sources = p.sources[:0]
	for _, f := range files {
		p.sources = append(p.sources, segments(f.Path))
	}
}

// Blocks returns the profile's blocks for file. A file in a module matches
// only its own key, import path plus base name, so a profile of another
// module or package never stands in for it. A file outside any module
// matches the unique, shortest key whose trailing path segments are its
// path, unless a longer bound source owns that key.
func (p *Profile) Blocks(file *analysis.File) ([]Block, bool) {
	if file.InModule {
		blocks, ok := p.Files[file.Package+"/"+path.Base(file.Path)]
		return blocks, ok
	}
	key, ok := p.suffixKey(file.Path)
	if !ok {
		return nil, false
	}
	return p.Files[key], true
}

// ErrNotInProfile is wrapped by For when the profile has no blocks for a
// file: the profile is for another shard, or for other packages.
var ErrNotInProfile = errors.New("coverage profile has no blocks for this file")

// ErrMismatch is wrapped when a profile's blocks do not fit the source being
// scored: the profile was made from different source.
var ErrMismatch = errors.New("coverage profile does not match the source")

// For returns file's blocks. It is an error when the profile has none for
// the file, or has a block that go cover would not instrument in this
// source.
func (p *Profile) For(file *analysis.File) ([]Block, error) {
	blocks, ok := p.Blocks(file)
	if !ok {
		return nil, fmt.Errorf("%w: %s (wrong shard, or a profile of other packages?)", ErrNotInProfile, file.Path)
	}
	want := spans(file.Layout)
	for _, b := range blocks {
		if n, ok := want[spanOf(b)]; !ok || n != b.Statements {
			return nil, fmt.Errorf("%w: %s has a block %s that go cover would not produce from this source", ErrMismatch, file.Path, describe(b))
		}
	}
	return blocks, nil
}

type span [4]int

func spanOf(b Block) span { return span{b.StartLine, b.StartCol, b.EndLine, b.EndCol} }

func spanOfLayout(b coverlayout.Block) span {
	return span{b.Start.Line, b.Start.Col, b.End.Line, b.End.Col}
}

// spans maps each layout block's span to its statement count.
func spans(layout []coverlayout.Block) map[span]int {
	out := make(map[span]int, len(layout))
	for _, b := range layout {
		out[spanOfLayout(b)] = b.Statements
	}
	return out
}

func describe(b Block) string {
	return fmt.Sprintf("%d.%d,%d.%d with %d statements", b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.Statements)
}

// Check is an error unless the profile's blocks for fn (those starting in
// its body) are exactly the blocks go cover instruments for fn in this
// source: the same count, spans and statement counts. A function that was
// added, moved or edited after the profile was made fails it.
func Check(blocks []Block, fn *analysis.Func) error {
	want := spans(fn.Blocks)
	have := 0
	for _, b := range blocks {
		start := coverlayout.Pos{Line: b.StartLine, Col: b.StartCol}
		if start.Before(fn.Body[0]) || !start.Before(fn.Body[1]) {
			continue
		}
		have++
		if n, ok := want[spanOf(b)]; !ok || n != b.Statements {
			return fmt.Errorf("%w: %s.%s at %s:%d: the profile's block %s is not one this source has", ErrMismatch, fn.Namespace, fn.Name, fn.File, fn.Start, describe(b))
		}
	}
	if have != len(fn.Blocks) {
		return fmt.Errorf("%w: %s.%s at %s:%d: the profile has %d blocks for it, this source has %d", ErrMismatch, fn.Namespace, fn.Name, fn.File, fn.Start, have, len(fn.Blocks))
	}
	return nil
}

func (p *Profile) suffixKey(filePath string) (string, bool) {
	want := segments(filePath)
	found, extra := "", -1
	ambiguous := false
	for key := range p.Files {
		have := segments(key)
		if !hasSuffix(have, want) || p.ownedByLonger(have, len(want)) {
			continue
		}
		switch d := len(have) - len(want); {
		case extra < 0 || d < extra:
			found, extra, ambiguous = key, d, false
		case d == extra:
			ambiguous = true
		}
	}
	return found, extra >= 0 && !ambiguous
}

// ownedByLonger reports whether a bound source longer than n segments also
// ends key.
func (p *Profile) ownedByLonger(key []string, n int) bool {
	for _, src := range p.sources {
		if len(src) > n && hasSuffix(key, src) {
			return true
		}
	}
	return false
}

func segments(p string) []string {
	var out []string
	for _, part := range strings.Split(p, "/") {
		if part != "" && part != "." && part != ".." {
			out = append(out, part)
		}
	}
	return out
}

func hasSuffix(have, want []string) bool {
	if len(want) == 0 || len(want) > len(have) {
		return false
	}
	offset := len(have) - len(want)
	for i, part := range want {
		if have[offset+i] != part {
			return false
		}
	}
	return true
}

// Percent is crapper's per-function coverage: of the statements in blocks
// whose lines overlap start..end, the percentage in blocks with a hit. No
// such statements is 0%.
func Percent(blocks []Block, start, end int) float64 {
	total, covered := 0, 0
	for _, b := range blocks {
		if b.EndLine < start || b.StartLine > end {
			continue
		}
		total += b.Statements
		if b.Count > 0 {
			covered += b.Statements
		}
	}
	if total == 0 {
		return 0
	}
	return 100 * float64(covered) / float64(total)
}

// CRAP is CC² × (1 − coverage)³ + CC, with coverage a percentage.
func CRAP(cc int, percent float64) float64 {
	c := float64(cc)
	uncovered := 1 - percent/100
	return c*c*uncovered*uncovered*uncovered + c
}

// Round4 rounds to four decimal places the way crapper's round(x, 4) does:
// correctly rounded from the float's exact value.
func Round4(v float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 4, 64), 64)
	return r
}
