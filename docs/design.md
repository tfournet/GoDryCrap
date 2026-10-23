# Design

## Why

Uncle Bob's [crapper](https://github.com/unclebob/crapper) and [dryer](https://github.com/unclebob/dryer) measure CRAP scores and structural duplication, but they are Python tools. GoDryCrap does all three jobs (complexity, CRAP and duplicates) for Go in one small binary that uses only the standard library, so it can run on any developer's machine and in CI.

## Goals

1. **`complexity`**: cyclomatic complexity of the Go functions a change touches. Reads the staged snapshot (the git index), never the working tree. Warn at 9, fail at 10 or more.
2. **`crap`**: CRAP = CC² × (1 − coverage)³ + CC for changed functions, read from an existing coverage profile, such as the per-shard `coverage.out` a CI test job already writes. It never reruns tests. A missing, empty or mismatched profile is an error, not 0% coverage. Warn at 9, fail at 10.
3. **`dupes`**: dryer-style structural similarity (Jaccard over normalized AST fingerprints) of changed functions against the other functions in their package. Advisory only, threshold 0.8. Fast enough for a pre-commit hook (target under 2 s).

All three also work on a commit range (`-diff BASE..HEAD`) and on explicit files and directories. Exit codes are stable: 0 ok, 1 warn, 2 fail, 3 error. A ratchet for functions that were already over the fail level is available behind `-ratchet`, off by default. Output is plain text for hooks, Markdown for `$GITHUB_STEP_SUMMARY`, and JSON or TSV for scripts.

## Rules

- Complexity counting follows crapper's Go rules (`src/crapper/languages/golang.py`, 0BSD): 1, plus each `if`, `for`, `range`, switch or select clause, `&&` and `||`.
- Similarity follows dryer's approach (0BSD: `astnorm.py`, `shape.py`, `extract.py`, `scan.py`), ported onto go/ast. Local names, field names and literals are normalized away; called function names, operators and tree shape are kept.
- Only the Go standard library parser (go/ast, go/parser, go/token) is used: no Python and no tree-sitter.
- No code is copied from crap4go, which has no license file.

## Out of scope

Running tests, mutation testing, and languages other than Go.
