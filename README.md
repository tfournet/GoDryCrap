# GoDryCrap

GoDryCrap is a fast Go pre-commit hook and CI gate that checks the functions a change touches for cyclomatic complexity, CRAP score and copy-paste duplication. It keeps each change easy to read and safe to change, for the people and the AI coding agents who work on the code.

A commit that adds a branchy function and a copy-pasted helper:

```
$ git commit -m "pricing rules"
godrycrap v1.1.1 (go1.27.3)
godrycrap complexity: 3 functions, 1 fail, 0 warn
status  cc  before  function                            location
fail    10  1       example.com/shop/price.Discount     price/price.go:4
ok      3   new     example.com/shop/price.ShippingFee  price/price.go:28
ok      3   new     example.com/shop/price.ReturnFee    price/price.go:40
godrycrap v1.1.1 (go1.27.3)
godrycrap dupes: 1 candidate pairs at or above 0.8 (advisory)
score   function                            location              similar to                        location
1.0000  example.com/shop/price.ShippingFee  price/price.go:28-37  example.com/shop/price.ReturnFee  price/price.go:40-49
```

The commit is blocked until `Discount` is split. The duplicate pair is reported but does not block.

GoDryCrap is one Go binary that uses only the standard library. Its rules come from Uncle Bob's [crapper](https://github.com/unclebob/crapper) and [dryer](https://github.com/unclebob/dryer); see [Acknowledgements](#acknowledgements). More detail is in [docs/design.md](docs/design.md), [docs/STATUS.md](docs/STATUS.md) (features, fixtures and known limits), [docs/parity.md](docs/parity.md) and [docs/benchmarks.md](docs/benchmarks.md).

## The three checks

Complex code with little test coverage is where changes break things, and duplicated code is where fixes go missing. AI coding agents make both more likely: they write code fast, and when they see only part of a codebase they tend to grow long, branching functions and to add a near-copy of a helper instead of reusing the one that exists. They do their best work on small, well-tested functions they can understand in one read, where the tests catch their mistakes. A check that runs on every commit gives agents and people the same immediate, objective feedback, instead of leaving it to a reviewer later.

### Complexity

A function with many branches is hard to read and hard to change: each path through it is another case to hold in mind, and another place a change can break something without anyone noticing.

