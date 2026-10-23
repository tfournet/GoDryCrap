package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tfournet/GoDryCrap/internal/gate"
)

// repo is a temporary git repository with a go.mod for module example.com/m.
type repo struct {
	t   *testing.T
	dir string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write("go.mod", "module example.com/m\n\ngo 1.27\n")
	r.git("add", "go.mod")
	r.commit("init")
	return r
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *repo) write(path, content string) {
	r.t.Helper()
	writeFile(r.t, filepath.Join(r.dir, path), content)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (r *repo) fixture(path, name string) {
	r.t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		r.t.Fatal(err)
	}
	r.write(path, string(data))
}

// stage writes the files and adds them to the index.
func (r *repo) stage(files map[string]string) {
	r.t.Helper()
	for path, content := range files {
		r.write(path, content)
		r.git("add", path)
	}
}

func (r *repo) commit(msg string) string {
	r.t.Helper()
	r.git("commit", "-q", "--allow-empty", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

// result is one in-process run of the command line.
type result struct {
	status         gate.Status
	stdout, stderr string
}

func (r *repo) run(args ...string) result {
	r.t.Helper()
	return runIn(r.t, append([]string{args[0], "-C", r.dir}, args[1:]...)...)
}

func runIn(t *testing.T, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	status := run(args, &stdout, &stderr)
	return result{status: status, stdout: stdout.String(), stderr: stderr.String()}
}

func (res result) want(t *testing.T, status gate.Status, substrings ...string) {
	t.Helper()
	if res.status != status {
		t.Errorf("status = %v, want %v\nstdout:\n%s\nstderr:\n%s", res.status, status, res.stdout, res.stderr)
	}
	for _, s := range substrings {
		if !strings.Contains(res.stdout+res.stderr, s) {
			t.Errorf("output lacks %q\nstdout:\n%s\nstderr:\n%s", s, res.stdout, res.stderr)
		}
	}
}

func (res result) lacks(t *testing.T, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		if strings.Contains(res.stdout, s) {
			t.Errorf("output has %q\n%s", s, res.stdout)
		}
	}
}

// fn is a small Go function with n-1 if statements, so complexity n.
func fn(name string, n int) string {
	var b strings.Builder
	b.WriteString("func " + name + "(x int) int {\n")
	for i := 1; i < n; i++ {
		b.WriteString("\tif x > 0 {\n\t\tx--\n\t}\n")
	}
	b.WriteString("\treturn x\n}\n")
	return b.String()
}
