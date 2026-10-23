// Command godrycrap checks changed Go functions for cyclomatic complexity,
// CRAP score and structural duplication.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/tfournet/GoDryCrap/internal/gate"
)

const usage = `Usage: godrycrap <command> [flags] [path ...]

Commands:
  complexity  cyclomatic complexity of changed functions
  crap        CRAP score of changed functions from an existing coverage profile
  dupes       structural near-duplicates of changed functions (advisory)

With no paths, each command scores the functions the staged diff touched
(HEAD against the git index; the working tree is never read), or the
functions a -diff BASE..HEAD range touched. With paths, it scores every
function in the named files and directories on disk. Flags may come
before or after paths; put a path that starts with - after --.

Exit codes: 0 ok, 1 warn, 2 fail, 3 error (usage, git, parse, or a missing,
empty or mismatched coverage profile). dupes exits 0 or 3.

Run 'godrycrap <command> -h' for the command's flags, and
'godrycrap version' (or -version) for the version.
`

type command func(args []string, stdout, stderr io.Writer) gate.Status

var commands = map[string]command{
	"complexity": runComplexity,
	"crap":       runCrap,
	"dupes":      runDupes,
}

func main() {
	os.Exit(int(run(os.Args[1:], os.Stdout, os.Stderr)))
}

// run runs the command line. A panic is a bug: it is reported with the bug
// hint and exits 3.
func run(args []string, stdout, stderr io.Writer) (status gate.Status) {
	cmdline = args
	defer func() {
		if r := recover(); r != nil {
			name := "godrycrap"
			if len(args) > 0 {
				name = args[0]
			}
			fmt.Fprintf(stderr, "godrycrap %s: panic: %v\n%s", name, r, debug.Stack())
			fmt.Fprint(stderr, bugHint(name, fmt.Sprintf("panic: %v", r), args))
			status = gate.Error
		}
	}()
	return dispatch(args, stdout, stderr)
}

func dispatch(args []string, stdout, stderr io.Writer) gate.Status {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return gate.Error
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return gate.OK
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, version())
		return gate.OK
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "godrycrap: unknown command %q\n\n%s", args[0], usage)
		return gate.Error
	}
	return cmd(args[1:], stdout, stderr)
}
