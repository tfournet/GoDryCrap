package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/tfournet/GoDryCrap/internal/analysis"
	"github.com/tfournet/GoDryCrap/internal/gate"
	"github.com/tfournet/GoDryCrap/internal/report"
	"github.com/tfournet/GoDryCrap/internal/snapshot"
	"github.com/tfournet/GoDryCrap/internal/usererr"
)

// options are the flags every command takes.
type options struct {
	format  string
	diff    string
	dir     string
	summary bool
	paths   []string
}

func (o *options) register(fs *flag.FlagSet) {
	fs.StringVar(&o.format, "format", "text", "output format: "+strings.Join(report.Formats, ", "))
	fs.StringVar(&o.diff, "diff", "", "score the functions changed in `BASE..HEAD` (or BASE...HEAD for the merge base) instead of the staged diff")
	fs.StringVar(&o.dir, "C", ".", "run git in `dir`")
	fs.BoolVar(&o.summary, "summary", false, "also append Markdown to $GITHUB_STEP_SUMMARY when it is set")
}

// parse parses args and checks the shared flags. A nil error with done set
// means -h was given.
func parse(fs *flag.FlagSet, o *options, args []string, stderr io.Writer) (done bool, err error) {
	fs.SetOutput(stderr)
	if o.paths, err = interleaved(fs, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, nil
		}
		return false, usererr.Wrap(err, flagHint(fs))
	}
	if !report.ValidFormat(o.format) {
		return false, usererr.Wrap(fmt.Errorf("unknown -format %q (want %s)", o.format, strings.Join(report.Formats, ", ")), flagHint(fs))
	}
	if o.diff != "" && len(o.paths) > 0 {
		return false, usererr.Wrap(errors.New("-diff and paths cannot be combined"), flagHint(fs))
	}
	return false, nil
}

func flagHint(fs *flag.FlagSet) string {
	return "run 'godrycrap " + fs.Name() + " -h' for its flags"
}

// interleaved parses flags that come before, between or after paths, and
// returns the paths. Everything after "--" is a path, so a path that starts
// with "-" goes after "--".
func interleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var paths []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return paths, nil
		}
		if consumed := len(args) - len(rest); consumed > 0 && args[consumed-1] == "--" {
			return append(paths, rest...), nil
		}
		paths = append(paths, rest[0])
		args = rest[1:]
	}
}

// target is what a command scores: the changed functions, and the diff they
// came from (nil in path mode).
type target struct {
	changes []analysis.Change
	files   []*analysis.File
	diff    *snapshot.Diff
}

func (o *options) load() (*target, error) {
	if len(o.paths) > 0 {
		files, err := snapshot.LoadPaths(o.paths)
		if err != nil {
			return nil, err
		}
		t := &target{files: files}
		for _, file := range files {
			for _, fn := range file.Funcs {
				t.changes = append(t.changes, analysis.Change{After: fn})
			}
		}
		return t, nil
	}
	d, err := snapshot.LoadDiff(o.dir, o.diff)
	if err != nil {
		return nil, err
	}
	return &target{changes: d.Changes, files: d.After, diff: d}, nil
}

// emit appends the table's Markdown to $GITHUB_STEP_SUMMARY when -summary
// is set, then writes the table to stdout in the chosen format. Text and
// Markdown start with the version line, JSON gets version and go_version
// fields, and TSV stays a plain table.
func (o *options) emit(stdout io.Writer, t report.Table, payload map[string]any) error {
	if err := o.appendSummary(t); err != nil {
		return usererr.Wrap(err, "check that $GITHUB_STEP_SUMMARY names a writable file")
	}
	payload["version"] = moduleVersion(readBuildInfo())
	payload["go_version"] = runtime.Version()
	err := banner(stdout, o.format)
	if err == nil {
		err = report.Write(stdout, o.format, t, payload)
	}
	return usererr.Wrap(err, "check that standard output is writable")
}

// banner writes the version line that starts text and Markdown output.
func banner(w io.Writer, format string) error {
	var err error
	switch format {
	case "text":
		_, err = fmt.Fprintln(w, version())
	case "markdown":
		_, err = fmt.Fprintf(w, "%s\n\n", version())
	}
	return err
}

func (o *options) appendSummary(t report.Table) error {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if !o.summary || path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := t.Markdown(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func counts(statuses []gate.Status) string {
	var warn, failed int
	for _, s := range statuses {
		switch s {
		case gate.Warn:
			warn++
		case gate.Fail:
			failed++
		}
	}
	return fmt.Sprintf("%d functions, %d fail, %d warn", len(statuses), failed, warn)
}
