# Status

GoDryCrap v1.0.0-rc1.

## Features

- `complexity`, `crap` and `dupes` each work on the staged diff, on a `-diff BASE..HEAD` or `BASE...HEAD` range, and on explicit files and directories.
- Warn at 9 and fail at 10. `-ratchet` is off by default.
- `dupes` is advisory, at a Jaccard threshold of 0.8.
- Exit codes are 0 ok, 1 warn, 2 fail, 3 error. A missing, empty, malformed or wrong-shard coverage profile is exit 3, never 0%. A stale profile is exit 3 when its block layout differs from the current source: profiles are checked block for block against the layout cmd/cover (Go 1.27) instruments. See Known limits for the one case this does not catch.
- Output is plain text, Markdown (with `-summary` appending to `$GITHUB_STEP_SUMMARY`), JSON or TSV.

## Quality bar

- Unit tests in every package, plus the fixtures below.
- Dogfood: every function in GoDryCrap scores under CRAP 9 on its own coverage (`go test -coverpkg=./...`): 201 functions, none at warn or fail (highest CRAP 8.25), with 95.7% statement coverage.
- `gofmt`, `go vet` and `staticcheck` are clean. go.mod is `go 1.27` with `toolchain go1.27.3`.
- `.github/workflows/ci.yml` runs all of this on every push and pull request, then runs the three commands on the repo itself and on each pull request's changed functions.
- Parity with crapper and dryer, and timings, are in [parity.md](parity.md) and [benchmarks.md](benchmarks.md).

## Fixtures