`godrycrap complexity` measures this as cyclomatic complexity (CC): start at 1, and add 1 for each `if`, `for`, `range`, `case` clause (default included), `select` clause (default included), `&&` and `||`. Function literals count toward the enclosing function. It warns at CC 9 and fails at CC 10 or more (`-warn`, `-fail`). Each changed function is listed with its CC after the change and before it (`new` when there was no before). It exits 1 when something warns and 2 when something fails; see [Exit codes](#exit-codes).

```sh
godrycrap complexity                     # staged diff, plain text for a hook
godrycrap complexity -diff origin/main...HEAD -format markdown
godrycrap complexity ./internal ./cmd     # paths: directories are walked recursively
```

### CRAP

Tests offset complexity. A test runs a path and pins down what it does, so a change that breaks the path fails the test instead of slipping through. CRAP (Change Risk Anti-Patterns) combines the two:

```text
CRAP = CC² × (1 − coverage)³ + CC
```

An untested function scores CC² + CC, so even CC 3 scores 12 and fails. At 100% coverage CRAP equals CC: tests make a function safe to change, but they cannot make 30 branches readable. `godrycrap crap` warns at CRAP 9 and fails at 10 or more (`-warn`, `-fail`), exiting 1 or 2 like `complexity`. A fail level of 10 is the target Uncle Bob [says he uses nowadays](https://x.com/unclebobmartin/status/2108946115169030596), much stricter than the original CRAP cut-off of 30. With the fail level at 10, a function needs more than this much coverage to pass:

| CC | Coverage needed |
| --- | --- |
| 1–2 | none |
| 3 | 8% |
| 4 | 28% |
| 5 | 42% |
| 6 | 52% |
| 7 | 61% |
| 8 | 69% |
| 9 | 77%, and it still warns |
| 10 or more | cannot pass; split the function |

Uncle Bob uses the same kind of gate on his own agents. He [writes](https://x.com/unclebobmartin/status/2108946115169030596) that "they are all _forced_ to thoroughly cover the code with unit tests. That enforcement comes from the crapper and mutator tools." Coverage shows that code ran, not that a test checked its result; mutation testing, which Uncle Bob's [mutator](https://github.com/unclebob/mutator) does, closes that gap. GoDryCrap does not do mutation testing.

`crap` reads an existing `go test -coverprofile` profile and never runs tests:

```sh
godrycrap crap -coverprofile coverage.out
godrycrap crap -coverprofile shard1.out -coverprofile shard2.out -diff origin/main...HEAD
```

The same shop example, with one test that covers part of `ShippingFee`:

```
$ godrycrap crap -coverprofile coverage.out ./price
godrycrap v1.1.1 (go1.27.3)
godrycrap crap: 3 functions, 2 fail, 0 warn
status  crap    cc  coverage  function                            location
fail    110     10  0%        example.com/shop/price.Discount     price/price.go:4
ok      3.3333  3   66.6667%  example.com/shop/price.ShippingFee  price/price.go:28
fail    12      3   0%        example.com/shop/price.ReturnFee    price/price.go:40
```

A function's coverage is the share of its statements, in profile blocks that overlap its lines, that ran at least once. A profile that cannot be trusted is an error (exit 3), never 0% coverage:

- no `-coverprofile`, or a profile file that does not exist;
- an empty profile (no blocks), a missing or unknown `mode:` header, or a malformed record: a non-positive or reversed span, a negative or overflowing count, trailing text, or the same span repeated with a different statement count;
- a changed file the profile has no blocks for (wrong shard, or a profile of another module or package), empty functions included;
- a profile that does not match the source being scored. GoDryCrap computes the blocks go cover would instrument in the current source (a port of cmd/cover's block rules from Go 1.27). Every profile block for the file must be one of them, with the same span and statement count, and each changed function must have exactly its computed blocks in the profile, no more and no fewer. This catches functions added, moved or edited after the profile was made, including a body that grew new branches while its first statement stayed put. Profiles must come from `go test` built with Go 1.27; an older toolchain splits blocks differently and is reported as a mismatch. An edit that keeps every block's span and statement count (renaming a variable to one of the same length, say) is not detected; see [docs/STATUS.md](docs/STATUS.md).

`-profiled-only` scores only the changed files the profile covers, for per-shard CI runs. It is still an error when the profile covers none of them.

### Duplicates

When the same logic lives in several places, a fix lands in one copy and the bug lives on in the others, and the codebase grows without getting more capable.

`godrycrap dupes` finds functions with nearly the same structure. Each function is normalized: call names, operators and the shape of the tree are kept, while local names, field names, type names in argument position and literals are dropped. Every subtree is fingerprinted, and two functions score the Jaccard similarity of their fingerprint sets (shared fingerprints over all fingerprints in either). Pairs at 0.8 or above are reported (`-threshold`); functions shorter than 4 lines (`-min-lines`) or 20 normalized nodes (`-min-nodes`) are skipped.

```sh
godrycrap dupes
godrycrap dupes -threshold 0.9 ./pkg/foo
```

It is advisory: it exits 0 unless there is an error. Functions are compared only within their package (directory). In the staged and range modes each changed function is compared with every function in its package (as the after tree holds it). In path mode every function is compared with every other function in its package; packages are loaded and released one at a time, so memory is bounded by the largest package, not the tree.

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

### Pre-commit hook

Save this as `.git/hooks/pre-commit` and make it executable. It blocks the commit when complexity fails (2) or errors (3); a warning (1) is shown but does not block. dupes always runs, and blocks only on an error (3). `cmd/godrycrap/readme_test.go` runs this exact script for every exit code.

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

### CI

GitHub Actions, per coverage shard:

```sh
godrycrap crap -summary -profiled-only -coverprofile coverage.out -diff "origin/$GITHUB_BASE_REF...HEAD"
```

This repo's own workflow (`.github/workflows/ci.yml`) runs gofmt, go vet, staticcheck and the tests, then runs all three commands on GoDryCrap itself, using its own coverage.

### Shared flags

| Flag | Meaning |
| --- | --- |
| `-format text\|markdown\|json\|tsv` | `text` (default) for hooks; `markdown` for `$GITHUB_STEP_SUMMARY`; `json` and `tsv` for parity scripts (namespace, name, file, lines, CC, coverage, CRAP; dupe pairs and scores) |
| `-summary` | also append the Markdown to `$GITHUB_STEP_SUMMARY` when it is set |
| `-diff BASE..HEAD` | score a commit range instead of the staged diff |
| `-C dir` | run git in `dir` |

### Ratchet

All three commands score only what a change touches, so existing complex code never blocks an unrelated commit. A function that is already over the line still fails when it is edited. `-ratchet` (off by default) softens that: in `complexity`, a function whose CC was already at the fail level before the change, and did not rise, warns instead of failing. In `crap`, a failing function that already existed and whose CC did not rise warns instead of failing; coverage before the change is not known, so the crap ratchet is on CC.

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

## How it stays fast

A hook that takes more than a couple of seconds stops being run, so GoDryCrap was built for that loop:

- **Changed functions only.** In the staged and range modes it reads only the files a diff touched (and, for `dupes`, the rest of their packages), and scores only the functions the diff changed.
- **Existing coverage.** `crap` reads the profile a CI test job already writes instead of rerunning tests. That takes 6–27 ms per profile, including the check of every block.
- **Strict profile checks.** Because tests are not rerun, a profile that is missing, malformed, from the wrong shard or older than the source is an error, not a silent 0% (see [CRAP](#crap)).

On a private Go monorepo of 39,770 functions, `complexity` over the whole tree takes 1.48 s, against 32.3 s for `crapper --no-coverage`. On real staged diffs from that monorepo a hook run takes 39–94 ms for `complexity` and 0.3–1.1 s for `dupes`. Uncle Bob's crapper and dryer are Python tools that cover six languages, and by default crapper regenerates coverage on each run; GoDryCrap brings their rules for Go into one binary built for the hook. See [docs/benchmarks.md](docs/benchmarks.md).

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
