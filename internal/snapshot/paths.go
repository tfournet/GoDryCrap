package snapshot

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/tfournet/GoDryCrap/internal/analysis"
	"github.com/tfournet/GoDryCrap/internal/usererr"
)

// pathHint is the hint for a path that cannot be read.
const pathHint = "check that each path exists and is readable"

// LoadPaths parses the Go files named by args, walking directories. Walked
// directories skip the same names crapper does, plus ent and gen. Named
// files and directories are skipped when they are tests, generated (by
// header), or under an ent or gen directory; other skip names such as
// testdata are scored when named, as crapper does. Paths are shown
// relative to the current directory. Syntax trees and source text are
// dropped once each file's functions are extracted.
func LoadPaths(args []string) ([]*analysis.File, error) {
	paths, err := ListPaths(args)
	if err != nil {
		return nil, err
	}
	var out []*analysis.File
	for _, pkg := range paths.Packages {
		files, err := paths.Load(pkg, false)
		if err != nil {
			return nil, err
		}
		out = append(out, files...)
	}
	return out, nil
}

// Paths is the Go source files named by args, grouped by directory (one
// package per directory), in path order.
type Paths struct {
	Packages [][]string
	cwd      string
	finder   *moduleFinder
}

// ListPaths finds the files LoadPaths would parse, without parsing them, so
// a caller can load and release one package at a time.
func ListPaths(args []string) (*Paths, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	files, err := goFiles(args)
	if err != nil {
		return nil, err
	}
	p := &Paths{cwd: cwd, finder: &moduleFinder{mods: analysis.Modules{}, roots: map[string]string{}}}
	byDir := map[string][]string{}
	var dirs []string
	for _, f := range files {
		dir := filepath.Dir(f)
		if byDir[dir] == nil {
			dirs = append(dirs, dir)
		}
		byDir[dir] = append(byDir[dir], f)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		p.Packages = append(p.Packages, byDir[dir])
	}
	return p, nil
}

// Load parses one package's files (absolute paths from Packages). With
// keepSyntax false, each file's syntax tree and source text are dropped.
func (p *Paths) Load(pkg []string, keepSyntax bool) ([]*analysis.File, error) {
	var out []*analysis.File
	for _, abs := range pkg {
		file, err := p.finder.parse(abs, displayPath(p.cwd, abs))
		if err != nil {
			return nil, err
		}
		if file == nil {
			continue
		}
		if !keepSyntax {
			file.DropSyntax()
		}
		out = append(out, file)
	}
	return out, nil
}

// goFiles expands args into the sorted, absolute paths of the Go source
// files to score.
func goFiles(args []string) ([]string, error) {
	seen := map[string]bool{}
	for _, arg := range args {
		found, err := expand(arg)
		if err != nil {
			return nil, err
		}
		for _, p := range found {
			abs, err := filepath.Abs(p)
			if err != nil {
				return nil, err
			}
			seen[abs] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func expand(arg string) ([]string, error) {
	info, err := os.Stat(arg)
	switch {
	case err != nil:
		return nil, usererr.Wrap(err, pathHint)
	case info.IsDir():
		files, err := walk(arg)
		return files, usererr.Wrap(err, pathHint)
	case analysis.IsGoSource(filepath.Base(arg)):
		return []string{arg}, nil
	}
	return nil, nil
}

// walk lists the Go source files under root, skipping excluded directories.
func walk(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p != root && analysis.SkipDir(d.Name()):
			return filepath.SkipDir
		case d.Type().IsRegular() && analysis.IsGoSource(d.Name()):
			found = append(found, p)
		}
		return nil
	})
	return found, err
}

// displayPath is abs relative to the working directory, or abs when it has
// no relative form.
func displayPath(cwd, abs string) string {
	if rel, err := filepath.Rel(cwd, abs); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(abs)
}

// moduleFinder records the go.mod files above directories on disk.
type moduleFinder struct {
	mods  analysis.Modules
	roots map[string]string
}

// parse reads and parses the file at abs, naming it display. A file under an
// ent or gen directory, below its module root (or display path outside any
// module), is skipped even when it was named directly.
func (m *moduleFinder) parse(abs, display string) (*analysis.File, error) {
	if analysis.InGeneratedDir(m.relative(abs, display)) {
		return nil, nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, usererr.Wrap(err, pathHint)
	}
	file, err := analysis.ParseFile(filepath.ToSlash(abs), src, m.mods)
	if file == nil || err != nil {
		return nil, err
	}
	file.Path = display
	for _, fn := range file.Funcs {
		fn.File = display
	}
	return file, nil
}

// relative is abs relative to its module root, or display outside a module.
func (m *moduleFinder) relative(abs, display string) string {
	root := m.root(filepath.Dir(abs))
	if root == "" {
		return display
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return display
	}
	return filepath.ToSlash(rel)
}

// root is the directory of the nearest go.mod at or above dir, or "".
func (m *moduleFinder) root(dir string) string {
	if r, ok := m.roots[dir]; ok {
		return r
	}
	r := ""
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if mod := analysis.ModulePath(data); err == nil && mod != "" {
		r = dir
		m.mods[filepath.ToSlash(dir)] = mod
	} else if parent := filepath.Dir(dir); parent != dir {
		r = m.root(parent)
	}
	m.roots[dir] = r
	return r
}
