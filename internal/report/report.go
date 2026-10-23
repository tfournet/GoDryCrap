// Package report renders result tables as plain text, Markdown, TSV or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Formats lists the accepted -format values.
var Formats = []string{"text", "markdown", "json", "tsv"}

// Table is one command's result: a heading, a one-line summary and rows.
type Table struct {
	Title   string
	Summary string
	Columns []string
	Rows    [][]string
}

// Write renders t in format; json renders payload instead of the table.
func Write(w io.Writer, format string, t Table, payload any) error {
	switch format {
	case "text":
		return t.Text(w)
	case "markdown":
		return t.Markdown(w)
	case "tsv":
		return t.TSV(w)
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	return fmt.Errorf("unknown format %q (want %s)", format, strings.Join(Formats, ", "))
}

// ValidFormat reports whether format is one of Formats.
func ValidFormat(format string) bool {
	for _, f := range Formats {
		if f == format {
			return true
		}
	}
	return false
}

// Text is the hook output: the summary, then aligned rows.
func (t Table) Text(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "%s: %s\n", t.Title, t.Summary); err != nil {
		return err
	}
	if len(t.Rows) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(t.Columns, "\t"))
	for _, row := range t.Rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

// Markdown is the $GITHUB_STEP_SUMMARY form.
func (t Table) Markdown(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n\n%s\n", t.Title, t.Summary)
	if len(t.Rows) > 0 {
		b.WriteString("\n")
		writeMarkdownRow(&b, t.Columns)
		sep := make([]string, len(t.Columns))
		for i := range sep {
			sep[i] = "---"
		}
		writeMarkdownRow(&b, sep)
		for _, row := range t.Rows {
			writeMarkdownRow(&b, row)
		}
	}
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func writeMarkdownRow(b *strings.Builder, cells []string) {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		escaped[i] = strings.ReplaceAll(cell, "|", `\|`)
	}
	b.WriteString("| " + strings.Join(escaped, " | ") + " |\n")
}

// TSV is a header row and one line per row, for parity scripts.
func (t Table) TSV(w io.Writer) error {
	var b strings.Builder
	b.WriteString(strings.Join(t.Columns, "\t") + "\n")
	clean := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	for _, row := range t.Rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			cells[i] = clean.Replace(cell)
		}
		b.WriteString(strings.Join(cells, "\t") + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