| Fixture or behavior | Test |
| --- | --- |
| CC exactly 9 (warn), 10 (fail), 11 (fail, with a `select` inside a function literal) | `TestComplexityThresholdFixtures`, `TestThresholdFixtures` |
| Missing profile, no `-coverprofile` | `TestCrapMissingProfile` |
| Empty profile (mode line only, and a zero-byte file) | `TestCrapEmptyProfile` |
| Wrong-shard profile, with and without `-profiled-only` | `TestCrapWrongShardProfile` |
| Stale profile (block past the last line) | `TestCrapStaleProfile` |
| Malformed profile records | `TestCrapMalformedProfile` |
| Deletion-only change | `TestComplexityDeletionOnly` |
| Renamed function (new, no before) | `TestComplexityRenamedFunctionIsNew` |
| Moved function (keeps identity), renamed file | `TestComplexityMovedFunctionKeepsIdentity`, `TestComplexityRenamedFileKeepsIdentity` |
| Value, pointer, generic (one and two type parameters) and parenthesized receivers | `TestComplexityMethods`, `TestReceiversAndNamespaces`, `TestParenthesizedReceiver` |
| Generated code skipped (`ent/`, `gen/`, header) plus vendor and `_test.go` | `TestComplexityGeneratedSkipped`, `TestGeneratedHeaderSkipped` |
| Staged vs working tree (the index wins) | `TestComplexityIndexWinsOverWorkingTree` |
| dupes finds a near-copy above 0.8 and ignores dissimilar code | `TestNearCopyFoundAndDissimilarIgnored`, `TestDupesStagedNearCopyInPackage` |
| Markdown output and `$GITHUB_STEP_SUMMARY` | `TestComplexityMarkdownAndSummary`, `TestCrapMarkdown`, `TestDupesDiffRangeAndMarkdown`, `TestMarkdownEscapesPipes` |
| Ratchet off by default and on behind `-ratchet` | `TestComplexityRatchet`, `TestCrapRatchet` |
| base..head and base...head ranges | `TestComplexityDiffRange`, `TestCrapDiffRangeAndPaths`, `TestDupesDiffRangeAndMarkdown` |
| Stale profile: block outside every function, appended function, shifted body | `TestCrapStaleProfile` |
| Only empty functions changed, unrelated profile, with and without `-profiled-only` | `TestCrapOnlyEmptyFunctionsChangedUnrelatedProfile`, `TestCrapEmptyFunction` |
| Wrong-module profile key; root `a.go` vs nested `p/a.go` | `TestCrapWrongModuleProfile`, `TestCrapRootFileDoesNotTakeNestedCoverage`, `TestModuleFileNeverFallsBackToAnotherImportPath`, `TestLongerSourceOwnsSuffixMatch` |
| Malformed profiles: header, mode, coordinates, counts, trailing text, inconsistent duplicates | `TestLoadRejectsMalformedProfiles`, `TestCrapMalformedProfile` |
| Generic callee names kept in dupes | `TestGenericCalleeNamesKept` |
| `ent/`, `gen/` excluded for named roots, descendants and files | `TestComplexityPathModeSkipsGeneratedDirsWhenNamed`, `TestComplexityPathModeModuleUnderGenDir` |
| README pre-commit hook, complexity exits 0-3 and a dupes error | `TestReadmeHookExitCodes` |
| Path-mode dupes compares within each package and loads packages one at a time; path mode drops syntax trees | `TestDupesPathModeComparesPerPackage`, `TestListPathsGroupsByDirectory`, `TestLoadPathsDropsSyntax` |
| Flags accepted after paths; `--` ends flags | `TestFlagsAfterPaths` |
| crapper's Go cases from `tests/test_go.py`, with identical expected names, namespaces and CCs | `TestCrapperCountsTheSameDecisions`, `TestCrapperMethodsNamespacedByReceiverType`, `TestCrapperImportPathComesFromGoMod` |
| Profiles checked against the exact cmd/cover block layout: count, spans and statement counts per function | `TestMatchesGoCover` (76 blocks from a real go1.27.3 profile), `TestCrapExpandedBodyKeepsFirstStatement`, `TestCrapStaleProfile` (appended function, shifted body, true positive), `TestCrapBlockCountAndStatementMismatch` |
| Repeated declarations (init) matched by content, not position | `TestRepeatedInitsMatchByContent`, `TestComplexityRepeatedInits` (deletion-only and insertion beside an unchanged CC-12 init, with and without `-ratchet`) |
| Same-profile CRAP parity on crapper's Go cases | `TestCrapperCRAPOnSameProfile` |
| Function spans use physical lines, as coverage does | `TestCrapLineDirectives` (same-filename `//line` before a function, and one inside a function, with fresh profiles) |
| Each side of a diff is parsed with its own tree's go.mod files | `TestComplexityRelocatedNestedModule` (nested module moved with its declared path, staged and `A..B` / `A...B`) |

## Known limits

- dupes builds its trees from go/ast, using tree-sitter-go's node kinds. On the parity sample (see [parity.md](parity.md)) it matched dryer exactly, but these places may still differ in structure:
  - A call through an indexed value, such as `handlers[i](x)`, is read as a generic instantiation. This is the same ambiguity tree-sitter-go has, and the indexed name is kept.
  - Type conversions such as `[]byte(s)` are normalized as calls.
- The crap ratchet is on CC, because coverage before the change is not known.
- Stale-profile detection is structural. A profile passes only when each changed function's blocks are exactly the ones cmd/cover would instrument in the current source: same count, spans and statement counts. An edit that leaves every block's span and statement count unchanged, such as renaming a variable or changing an operator or literal within the same columns, is not detected, and the old hit counts are used. Closing this gap would need evidence of source identity alongside the profile, such as a source digest.
- Profiles must come from `go test` built with Go 1.27. An older toolchain's cmd/cover lays blocks out differently (for example, it does not split blocks at blank and comment lines), so its profiles are reported as mismatches.
- When several declarations share an identity (init functions) and a diff both adds one and edits another in the same file, the edited one cannot be told apart from the new one. Both are reported as new, so `-ratchet` does not excuse the edited one. Edits and additions in separate commits keep history.
- A path that starts with `-` must follow `--`.
