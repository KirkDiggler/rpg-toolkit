# mind/perception implementation

[PR #1686](https://github.com/KirkDiggler/rpg-toolkit/pull/1686) implements
[issue #1685](https://github.com/KirkDiggler/rpg-toolkit/issues/1685) as a new
module, `mind/perception` v0.1.0, on branch `feat/1685-mind-perception`. As of
this record the PR is open — reviewed and approved with findings, not yet
merged.

No `plan.md` was written for this idea. The issue itself was already an
executable ruling (API, nine numbered rules, thirteen test cases, a
done-when), so there was no separate planning phase between design approval
and code: implementation began directly against the issue and this
`design.md`/`implementation.md` pair was added retroactively, during the
PR's independent review, once the review flagged that the rule numbers cited
in code comments resolved only inside a GitHub issue rather than a durable
record. A `plan.md` written after the fact, for work already complete in one
sitting, would only restate what these two files already say — so the pair
stands alone here rather than the family's usual trio.

## What shipped

Three commits landed the module and its first two fixes:

- `46ac5df2` — the module itself: `doc.go`, `perception.go`, `data.go`,
  `errors.go`, `perception_test.go`, wrapping `play/intel v0.3.0`. Thirteen
  test cases, one per numbered case in the issue.
- `a0e0c895` — `On`'s not-held answer translated to a package-owned
  `ErrNotHeld` instead of passed through as a bare wrap of
  `intel.ErrNotHeld`, so a caller never has to import `play/intel` to ask
  the single most common question this package answers.
- `c872e39b` — a `prealloc` fix for CI's pinned `golangci-lint` v2.3.1
  (older than the locally-installed v2.13.2, which did not flag the same
  loop). No behavior change.

Two mutation checks, applied to the working tree and reverted rather than
committed, confirmed the done-when's load-bearing claims at each stage:
forcing every `Refreshed` subject into `Changed` broke case 2
(`TestRefreshedWithoutChangeIsNotChanged`), and skipping observers whose
built percept is empty broke case 7
(`TestObserverReachingNothingFadesEverything`).

## Independent review, and what it changed

An independent review of PR #1686 verified the gates and both mutation
checks directly, then approved with two Important findings and several
minor ones. Full text is on the PR. What changed here as a result:

- **R7 stopped deriving `Changed` and started consuming it.** The issue's
  original rule 7 compared a holding's `Observed` to `pass.At` after
  landing — correct only while `At` strictly increases. Reusing `At` (or
  leaving it at the zero value) made every refresh read as a change. The
  fix was taken at the lower layer instead of documented around: `play/intel
  v0.4.0` ([#1688](https://github.com/KirkDiggler/rpg-toolkit/pull/1688))
  added `Changed []Subject` to `SurveilOutput`, reporting the
  `bytes.Equal` comparison the store already made and previously discarded.
  This module bumped to that tag and `deltaFrom` now maps
  `out.Changed` directly — the same shape as `Faded` and `Reacquired`,
  no reconstruction, no `At` dependency, and no error path through
  `deltaFrom` any more (it dropped from `(*Delta, error)` to a plain
  `*Delta`). `TestSameAtNeverForcesChanged` pins the pathological case the
  review reproduced: three passes at the same `At` with an identical
  payload, `Changed` never fires.
- **Duplicate `Presence`/`Observer` IDs are now rejected, not merely
  possible.** `slices.SortFunc` is not stable, so which of two same-ID
  presences survived intel's last-wins dedupe depended on input order — a
  crack in rule 6's determinism promise for a caller bug. `validatePass`
  gained two more checks (`ErrDuplicateSubject`, `ErrDuplicateObserver`),
  run only after every empty-ID check has cleared the whole `Pass`, so a
  duplicate never outranks an empty ID regardless of position.
- **Documentation now matches what the code actually guarantees.** `Faded`
  now says "no longer current via any channel" instead of "stopped being
  delivered this pass" — identical today with one channel, but the former
  is intel's real semantics and the latter would mislead the day a second
  channel arrives. `Observe`'s "nothing is written" claim is now scoped to
  validation errors rather than stated as an unconditional guarantee.
  `doc.go` no longer calls this a "leaf module" (`play/README.md`'s leaf
  promise is "depends only on core"; this depends on `play/intel` too) and
  now states the persistence exception to "callers never see intel"
  explicitly, rather than leaving `Load`'s wrapped `intel.ErrInvalidData`
  and `Data.Intel`'s exposed `intel.Data` looking like a silent
  contradiction. `Data.Intel`'s `omitempty` tag was dropped — a no-op on a
  struct field.
- **Two gaps in test coverage closed.** `Load`'s rejection path
  (`TestLoadRejectsInvalidData`) and `Held`/`On`'s own empty-ID validation
  (`TestHeldAndOnValidateEmptyIDs`) are now exercised directly.

The review's suggestion of an `OnInput` struct for parity with intel's I/O
convention was considered and declined: intel's `*Input` types exist
because its verbs take variable-length report lists, and `On`/`Held` are
fixed-arity reads with no plausible parameter growth. Recorded in
`design.md`'s Queries section as a decision, not an oversight.

The review's adoption note — that encounter's current unplaced-observer
`continue` must keep unplaced observers **out of** `Pass.Observers`, since
an observer *in* `Observers` with no reach now fades everything under R4 —
is carried forward for the encounter-adoption PR, not addressed here.

## Verification

- `go test ./... -race` passes from `mind/perception`: 17 `=== RUN` lines (1
  suite, 16 test methods, `-v` output grepped for the literal marker rather
  than trusted from exit code).
- `gofmt -l .`, `go vet ./...`, and `go mod tidy` (no diff) all clean.
- `golangci-lint run ./...` clean under both the locally installed v2.13.2
  and, since local and CI disagreed once already on `prealloc`, a
  side-installed copy of CI's exact pinned v2.3.1.
- Both done-when mutation checks re-run against the revised code (consuming
  `out.Refreshed` in place of `out.Changed`, and re-adding the empty-percept
  `continue`) and confirmed to break case 2 and case 7 respectively, then
  reverted.

Encounter adoption, and everything the "Do not add" section of #1685 named
(`Report`, a second channel, forgery, contacts, naming), remain outside this
module, as scoped from the start.
