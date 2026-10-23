# Parity with crapper and dryer

GoDryCrap was compared with Uncle Bob's [crapper](https://github.com/unclebob/crapper) and [dryer](https://github.com/unclebob/dryer) (`master`, Oct 2026) on three kinds of input:

- a sample of five packages from a large private Go monorepo (39,770 functions in total), parsed statically;
- GoDryCrap itself;
- crapper's own Go test cases.

Every scan of the private monorepo ran under `timeout` and a 2 GiB virtual-memory cap. Coverage profiles were written by `go test -coverprofile` with Go 1.27.3.

## Summary

| Check | Sample | Result |
| --- | --- | --- |
| Complexity vs `crapper --no-coverage` | 5 packages, 1,584 functions | 1,584 of 1,584 identical (names, namespaces and CC) |
| CRAP vs `crapper --use-existing-coverage`, same profile | 2 packages, 313 functions | 313 of 313 match |
| CRAP, same profile | GoDryCrap itself (`-coverpkg=./...`), 158 functions | 158 of 158 match after de-duplication (below) |
| CRAP, same profile | crapper's Go cases | 4 of 4 match |
| Dupes vs `dryer --threshold 0.8 --min-lines 4 --min-nodes 20` | 5 packages | 281 of 281 pairs, identical scores (largest difference 0.0) |

## Method

**Complexity.** Each package was run in path mode with `crapper --no-coverage <dir>` and `godrycrap complexity -format tsv <dir>`. Functions are keyed by the multiset of `namespace.name`, plus CC. There were no differences. crapper's own Go cases (`tests/test_go.py`) are ported as fixtures in `testdata/crapper`; their tests (`TestCrapperCountsTheSameDecisions`, `TestCrapperMethodsNamespacedByReceiverType`, `TestCrapperImportPathComesFromGoMod`) expect exactly crapper's names, namespaces and complexities.

**CRAP.** Both tools read the same profile: `crapper --use-existing-coverage` and `godrycrap crap -format tsv -coverprofile <profile> <dir>`. A function matches when CC is the same, coverage is within 0.05 percentage points and CRAP is within 0.05. crapper finds a profile's import-path keys by suffix matching from the module root, so it was run in a mirror directory holding `go.mod`, a copy of the package and `coverage.out`; run with `--root` pointing elsewhere, crapper scores every function at 0%, its documented behavior for a function the report does not mention. GoDryCrap needs no mirror: it builds each file's key from the `go.mod` import path, and a file whose key is not in the profile is exit 3, not 0%.

**Dupes.** Each package was run in path mode with the same threshold and filters for both tools. A pair is identified by file base names plus start and end lines. Scores are equal when they are within 0.0005.

### crapper's Go cases, on the same profile

crapper's Go cases from `tests/test_go.py` are kept as a module in `testdata/crapper/coverage`, along with the profile go1.27.3 wrote for it. Two changes make it compile: `vars.go` files declare the free variables the cases use, and `for _, item := range items {` became `for range items {`, which keeps the line layout and the complexity. Both tools read that one profile from the module root:

```sh
cd testdata/crapper/coverage
go test -count=1 -coverprofile=coverage.out ./...
crapper --use-existing-coverage
godrycrap crap -format tsv -coverprofile coverage.out .
```

| Function | Namespace | crapper CC / cov / CRAP | GoDryCrap CC / cov / CRAP |
| --- | --- | --- | --- |
| Branchy | github.com/acme/demo/sample | 5 / 80% / 5.2 | 5 / 80% / 5.2 |
| Place | github.com/acme/demo/internal/game.Board | 3 / 100% / 3.0 | 3 / 100% / 3 |
| Run | github.com/acme/demo/sample.Widget | 3 / 100% / 3.0 | 3 / 100% / 3 |
| Simple | github.com/acme/demo/sample | 1 / 0% / 2.0 | 1 / 0% / 2 |

**All 4 match.** `TestCrapperCRAPOnSameProfile` keeps GoDryCrap pinned to crapper's numbers.

### Difference: duplicate blocks in -coverpkg profiles

With Go 1.27, a `-coverpkg` profile lists each block once per test binary. For GoDryCrap that was 7–8 copies of every block. The tools merge the copies differently:

- **crapper** adds up the statements of every copy, so a fully covered function reads about 14.3% (1/7).
- **GoDryCrap** merges identical spans into one block per span, keeping the highest hit count. `go tool cover` also keeps one block per span but sums the counts (ORs them in set mode). Both agree on whether a block was hit, and coverage only asks that, so GoDryCrap's percentages are the ones `go tool cover -func` reports; only the merged counts differ.

Fed the merged profile (one line per block, highest hits), crapper agrees on all 158 functions. The difference is deliberate: like `go tool cover`, GoDryCrap counts each block once. A plain per-package profile without `-coverpkg` has no duplicate blocks, so it is not affected.

### Other deliberate CRAP differences

These are described in the README.

- **Profile mismatches are exit 3.** These are errors, never 0%: a file missing from the profile; a malformed record; a block that is not one cmd/cover (Go 1.27) would instrument in the current source (blocks of package-level function literals are valid); or a changed function whose profile blocks are not exactly its computed blocks. crapper checks none of these. An edit that keeps every block's span and statement count is not detected (see [STATUS.md](STATUS.md)).
- **Keys match by import path.** A file in a module matches only `import/path/file.go`. crapper suffix-matches every file.

## Dupes differences

- **Default threshold.** GoDryCrap defaults to 0.8. dryer defaults to 0.82. The runs above both used 0.8.
- **Path mode compares within a package.** GoDryCrap path mode compares functions within each package directory. dryer compares every form across all the files it is given. All 281 pairs above lie within a single directory, for both tools, so this does not affect the results above. On a tree where dryer reports pairs across packages, GoDryCrap does not report them.
- **Diff mode compares with the package.** A changed function is compared with every function in its package. dryer's `--changed` compares changed files only with each other.
- **Known structural approximations.** These did not show up in the sample:
  - A call through an indexed value, such as `handlers[i](x)`, is read as a generic instantiation. tree-sitter-go has the same ambiguity.
  - Type conversions such as `[]byte(s)` are normalized as calls.
