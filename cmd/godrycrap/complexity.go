package main

import (
	"flag"
	"io"
	"strconv"

	"github.com/tfournet/GoDryCrap/internal/analysis"
	"github.com/tfournet/GoDryCrap/internal/gate"
	"github.com/tfournet/GoDryCrap/internal/report"
)

type complexityRow struct {
	Status    string `json:"status"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	File      string `json:"file"`
	Line      int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Before    *int   `json:"before_cc"`
	CC        int    `json:"cc"`
	Ratchet   bool   `json:"ratchet,omitempty"`
}

type thresholds struct {
	warn, fail float64
	ratchet    bool
}

func (t *thresholds) register(fs *flag.FlagSet, what, ratchet string) {
	fs.Float64Var(&t.warn, "warn", 9, "warn when "+what+" is at or above this")
	fs.Float64Var(&t.fail, "fail", 10, "fail when "+what+" is at or above this")
	fs.BoolVar(&t.ratchet, "ratchet", false, ratchet)
}

// ratcheted reports whether a failing change is excused by the ratchet: the
// function existed with CC already at the fail level, and CC did not rise.
func (t *thresholds) ratcheted(c analysis.Change) bool {
	return t.ratchet && c.Before != nil && float64(c.Before.CC) >= t.fail && c.After.CC <= c.Before.CC
}

func runComplexity(args []string, stdout, stderr io.Writer) gate.Status {
	fs := flag.NewFlagSet("complexity", flag.ContinueOnError)
	var o options
	var th thresholds
	o.register(fs)
	th.register(fs, "CC", "a function whose CC was already at the fail level and did not rise warns instead of failing")
	if done, err := parse(fs, &o, args, stderr); done || err != nil {
		return parseStatus(stderr, "complexity", err)
	}
	t, err := o.load()
	if err != nil {
		return fail(stderr, "complexity", err)
	}
	rows, statuses := complexityRows(t.changes, th)
	table := report.Table{
		Title:   "godrycrap complexity",
		Summary: counts(statuses),
		Columns: []string{"status", "cc", "before", "function", "location"},
	}
	for _, r := range rows {
		table.Rows = append(table.Rows, []string{r.Status, strconv.Itoa(r.CC), beforeText(r.Before), r.Namespace + "." + r.Name, r.File + ":" + strconv.Itoa(r.Line)})
	}
	if err := o.emit(stdout, table, map[string]any{"command": "complexity", "status": gate.Worst(statuses).String(), "functions": rows}); err != nil {
		return fail(stderr, "complexity", err)
	}
	return gate.Worst(statuses)
}

func complexityRows(changes []analysis.Change, th thresholds) ([]complexityRow, []gate.Status) {
	rows := make([]complexityRow, 0, len(changes))
	statuses := make([]gate.Status, 0, len(changes))
	for _, c := range changes {
		status := gate.Level(float64(c.After.CC), th.warn, th.fail)
		excused := status == gate.Fail && th.ratcheted(c)
		if excused {
			status = gate.Warn
		}
		row := complexityRow{
			Status: status.String(), Namespace: c.After.Namespace, Name: c.After.Name,
			File: c.After.File, Line: c.After.Start, EndLine: c.After.End, CC: c.After.CC, Ratchet: excused,
		}
		if c.Before != nil {
			before := c.Before.CC
			row.Before = &before
		}
		rows = append(rows, row)
		statuses = append(statuses, status)
	}
	return rows, statuses
}

func beforeText(before *int) string {
	if before == nil {
		return "new"
	}
	return strconv.Itoa(*before)
}

func parseStatus(stderr io.Writer, name string, err error) gate.Status {
	if err == nil {
		return gate.OK
	}
	return fail(stderr, name, err)
}
