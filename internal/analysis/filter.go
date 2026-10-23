// Package analysis extracts Go functions, their identity, and their cyclomatic
// complexity from source held in memory.
package analysis

import "strings"

// skipDirs are directory names whose contents are never scored. The first
// group matches crapper's and dryer's SKIP_DIRS and TEST_DIRS; ent and gen
// commonly hold generated code (for example, entgo and protobuf output).
var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, ".idea": true, ".venv": true,
	"venv": true, "node_modules": true, "vendor": true, "target": true,
	"dist": true, "build": true, "out": true, "coverage": true,
	".metrics": true, "testdata": true, "__pycache__": true, ".clj-kondo": true,
	"test": true, "tests": true, "spec": true, "specs": true, "__tests__": true,
	"ent": true, "gen": true,
}

// generatedDirs hold generated code wherever they appear, even below a path
// named explicitly.
var generatedDirs = map[string]bool{"ent": true, "gen": true}

// InGeneratedDir reports whether a slash-separated path has an ent or gen
// directory component.
func InGeneratedDir(path string) bool {
	parts := strings.Split(path, "/")
	for _, part := range parts[:len(parts)-1] {
		if generatedDirs[part] {
			return true
		}
	}
	return false
}

// SkipDir reports whether a directory with this base name is never scored.
func SkipDir(name string) bool {
	return skipDirs[name]
}

// IsGoSource reports whether a file name is a non-test Go source file.
func IsGoSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

// SkipPath reports whether a slash-separated path, relative to the repository
// or walk root, is excluded before it is parsed: non-Go files, _test.go files,
// and files under a skipped directory.
func SkipPath(path string) bool {
	parts := strings.Split(path, "/")
	if !IsGoSource(parts[len(parts)-1]) {
		return true
	}
	for _, part := range parts[:len(parts)-1] {
		if skipDirs[part] {
			return true
		}
	}
	return false
}
