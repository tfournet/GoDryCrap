package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// hookScript is the pre-commit hook as the README documents it.
func hookScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	start := strings.Index(readme, "```sh\n#!/bin/sh\ngodrycrap complexity\n")
	if start < 0 {
		t.Fatal("README has no pre-commit hook block")
	}
	body := readme[start+len("```sh\n"):]
	return body[:strings.Index(body, "```")]
}

// TestReadmeHookExitCodes runs the documented hook with a stub godrycrap
// for every complexity and dupes exit code.
func TestReadmeHookExitCodes(t *testing.T) {
	dir := t.TempDir()
	hook := filepath.Join(dir, "pre-commit")
	writeFile(t, hook, hookScript(t))
	stub := "#!/bin/sh\necho \"$1\" >> \"$STUB_LOG\"\n[ \"$1\" = complexity ] && exit \"$COMPLEXITY_EXIT\"\nexit \"$DUPES_EXIT\"\n"
	writeFile(t, filepath.Join(dir, "bin", "godrycrap"), stub)
	if err := os.Chmod(filepath.Join(dir, "bin", "godrycrap"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ complexity, dupes, want int }{
		{0, 0, 0},
		{1, 0, 0},
		{2, 0, 2},
		{3, 0, 3},
		{0, 3, 3},
		{1, 3, 3},
		{2, 3, 2},
	}
	for _, tc := range cases {
		log := filepath.Join(t.TempDir(), "calls")
		cmd := exec.Command("sh", hook)
		cmd.Env = append(os.Environ(),
			"PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
			"COMPLEXITY_EXIT="+strconv.Itoa(tc.complexity), "DUPES_EXIT="+strconv.Itoa(tc.dupes), "STUB_LOG="+log)
		got := 0
		var exit *exec.ExitError
		if err := cmd.Run(); errors.As(err, &exit) {
			got = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("complexity %d, dupes %d: hook exit = %d, want %d", tc.complexity, tc.dupes, got, tc.want)
		}
		if calls, _ := os.ReadFile(log); string(calls) != "complexity\ndupes\n" {
			t.Errorf("complexity %d: calls = %q, want both commands", tc.complexity, calls)
		}
	}
}
