package gitindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRepo(t *testing.T) *Repo {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a b.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "a b.go").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestReadIndexBeforeFirstCommit(t *testing.T) {
	r := gitRepo(t)
	if r.HasHEAD() {
		t.Error("HasHEAD before the first commit")
	}
	blobs, err := r.Read([]string{":a b.go", ":missing.go", "HEAD:a b.go"})
	if err != nil {
		t.Fatal(err)
	}
	if string(blobs[":a b.go"]) != "package a\n" || len(blobs) != 1 {
		t.Errorf("blobs = %q", blobs)
	}
	changed, err := r.ChangedGoFiles("", "")
	if err != nil || len(changed) != 1 || changed[0] != "a b.go" {
		t.Errorf("changed = %q, %v", changed, err)
	}
}

func TestReadRejectsNewlinesAndTrees(t *testing.T) {
	r := gitRepo(t)
	if _, err := r.Read([]string{":a\nb.go"}); err == nil {
		t.Error("no error for a newline in a path")
	}
	blobs, err := r.Read(nil)
	if err != nil || len(blobs) != 0 {
		t.Errorf("Read(nil) = %v, %v", blobs, err)
	}
}

func TestResolveRangeErrors(t *testing.T) {
	r := gitRepo(t)
	for _, spec := range []string{"HEAD", "..HEAD", "a..", "nope..HEAD", "HEAD...nope"} {
		if _, _, err := r.ResolveRange(spec); err == nil {
			t.Errorf("ResolveRange(%q): no error", spec)
		}
	}
}
