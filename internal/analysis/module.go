package analysis

import (
	"path"
	"strconv"
	"strings"
)

// ModulePath returns the module path declared in go.mod content, or "" when
// there is no module directive.
func ModulePath(gomod []byte) string {
	for _, line := range strings.Split(string(gomod), "\n") {
		line, _, _ = strings.Cut(line, "//")
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}
		if unquoted, err := strconv.Unquote(fields[1]); err == nil {
			return unquoted
		}
		return fields[1]
	}
	return ""
}

// Modules maps module root directories to module paths. Directories are
// slash-separated; "." is the root the paths are relative to.
type Modules map[string]string

// ImportPath is the import path of the package in dir, the way crapper names
// it: the module path plus the directory below the nearest go.mod. Without a
// go.mod above dir it is the package clause name, and ok is false.
func (m Modules) ImportPath(dir, pkgName string) (importPath string, ok bool) {
	dir = path.Clean(dir)
	for current := dir; ; current = path.Dir(current) {
		if mod, found := m[current]; found && mod != "" {
			if current == dir {
				return mod, true
			}
			rel := strings.TrimPrefix(dir, current)
			if current == "." {
				rel = dir
			}
			return mod + "/" + strings.TrimPrefix(rel, "/"), true
		}
		if current == "." || current == "/" {
			return pkgName, false
		}
	}
}
