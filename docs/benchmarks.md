# Benchmarks

Measured in October 2026 on an 8-core Linux machine with 15 GiB RAM, using Go 1.27.3. The large-tree runs used a private Go monorepo of 39,770 functions; every scan of it ran under `timeout` and a 2 GiB virtual-memory cap (`ulimit -v 2097152`).

The target is under 2 s for a hook run.

## Hook runs on real staged diffs

Six recent commits from the monorepo were each replayed as a staged diff (a worktree at the commit's parent, `git read-tree <parent>`, then `git checkout <commit> -- <changed .go files>`), and `godrycrap complexity` and `godrycrap dupes` were run from the module root. The commits touched 2–29 files and 2–32 functions each. With blobs local, each commit was run twice:

| Command | Warm time, all six commits |
| --- | --- |
| `godrycrap complexity` (staged) | 39–94 ms |
| `godrycrap complexity -diff <parent>..<commit>` | 36–73 ms |
| `godrycrap dupes` (staged) | 307–1130 ms |

**Every warm hook run is under 2 s.** dupes is slower because it parses every file in each touched package. On a partial clone, the first staged run can be slower while git fetches missing blobs; that time is git's, not GoDryCrap's.

## Per-package and whole-module runs

| Run | GoDryCrap | Reference |
| --- | --- | --- |
| complexity, per package (five packages, 125–818 functions) | 8–28 ms | crapper `--no-coverage` 0.2–0.6 s |
| dupes, per package (same five packages) | 17–99 ms | dryer 159–1815 ms |
| complexity, whole monorepo (39,770 functions) | 1.48 s | crapper `--no-coverage` 32.3 s |
| dupes, one 696-file package (749 pairs) | 5.5 s | |
| dupes, whole monorepo (2,867 pairs) | 8.4 s, under the 2 GiB virtual cap | |

Path-mode dupes loads, compares and releases one package (directory) at a time and keeps only the reported pairs, and path-mode complexity and crap drop syntax trees once each file's functions are extracted. Memory is bounded by the largest package, not the tree.

## CRAP scoring per coverage profile

`godrycrap crap -coverprofile <profile> <package dir>` on two monorepo packages, with one per-package profile each (641 and 2,905 profile lines), as a CI shard would write, three runs each: 6–27 ms per profile, including the check of every profile block against the cmd/cover layout of the current source.

## GOROOT/src memory and time

To give numbers anyone can reproduce, GoDryCrap was also run over `$GOROOT/src` (Go 1.27.3; 8,088 `.go` files). Peak RSS is from `getrusage`.

| Run | Peak RSS | Time |
| --- | --- | --- |
| `godrycrap dupes -format tsv $GOROOT/src` | 79 MiB | 3.9 s |
| `godrycrap complexity -format tsv $GOROOT/src` | 89 MiB | 1.7 s |

Both commands also compute the cmd/cover block layout of each parsed file, so `crap` can check profiles exactly.
