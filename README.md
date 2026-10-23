# GoDryCrap

A single Go binary that checks changed Go functions for cyclomatic complexity, CRAP score (from an existing coverage profile) and structural duplication. It is built to run as a fast pre-commit hook and as a CI gate.

See [docs/design.md](docs/design.md) for the design goals, [docs/STATUS.md](docs/STATUS.md) for features, test fixtures and known limits, and [docs/parity.md](docs/parity.md) and [docs/benchmarks.md](docs/benchmarks.md) for how it compares with crapper and dryer.

The complexity and similarity rules are ported from Uncle Bob's 0BSD-licensed [crapper](https://github.com/unclebob/crapper) and [dryer](https://github.com/unclebob/dryer); see [Acknowledgements](#acknowledgements).

## Why GoDryCrap

We wanted a pre-commit hook and a CI gate that keep new and changed Go functions simple and tested, and that flag copy-paste duplication before it lands. The bar is warn at 9 and fail at 10, for both cyclomatic complexity and CRAP. That follows the CRAP target of 10 that Uncle Bob [says he uses nowadays](https://x.com/unclebobmartin/status/2108946115169030596), and it is much stricter than the original CRAP cut-off of 30.

That gate had to meet a few requirements:

- **Fast enough for every commit.** It runs on each developer's machine with nothing but Go and git, and it has to finish well under a couple of seconds on a large monorepo, or people stop running it.
- **Only what changed.** It scores only the functions a change touches, so an existing backlog of complex code never blocks an unrelated commit. `-ratchet` (off by default) lets a function that was already over the line, and did not get worse, warn instead of fail.
- **Reuse coverage, never rerun tests.** `crap` reads the coverage profile a CI test job already writes. A profile that is missing or does not match the source is an error, never a silent 0%.

Uncle Bob's [crapper](https://github.com/unclebob/crapper) and [dryer](https://github.com/unclebob/dryer) had the right ideas: CRAP scores per function, and structural duplicate detection that ignores names and literals. They are Python tools that cover six languages, and by default crapper regenerates coverage on each run. GoDryCrap brings their rules for Go into a single standard-library Go binary built for that hook loop. On a private Go monorepo of 39,770 functions, `complexity` over the whole tree takes 1.48 s, against 32.3 s for `crapper --no-coverage`. On real staged diffs a hook run takes 39–94 ms for `complexity` and 0.3–1.1 s for `dupes`, and `crap` scores a coverage profile in 6–27 ms. See [docs/benchmarks.md](docs/benchmarks.md).

## Why DRY and CRAP matter

- **CRAP.** Complex code with little test coverage is where changes break things. Low complexity and good coverage keep each function easy to read, change and test.
- **DRY.** When the same logic lives in several places, a fix lands in one copy and the bug lives on in the others. The codebase grows without getting more capable.

Both matter even more with AI coding agents. Agents write code fast. When they see only part of a codebase, they tend to add a near-copy of a helper instead of finding and reusing the one that exists, and to grow long, branching functions. They also work best on small, well-tested functions they can understand in one read, where the tests catch their mistakes. Uncle Bob relies on the same kind of gate for his own agents. He [writes](https://x.com/unclebobmartin/status/2108946115169030596) that "they are all _forced_ to thoroughly cover the code with unit tests. That enforcement comes from the crapper and mutator tools."

A fast, automatic gate on every commit gives agents and people the same immediate, objective feedback, so problems don't wait for a reviewer to catch them later. That is why GoDryCrap runs as a pre-commit hook and scores only what changed.

## Install

The repo tracks the latest stable Go release (currently `go 1.27` with `toolchain go1.27.3` in go.mod). It uses only the standard library and needs nothing at run time beyond `git`.

```sh
go install github.com/tfournet/GoDryCrap/cmd/godrycrap@latest
# or, from a checkout:
go build -o godrycrap ./cmd/godrycrap
```

## Usage

```
godrycrap <complexity|crap|dupes> [flags] [path ...]
```

Flags may come before or after paths. Put a path that starts with `-` after `--`.

Each command picks the functions to score in one of three ways:

| Mode | How | Reads |
| --- | --- | --- |
| Staged (default) | no paths, no `-diff` | HEAD and the git index. The working tree is never read. |
| Range | `-diff BASE..HEAD`, or `-diff BASE...HEAD` for the merge base | the two commits |
| Paths | one or more files or directories | the files on disk; every function in them is scored |

In the staged and range modes a function is scored when the diff touched it: it is new, its declaration text changed, or it moved to another file. A function's identity is its package, receiver type and name, so a renamed function is new (with no "before") and a function moved to another file in the same package keeps its history. A change that only deletes code has nothing to score and exits 0.

### complexity

```sh
godrycrap complexity                     # staged diff, plain text for a hook
godrycrap complexity -diff origin/main...HEAD -format markdown
godrycrap complexity ./internal ./cmd     # paths: directories are walked recursively
```

Reports each changed function's CC after the change, and before it (`new` when there was no before). Warn at CC 9, fail at CC 10 or more (`-warn`, `-fail`).

`-ratchet` (off by default): a function whose CC was already at the fail level before the change, and did not rise, warns instead of failing.

### crap

```sh
godrycrap crap -coverprofile coverage.out
godrycrap crap -coverprofile shard1.out -coverprofile shard2.out -diff origin/main...HEAD
```

CRAP = CC² × (1 − coverage)³ + CC, using an existing `go test -coverprofile` profile. It never runs tests. Warn at CRAP 9, fail at 10 (`-warn`, `-fail`).

These are errors (exit 3), never 0% coverage:

- no `-coverprofile`, or a profile file that does not exist;
- an empty profile (no blocks), a missing or unknown `mode:` header, or a malformed record: a non-positive or reversed span, a negative or overflowing count, trailing text, or the same span repeated with a different statement count;
- a changed file the profile has no blocks for (wrong shard, or a profile of another module or package), empty functions included;
- a profile that does not match the source being scored. GoDryCrap computes the blocks go cover would instrument in the current source (a port of cmd/cover's block rules from Go 1.27). Every profile block for the file must be one of them, with the same span and statement count, and each changed function must have exactly its computed blocks in the profile, no more and no fewer. This catches functions added, moved or edited after the profile was made, including a body that grew new branches while its first statement stayed put. Profiles must come from `go test` built with Go 1.27; an older toolchain splits blocks differently and is reported as a mismatch. An edit that keeps every block's span and statement count (renaming a variable to one of the same length, say) is not detected; see [docs/STATUS.md](docs/STATUS.md).

`-profiled-only` scores only the changed files the profile covers, for per-shard CI runs. It is still an error when the profile covers none of them.

`-ratchet` (off by default): a failing function that already existed and whose CC did not rise warns instead of failing. Coverage before the change is not known, so the ratchet is on CC.

### dupes

```sh
godrycrap dupes
godrycrap dupes -threshold 0.9 ./pkg/foo
```

Advisory: it always exits 0 unless there is an error. Functions are compared only within their package (directory). In the staged and range modes each changed function is compared with every function in its package (as the after tree holds it). In path mode every function is compared with every other function in its package; packages are loaded and released one at a time, so memory is bounded by the largest package, not the tree. Flags: `-threshold` (default 0.8), `-min-lines` (default 4), `-min-nodes` (default 20).

### Shared flags

| Flag | Meaning |
| --- | --- |
| `-format text\|markdown\|json\|tsv` | `text` (default) for hooks; `markdown` for `$GITHUB_STEP_SUMMARY`; `json` and `tsv` for parity scripts (namespace, name, file, lines, CC, coverage, CRAP; dupe pairs and scores) |
| `-summary` | also append the Markdown to `$GITHUB_STEP_SUMMARY` when it is set |
| `-diff BASE..HEAD` | score a commit range instead of the staged diff |
| `-C dir` | run git in `dir` |

### Version

`godrycrap version` (or `-version`, `--version`) prints the version line, such as `godrycrap v1.1.0 (go1.27.3)`. A `go install ...@v1.1.0` binary reports its module version; a build from a checkout reports the commit (with `+dirty` for uncommitted changes) or `devel`.

Text and Markdown output start with that line. JSON output has `version` and `go_version` fields instead, and TSV output has no version line, so it stays a plain table. The `-summary` Markdown has no version line.

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | ok |
| 1 | warn: something reached the warn level, nothing reached fail |
| 2 | fail: something reached the fail level |
| 3 | error: usage, git or parse failure, or a missing, empty or mismatched coverage profile |

`dupes` exits only 0 or 3.

### What is skipped

- `_test.go` files (as in crapper's `discover.py`).
- Files under a directory named `ent` or `gen` (below the module root), even when the file or directory is named on the command line.
- When walking a directory: files under any directory crapper and dryer skip (`vendor`, `testdata`, `node_modules`, `build`, `dist`, `out`, `target`, `coverage`, `test`, `tests`, `spec`, `specs`, `__tests__`, `.git`, `.venv` and a few more; see `internal/analysis/filter.go`). These, except `ent` and `gen`, are scored when named directly, as crapper does.
- Files with the standard `// Code generated ... DO NOT EDIT.` header.
- Function declarations without a body.

A staged (or HEAD-side, in range mode) file that does not parse is an error. A before-side file that does not parse is treated as absent.

## Hook and CI

Pre-commit hook (`.git/hooks/pre-commit`). It blocks the commit when complexity fails (2) or errors (3); a warning (1) is shown but does not block. dupes always runs, and blocks only on an error (3). `cmd/godrycrap/readme_test.go` runs this exact script for every exit code.

```sh
#!/bin/sh
godrycrap complexity
complexity=$?
godrycrap dupes
dupes=$?
if [ "$complexity" -ge 2 ]; then
  exit "$complexity"
fi
exit "$dupes"
```

GitHub Actions, per coverage shard:

```sh
godrycrap crap -summary -profiled-only -coverprofile coverage.out -diff "origin/$GITHUB_BASE_REF...HEAD"
```

This repo's own workflow (`.github/workflows/ci.yml`) runs gofmt, go vet, staticcheck and the tests, then runs all three commands on GoDryCrap itself, using its own coverage.

## Differences from crapper and dryer

Complexity and CRAP:

- **Same counting.** 1, plus each `if`, `for`, `range`, `case` clause (default included), `select` clause (default included), `&&` and `||`. Function literals count toward the enclosing function. Spans run from `func` to the closing brace, and names follow crapper's namespaces (`import/path` or `import/path.Receiver`).
- **Coverage per function is crapper's.** It counts the statements in profile blocks whose lines overlap the function's lines, covered when the hit count is above 0, and 0% when there are no statements. Coverage and CRAP are rounded to four places, coverage first, as crapper does.
- **Missing coverage is an error.** crapper scores a file the profile does not mention at 0%. GoDryCrap treats that as an error (exit 3), or skips the file with `-profiled-only`. It also rejects a profile whose blocks do not fit the source (see above); crapper does not check.
- **Merged profiles count each block once.** When several records have the same block span (merged shards, or the per-test-binary copies a `-coverpkg` profile has), GoDryCrap keeps one block with the highest count. `go tool cover` keeps one block too, but sums the counts (ORs them in set mode). Whether a block was hit, and so every coverage percentage, is the same either way. crapper adds the statements of every copy, which lowers coverage when profiles overlap; see [docs/parity.md](docs/parity.md).
- **Profile paths match by import path.** A file in a module matches only `import/path/file.go`. A file outside any module falls back to a unique, shortest suffix match on path segments, with crapper's longer-source-owns rule. crapper uses suffix matching for every file.
- **Generated code is skipped.** crapper skips only test files and its skip directories. GoDryCrap also skips `ent/`, `gen/` and files with a `Code generated` header.
- **Thresholds.** Warn at 9 and fail at 10 replace crapper's single `--threshold`.

Duplicates:

- **Same pipeline.** Functions are normalized, every subtree is fingerprinted, and pairs are scored by Jaccard over the fingerprint sets. Call names, operators and tree shape are kept; local names, field names, type names in argument position and literals are dropped. `min-lines` 4 and `min-nodes` 20 are dryer's defaults.
- **Threshold.** The default is 0.8. dryer's default is 0.82.
- **The tree is built from go/ast, not tree-sitter.** The normalizer emits tree-sitter-go node kinds (`statement_list`, `expression_list`, `qualified_type`, `pointer_type`, `generic_type`, `literal_element` and so on) so its trees approximate dryer's. They are not guaranteed identical, so scores can differ slightly from dryer's.
- **Fingerprints are 64-bit hashes.** Each fingerprint is a structural hash of a subtree instead of its printed form. Equal subtrees hash equally, so the sets have the same sizes and overlaps, barring a 64-bit collision.
- **Comparisons stay within a package.** A changed function is compared with the functions in its own package; dryer's `--changed` compares changed files with each other only. In path mode GoDryCrap compares functions per package (directory); dryer compares every form across all the files it was given, so cross-package pairs dryer reports are not reported here.

## Reporting bugs

Bug reports from people and coding agents are welcome. When godrycrap hits an internal error or panics, it exits 3 and prints a ready-made command on stderr:

```
godrycrap: internal error. Report it: gh issue create -R tfournet/GoDryCrap --title '<command>: <error>' --body '<version>, <go version>, <os/arch>, command: <arguments>'
godrycrap: or open https://github.com/tfournet/GoDryCrap/issues
```

The body holds only the version, platform and arguments, never file contents, and the hint cuts every path in the arguments and the title down to its base name, so no directory structure goes into a public issue. Still read the text before you file it. Errors you can fix yourself (a bad flag, a missing, malformed or stale coverage profile, a directory that is not a git repository, a syntax error) print a `godrycrap: hint:` line that says how instead.

To report a bug, run that command or use the [bug report form](https://github.com/tfournet/GoDryCrap/issues/new?template=bug_report.yml), and include:

- the output of `godrycrap version`, and your OS and architecture;
- the command you ran and its exit code;
- what you expected and what happened, with the stderr output;
- a minimal repro: the smallest Go files (and coverage profile, if it matters) that show the bug.

Issues are public, so the repro must be made up. Never paste proprietary code, file contents or coverage profiles from a private project.

## Acknowledgements

GoDryCrap would not exist without the work of Robert C. Martin ([Uncle Bob](https://cleancoder.com)).

- **[crapper](https://github.com/unclebob/crapper)** (Python, 0BSD) computes CRAP = CC² × (1 − coverage)³ + CC per function for Clojure, Java, Go, TypeScript, Rust and Python. Its Go rules follow his earlier [crap4go](https://github.com/unclebob/crap4go), and its formula is the one from [crap4clj](https://github.com/unclebob/crap4clj) and [crap4java](https://github.com/unclebob/crap4java).
- **[dryer](https://github.com/unclebob/dryer)** (Python, 0BSD) finds candidate duplicate code in the same six languages. It scores pairs of functions by the Jaccard similarity of their normalized structural fingerprints. Its Clojure side follows his [dry4clj](https://github.com/unclebob/dry4clj).

He announced both, along with mutator and uml-viewer, [on X on October 10, 2026](https://x.com/unclebobmartin/status/2108914390980342164), and invited people to "use them as templates to build the tools that are customized to your needs. The licenses are all BSD Zero, meaning that there are no strings." GoDryCrap is one of those tools.

**Ported from crapper and dryer (0BSD).** Their rules were reimplemented in Go; no Python code was copied.

- From crapper: Go complexity counting (`src/crapper/languages/golang.py`); function names and namespaces (`import/path` or `import/path.Receiver`); per-function coverage from profile blocks, its rounding, and its longer-source-owns path rule; the skipped directories and `_test.go` files; and its Go test cases, kept as fixtures in `testdata/crapper`.
- From dryer: form normalization, subtree fingerprints and Jaccard scoring (`astnorm.py`, `shape.py`, `extract.py`, `scan.py`), the tree shape it builds from tree-sitter-go (approximated on go/ast), and its `min-lines` 4 and `min-nodes` 20 defaults.

**New in GoDryCrap.**

- Staged-snapshot reads from the git index, `-diff` ranges, and function identity across moves and renames.
- Separate warn and fail thresholds, stable exit codes, and `-ratchet`.
- Strict coverage checks: a missing, malformed or stale profile is an error, checked block for block against the layout cmd/cover would instrument.
- One merged block per span for `-coverpkg` and sharded profiles.
- Import-path profile matching, and skipping of generated code.
- A go/ast normalizer in place of tree-sitter, with 64-bit hashed fingerprints.
- Duplicate comparison within each package.
- Markdown, JSON and TSV output.

No code comes from crap4go, which has no license file. [docs/parity.md](docs/parity.md) shows where results match crapper and dryer exactly and where they differ on purpose.

**The CRAP metric** (Change Risk Anti-Patterns) was created by Alberto Savoia and Bob Evans, who launched it in 2007 with the Crap4j Eclipse plug-in and [crap4j.org](http://www.crap4j.org/). Savoia tells the story in [This Code is CRAP](https://testing.googleblog.com/2011/02/this-code-is-crap.html) on the Google Testing Blog, where a score above 30 marks a method as CRAPpy.

**The Go project.** `internal/coverlayout` ports the block rules of Go 1.27's `cmd/cover`, so profiles can be checked against the exact blocks `go test` instruments.

## License

GoDryCrap is released under the [0BSD license](LICENSE). The parts ported from crapper and dryer, and the fixtures in `testdata/crapper`, come from 0BSD sources; see [Acknowledgements](#acknowledgements). `internal/coverlayout` is derived from Go's `cmd/cover` and keeps the Go Authors' BSD-3-Clause notice in [internal/coverlayout/LICENSE-go](internal/coverlayout/LICENSE-go). [NOTICE](NOTICE) lists all third-party material.
