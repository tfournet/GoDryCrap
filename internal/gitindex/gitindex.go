// Package gitindex reads the staged snapshot (the git index) and HEAD without
// touching the working tree.
package gitindex

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// Repo is a git work tree, addressed by its top-level directory.
type Repo struct {
	Root string
}

// Open finds the repository that contains dir.
func Open(dir string) (*Repo, error) {
	out, err := run(dir, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	return &Repo{Root: strings.TrimSpace(string(out))}, nil
}

// HasHEAD reports whether HEAD names a commit; it does not before the first
// commit.
func (r *Repo) HasHEAD() bool {
	_, err := run(r.Root, nil, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	return err == nil
}

// ChangedGoFiles lists .go paths that differ between base and head,
// deletions included. Renames are reported as a deletion and an addition.
// An empty head compares base with the index; an empty base before the first
// commit compares the empty tree with the index.
func (r *Repo) ChangedGoFiles(base, head string) ([]string, error) {
	args := []string{"diff", "--name-only", "-z", "--no-renames"}
	switch {
	case head != "":
		args = append(args, base, head)
	case base != "":
		args = append(args, "--cached", base)
	default:
		args = append(args, "--cached")
	}
	out, err := run(r.Root, nil, append(args, "--", "*.go")...)
	if err != nil {
		return nil, err
	}
	return splitZ(out), nil
}

// ResolveRange turns "A..B" into commit IDs for A and B, and "A...B" into
// the merge base of A and B, and B.
func (r *Repo) ResolveRange(spec string) (base, head string, err error) {
	left, right, threeDot := strings.Cut(spec, "...")
	if !threeDot {
		var twoDot bool
		left, right, twoDot = strings.Cut(spec, "..")
		if !twoDot {
			return "", "", fmt.Errorf("diff range %q: want BASE..HEAD or BASE...HEAD", spec)
		}
	}
	if left == "" || right == "" {
		return "", "", fmt.Errorf("diff range %q: both ends are required", spec)
	}
	if head, err = r.commit(right); err != nil {
		return "", "", err
	}
	if !threeDot {
		base, err = r.commit(left)
		return base, head, err
	}
	out, err := run(r.Root, nil, "merge-base", left, right)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(string(out)), head, nil
}

func (r *Repo) commit(rev string) (string, error) {
	out, err := run(r.Root, nil, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("unknown revision %q", rev)
	}
	return strings.TrimSpace(string(out)), nil
}

// IndexFiles lists index paths matching the given pathspecs.
func (r *Repo) IndexFiles(pathspecs ...string) ([]string, error) {
	args := append([]string{"ls-files", "-z", "--cached", "--"}, pathspecs...)
	out, err := run(r.Root, nil, args...)
	if err != nil {
		return nil, err
	}
	return splitZ(out), nil
}

// TreeFiles lists the blob paths in commit rev. With dir set it lists only
// the files directly in dir; otherwise it lists the whole tree.
func (r *Repo) TreeFiles(rev, dir string) ([]string, error) {
	args := []string{"ls-tree", "-r", "-z", "--name-only", rev}
	if dir != "" {
		args = []string{"ls-tree", "-z", "--name-only", rev, "--", dir + "/"}
		if dir == "." {
			args = []string{"ls-tree", "-z", "--name-only", rev}
		}
	}
	out, err := run(r.Root, nil, args...)
	if err != nil {
		return nil, err
	}
	return splitZ(out), nil
}

// Index is the revision prefix that names a path's staged blob.
const Index = ":"

// Read returns the content of each object name ("HEAD:path" or ":path").
// Names that do not exist, or that are not blobs, are absent from the map.
func (r *Repo) Read(names []string) (map[string][]byte, error) {
	found := make(map[string][]byte, len(names))
	if len(names) == 0 {
		return found, nil
	}
	var input bytes.Buffer
	for _, name := range names {
		if strings.ContainsAny(name, "\n\r") {
			return nil, fmt.Errorf("path contains a newline: %q", name)
		}
		input.WriteString(name + "\n")
	}
	out, err := run(r.Root, &input, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(bytes.NewReader(out))
	for _, name := range names {
		content, ok, err := readObject(reader)
		if err != nil {
			return nil, fmt.Errorf("git cat-file %s: %w", name, err)
		}
		if ok {
			found[name] = content
		}
	}
	return found, nil
}

// readObject reads one `git cat-file --batch` record. ok is false for a
// missing object or one that is not a blob.
func readObject(reader *bufio.Reader) (content []byte, ok bool, err error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, false, errors.New("short output")
	}
	if strings.HasSuffix(header, " missing\n") || strings.HasSuffix(header, " ambiguous\n") {
		return nil, false, nil
	}
	fields := strings.Fields(header)
	if len(fields) != 3 {
		return nil, false, fmt.Errorf("bad header %q", header)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, false, fmt.Errorf("bad header %q", header)
	}
	content = make([]byte, size+1)
	if _, err := io.ReadFull(reader, content); err != nil {
		return nil, false, errors.New("short content")
	}
	return content[:size], fields[1] == "blob", nil
}

func splitZ(out []byte) []string {
	var paths []string
	for _, item := range bytes.Split(out, []byte{0}) {
		if len(item) > 0 {
			paths = append(paths, string(item))
		}
	}
	return paths
}

func run(dir string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.quotepath=off"}, args...)...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = exit.Error()
			}
			return nil, fmt.Errorf("git %s: %s", args[0], msg)
		}
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}
