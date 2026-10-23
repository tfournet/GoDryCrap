// Package snapshot loads Go files from a git diff (staged, or BASE..HEAD) or
// from explicit paths on disk.
package snapshot

import (
	"path"

	"github.com/tfournet/GoDryCrap/internal/analysis"
	"github.com/tfournet/GoDryCrap/internal/gitindex"
)

// Diff is a git diff: the changed files as the before and after trees hold
// them, and the functions the diff touched.
type Diff struct {
	Repo    *gitindex.Repo
	Before  []*analysis.File
	After   []*analysis.File
	Changes []analysis.Change
	// before is nil when there is no before tree (before the first commit).
	before, after *side
}

// side is one tree of the diff and the modules it declares. Each side's
// files are parsed with its own go.mod files, so a module that moved keeps
// its import path on both sides.
type side struct {
	// prefix names a path's blob: "<commit>:", or ":" for the index.
	prefix string
	// commit is the tree's commit, or "" for the index.
	commit string
	mods   analysis.Modules
}

// LoadDiff reads the diff of the repository containing dir. An empty spec is
// the staged diff (HEAD against the index); otherwise spec is BASE..HEAD or
// BASE...HEAD. Files that fail to parse in the before tree are treated as
// absent there; an after file that fails to parse is an error.
func LoadDiff(dir, spec string) (*Diff, error) {
	repo, err := gitindex.Open(dir)
	if err != nil {
		return nil, err
	}
	d := &Diff{Repo: repo}
	if err := d.resolve(spec); err != nil {
		return nil, err
	}
	for _, s := range []*side{d.before, d.after} {
		if err := d.loadModules(s); err != nil {
			return nil, err
		}
	}
	changed, err := repo.ChangedGoFiles(d.before.base(), d.after.commit)
	if err != nil {
		return nil, err
	}
	paths := scorable(changed)
	if d.Before, err = d.parse(d.before, paths, true); err != nil {
		return nil, err
	}
	if d.After, err = d.parse(d.after, paths, false); err != nil {
		return nil, err
	}
	d.Changes = analysis.Changes(d.Before, d.After)
	return d, nil
}

// resolve sets the two sides for spec: HEAD and the index for the staged
// diff, or the range's two commits.
func (d *Diff) resolve(spec string) error {
	d.after = &side{prefix: gitindex.Index}
	if spec == "" {
		if d.Repo.HasHEAD() {
			d.before = &side{prefix: "HEAD:", commit: "HEAD"}
		}
		return nil
	}
	base, head, err := d.Repo.ResolveRange(spec)
	if err != nil {
		return err
	}
	d.before = &side{prefix: base + ":", commit: base}
	d.after = &side{prefix: head + ":", commit: head}
	return nil
}

// base is the revision to diff from: the before commit, or "" when there
// is no before tree.
func (s *side) base() string {
	if s == nil {
		return ""
	}
	return s.commit
}

func scorable(paths []string) []string {
	var out []string
	for _, p := range paths {
		if !analysis.SkipPath(p) {
			out = append(out, p)
		}
	}
	return out
}

// parse reads paths from side s and parses them with its modules. Paths
// absent from the tree are skipped. With lenient set, a file that fails to
// parse is skipped too.
func (d *Diff) parse(s *side, paths []string, lenient bool) ([]*analysis.File, error) {
	if s == nil {
		return nil, nil
	}
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = s.prefix + p
	}
	blobs, err := d.Repo.Read(names)
	if err != nil {
		return nil, err
	}
	var files []*analysis.File
	for _, p := range paths {
		file, err := parseBlob(p, blobs[s.prefix+p], s.mods, lenient)
		if err != nil {
			return nil, err
		}
		if file != nil {
			files = append(files, file)
		}
	}
	return files, nil
}

func parseBlob(p string, src []byte, mods analysis.Modules, lenient bool) (*analysis.File, error) {
	if src == nil {
		return nil, nil
	}
	file, err := analysis.ParseFile(p, src, mods)
	if lenient {
		return file, nil
	}
	return file, err
}

// list lists side s's paths: everything when dir is "", or the entries
// directly in dir.
func (d *Diff) list(s *side, dir string) ([]string, error) {
	if s.commit != "" {
		return d.Repo.TreeFiles(s.commit, dir)
	}
	switch dir {
	case "":
		return d.Repo.IndexFiles()
	case ".":
		return d.Repo.IndexFiles(":(glob)*")
	}
	return d.Repo.IndexFiles(":(glob)" + dir + "/*")
}

// PackageFiles parses every non-test, non-generated Go file directly in dir
// (slash-separated, relative to the repository root) as the after tree
// holds it.
func (d *Diff) PackageFiles(dir string) ([]*analysis.File, error) {
	listed, err := d.list(d.after, dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range scorable(listed) {
		if path.Dir(p) == dir {
			paths = append(paths, p)
		}
	}
	return d.parse(d.after, paths, false)
}

// loadModules reads every go.mod in side s.
func (d *Diff) loadModules(s *side) error {
	if s == nil {
		return nil
	}
	listed, err := d.list(s, "")
	if err != nil {
		return err
	}
	var names []string
	for _, p := range listed {
		if path.Base(p) == "go.mod" {
			names = append(names, s.prefix+p)
		}
	}
	blobs, err := d.Repo.Read(names)
	if err != nil {
		return err
	}
	s.mods = analysis.Modules{}
	for name, data := range blobs {
		if mod := analysis.ModulePath(data); mod != "" {
			s.mods[path.Dir(name[len(s.prefix):])] = mod
		}
	}
	return nil
}
