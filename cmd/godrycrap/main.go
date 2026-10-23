// Command godrycrap checks changed Go functions for cyclomatic complexity,
// CRAP score and structural duplication.
package main

import (
	"fmt"
	"io"
	"os"

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

Run 'godrycrap <command> -h' for the command's flags.
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

func run(args []string, stdout, stderr io.Writer) gate.Status {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return gate.Error
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, usage)
		return gate.OK
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "godrycrap: unknown command %q\n\n%s", args[0], usage)
		return gate.Error
	}
	return cmd(args[1:], stdout, stderr)
}
