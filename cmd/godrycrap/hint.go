package main

import (
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/tfournet/GoDryCrap/internal/gate"
	"github.com/tfournet/GoDryCrap/internal/usererr"
)

const issuesURL = "https://github.com/tfournet/GoDryCrap/issues"

// cmdline is the current run's arguments, for the bug hint.
var cmdline []string

// fail reports err and returns gate.Error. A user error gets its fix hint;
// any other error is a bug in godrycrap and gets the bug hint.
func fail(stderr io.Writer, name string, err error) gate.Status {
	fmt.Fprintf(stderr, "godrycrap %s: %v\n", name, err)
	if hint, ok := usererr.HintOf(err); ok {
		fmt.Fprintf(stderr, "godrycrap: hint: %s\n", hint)
		return gate.Error
	}
	fmt.Fprint(stderr, bugHint(name, err.Error(), cmdline))
	return gate.Error
}

// bugHint is the stderr text that asks for an issue. The body holds the
// version, platform and arguments only, never file contents, and paths in
// the arguments and the title are cut to their base names.
func bugHint(name, msg string, args []string) string {
	title := name + ": " + shortError(stripDirs(msg))
	body := fmt.Sprintf("godrycrap %s, %s, %s/%s, command: %s",
		moduleVersion(readBuildInfo()), runtime.Version(), runtime.GOOS, runtime.GOARCH,
		shellJoin(append([]string{"godrycrap"}, redactArgs(args)...)))
	return fmt.Sprintf("godrycrap: internal error. Report it: gh issue create -R tfournet/GoDryCrap --title %s --body %s\ngodrycrap: or open %s\n",
		shellQuote(title), shellQuote(body), issuesURL)
}

// pathInText matches a path with at least one directory separator.
var pathInText = regexp.MustCompile(`(?:\b[A-Za-z]:)?[\w.~@+-]*(?:[/\\][\w.~@+-]*)+`)

// stripDirs replaces each path in msg with its base name.
func stripDirs(msg string) string {
	return pathInText.ReplaceAllStringFunc(msg, baseName)
}

// redactArgs keeps flags and the subcommand and cuts every path-like
// argument or flag value to its base name.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		name, value, isFlag := strings.Cut(a, "=")
		switch {
		case isFlag && strings.HasPrefix(name, "-"):
			out[i] = name + "=" + baseName(value)
		case strings.HasPrefix(a, "-"):
			out[i] = a
		default:
			out[i] = baseName(a)
		}
	}
	return out
}

// baseName is the last element of a path that has a directory, or s
// without trailing separators when it has none. A path of separators only
// stays as it is.
func baseName(s string) string {
	trimmed := strings.TrimRight(s, `/\`)
	if trimmed == "" {
		return s
	}
	return trimmed[strings.LastIndexAny(trimmed, `/\`)+1:]
}

// shortError is the first line of msg, cut to 80 characters.
func shortError(msg string) string {
	msg, _, _ = strings.Cut(msg, "\n")
	if utf8.RuneCountInString(msg) <= 80 {
		return msg
	}
	return string([]rune(msg)[:77]) + "..."
}

// shellQuote quotes s for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = a
		if a == "" || strings.ContainsFunc(a, unsafeInShell) {
			quoted[i] = shellQuote(a)
		}
	}
	return strings.Join(quoted, " ")
}

func unsafeInShell(r rune) bool {
	return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./=:,+@%", r))
}
