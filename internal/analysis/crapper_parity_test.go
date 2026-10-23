package analysis

import (
	"os"
	"reflect"
	"testing"
)

// Ported from crapper's tests/test_go.py. Without a go.mod, crapper names
// the namespace after the package clause, as ParseFile does with no
// modules.
type crapperRow struct {
	Name, Namespace string
	CC              int
}

func crapperRows(t *testing.T, fixture string) []crapperRow {
	t.Helper()
	src, err := os.ReadFile("../../testdata/crapper/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	file := parse(t, fixture, string(src), nil)
	var rows []crapperRow
	for _, fn := range file.Funcs {
		rows = append(rows, crapperRow{fn.Name, fn.Namespace, fn.CC})
	}
	return rows
}

// test_counts_the_same_decisions_as_crap4go
func TestCrapperCountsTheSameDecisions(t *testing.T) {
	want := []crapperRow{{"Branchy", "sample", 5}}
	if got := crapperRows(t, "sample.go"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// test_methods_are_namespaced_by_receiver_type
func TestCrapperMethodsNamespacedByReceiverType(t *testing.T) {
	want := []crapperRow{{"Run", "sample.Widget", 3}, {"Simple", "sample", 1}}
	if got := crapperRows(t, "widget.go"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
