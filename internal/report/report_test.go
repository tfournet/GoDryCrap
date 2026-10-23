package report

import (
	"strings"
	"testing"
)

var table = Table{
	Title:   "t",
	Summary: "2 rows",
	Columns: []string{"a", "b"},
	Rows:    [][]string{{"x|y", "1"}, {"tab\there", "line\nbreak"}},
}

func render(t *testing.T, format string, tb Table) string {
	t.Helper()
	var b strings.Builder
	if err := Write(&b, format, tb, map[string]int{"n": 1}); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestMarkdownEscapesPipes(t *testing.T) {
	got := render(t, "markdown", table)
	if !strings.Contains(got, `| x\|y | 1 |`) || !strings.HasPrefix(got, "### t\n\n2 rows\n\n| a | b |\n| --- | --- |\n") {
		t.Errorf("markdown =\n%s", got)
	}
	if got := render(t, "markdown", Table{Title: "t", Summary: "none"}); got != "### t\n\nnone\n\n" {
		t.Errorf("empty markdown = %q", got)
	}
}

func TestTSVCleansCells(t *testing.T) {
	if got := render(t, "tsv", table); got != "a\tb\nx|y\t1\ntab here\tline break\n" {
		t.Errorf("tsv = %q", got)
	}
}

func TestText(t *testing.T) {
	got := render(t, "text", table)
	if !strings.HasPrefix(got, "t: 2 rows\na    b\nx|y  1\n") {
		t.Errorf("text =\n%s", got)
	}
	if got := render(t, "text", Table{Title: "t", Summary: "none"}); got != "t: none\n" {
		t.Errorf("empty text = %q", got)
	}
}

func TestJSONAndUnknownFormat(t *testing.T) {
	if got := render(t, "json", table); got != "{\n  \"n\": 1\n}\n" {
		t.Errorf("json = %q", got)
	}
	if err := Write(&strings.Builder{}, "xml", table, nil); err == nil {
		t.Error("no error for unknown format")
	}
	if ValidFormat("xml") || !ValidFormat("tsv") {
		t.Error("ValidFormat is wrong")
	}
}
