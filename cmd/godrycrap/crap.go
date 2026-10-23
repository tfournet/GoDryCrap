package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tfournet/GoDryCrap/internal/analysis"
	"github.com/tfournet/GoDryCrap/internal/coverage"
	"github.com/tfournet/GoDryCrap/internal/gate"
	"github.com/tfournet/GoDryCrap/internal/report"
)

type crapRow struct {
	Status    string  `json:"status"`
	Namespace string  `json:"namespace"`
	Name      string  `json:"name"`
	File      string  `json:"file"`
	Line      int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Before    *int    `json:"before_cc"`
	CC        int     `json:"cc"`
	Coverage  float64 `json:"coverage"`
	CRAP      float64 `json:"crap"`
	Ratchet   bool    `json:"ratchet,omitempty"`
	level     gate.Status
}

// profileList is a repeatable -coverprofile flag.
type profileList []string

func (p *profileList) String() string { return strings.Join(*p, ",") }

func (p *profileList) Set(v string) error {
	*p = append(*p, v)
	return nil
}

type crapOptions struct {
	options
	thresholds
	profiles     profileList
	profiledOnly bool
}

func runCrap(args []string, stdout, stderr io.Writer) gate.Status {
	fs := flag.NewFlagSet("crap", flag.ContinueOnError)
	var o crapOptions
	o.options.register(fs)
	o.thresholds.register(fs, "CRAP", "a failing function that already existed and whose CC did not rise warns instead of failing (coverage before the change is unknown)")
	fs.Var(&o.profiles, "coverprofile", "existing go test coverage `profile` to read; repeat to merge shards (required)")
	fs.BoolVar(&o.profiledOnly, "profiled-only", false, "score only the changed files the profile covers; still an error when it covers none")
	if done, err := parse(fs, &o.options, args, stderr); done || err != nil {
		return parseStatus(stderr, "crap", err)
	}
	rows, err := o.score()
	if err != nil {
		return fail(stderr, "crap", err)
	}
	statuses := make([]gate.Status, len(rows))
	table := report.Table{
		Title:   "godrycrap crap",
		Columns: []string{"status", "crap", "cc", "coverage", "function", "location"},
	}
	for i, r := range rows {
		statuses[i] = r.level
		table.Rows = append(table.Rows, []string{r.Status, number(r.CRAP), strconv.Itoa(r.CC), number(r.Coverage) + "%", r.Namespace + "." + r.Name, r.File + ":" + strconv.Itoa(r.Line)})
	}
	table.Summary = counts(statuses)
	if err := o.emit(stdout, table, map[string]any{"command": "crap", "status": gate.Worst(statuses).String(), "functions": rows}); err != nil {
		return fail(stderr, "crap", err)
	}
	return gate.Worst(statuses)
}

// score loads the profile and the changed functions and scores each one.
func (o *crapOptions) score() ([]crapRow, error) {
	profile, err := coverage.Load(o.profiles)
	if err != nil {
		return nil, err
	}
	t, err := o.load()
	if err != nil {
		return nil, err
	}
	profile.Bind(t.files)
	blocks, err := o.fileBlocks(profile, t)
	if err != nil {
		return nil, err
	}
	rows := []crapRow{}
	for _, c := range t.changes {
		if b, ok := blocks[c.After.File]; ok {
			rows = append(rows, o.row(c, b))
		}
	}
	return rows, nil
}

// fileBlocks looks up the profile's blocks for each file with a changed
// function, and checks that each changed function has its own block.
// With -profiled-only, files the profile does not cover are left out, but
// it is still an error when it covers none of them.
func (o *crapOptions) fileBlocks(profile *coverage.Profile, t *target) (map[string][]coverage.Block, error) {
	byFile := map[string][]*analysis.Func{}
	for _, c := range t.changes {
		byFile[c.After.File] = append(byFile[c.After.File], c.After)
	}
	found := map[string][]coverage.Block{}
	for _, file := range t.files {
		fns, changed := byFile[file.Path]
		if !changed {
			continue
		}
		blocks, keep, err := o.lookup(profile, file, fns)
		if err != nil {
			return nil, err
		}
		if keep {
			found[file.Path] = blocks
		}
	}
	if len(byFile) > 0 && len(found) == 0 {
		return nil, fmt.Errorf("%w: none of the %d changed files (wrong shard?)", coverage.ErrNotInProfile, len(byFile))
	}
	return found, nil
}

// lookup is one file's blocks, checked against each changed function in it.
// keep is false for a file the profile does not cover under -profiled-only.
func (o *crapOptions) lookup(profile *coverage.Profile, file *analysis.File, fns []*analysis.Func) (blocks []coverage.Block, keep bool, err error) {
	blocks, err = profile.For(file)
	if errors.Is(err, coverage.ErrNotInProfile) && o.profiledOnly {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	for _, fn := range fns {
		if err := coverage.Check(blocks, fn); err != nil {
			return nil, false, err
		}
	}
	return blocks, true, nil
}

func (o *crapOptions) row(c analysis.Change, blocks []coverage.Block) crapRow {
	fn := c.After
	pct := coverage.Round4(coverage.Percent(blocks, fn.Start, fn.End))
	score := coverage.Round4(coverage.CRAP(fn.CC, pct))
	status := gate.Level(score, o.warn, o.fail)
	excused := status == gate.Fail && o.ratchet && c.Before != nil && fn.CC <= c.Before.CC
	if excused {
		status = gate.Warn
	}
	row := crapRow{
		Status: status.String(), Namespace: fn.Namespace, Name: fn.Name, File: fn.File,
		Line: fn.Start, EndLine: fn.End, CC: fn.CC, Coverage: pct, CRAP: score, Ratchet: excused, level: status,
	}
	if c.Before != nil {
		before := c.Before.CC
		row.Before = &before
	}
	return row
}

func number(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
