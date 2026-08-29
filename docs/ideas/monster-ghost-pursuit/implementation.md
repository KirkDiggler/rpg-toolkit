# Monster Ghost Pursuit — Implementation Record

**Implemented:** 2026-08-28

**Design:** [design.md](design.md)

**Plan:** [plan.md](plan.md)

**Decision:** [ADR-0046](../../adr/0046-encounter-owns-location-knowledge.md)

## Published modules

The final remote tags were resolved to their peeled commits on 2026-08-28:

| Module | Published tag | Commit |
|---|---|---|
| encounter | `rulebooks/dnd5e/encounter/v0.38.1` | `d556014ee9ad2c951e768ef853756ec06fe495d4` |
| behavior | `rulebooks/dnd5e/behavior/v0.2.0` | `d8b89c8811959cfd8027adb0c43ee9fccc73b880` |
| session | `rulebooks/dnd5e/session/v0.36.0` | `a5fbb260ec042e0c1c6a3ebf9e5f054d730658a7` |

Encounter `v0.38.1` is the final provider release for this slice. Behavior
`v0.2.0` consumes the remembered-view API already published in encounter
`v0.38.0`, while session `v0.36.0` pins the corrected encounter `v0.38.1` and
behavior `v0.2.0`. No committed local replacement or Go workspace is involved.

## Implementation commits

| Task | Commit | Result |
|---|---|---|
| strict location testimony | `7eed97a9ef65486c1507545f35be98c75d7353c5` | add typed known/unknown location knowledge |
| strict location review | `a11c4a876dd2ed98353f15d5f1c6de1abb53103c` | reject ambiguous field names and duplicate keys |
| encounter-owned deltas | `0a79e6a9b772a6c3ff9b20f7d2dcb16cfc6a9fad` | project and merge Intel changes by value |
| remembered views | `67050d5aa892d14cef26050fa4a57d5702640827` | expose held known exact-cell targets |
| remembered view review | `edf03f94c4a68501ab072e3b327222e946d62cf5` | cover unknown, stale, origin, ordering, and unreachable cases |
| driven arrival correction | `4d0f1eff0ca1d2b686b122604958dfec8ebabd7b` | correct held known ghosts on exact arrival |
| behavior fallback | `f73602773157c35e584fedb28a69b2ac15d051db` | prefer visible targets, then remembered movement |
| correction propagation review | `d556014ee9ad2c951e768ef853756ec06fe495d4` | surface every nested driven correction delta |
| behavior review | `d8b89c8811959cfd8027adb0c43ee9fccc73b880` | cover remembered pursuit boundaries |
| session projection | `8085052c2498163e1cf9b20b971b114b2426874c` | mirror knowledge and correction contracts |
| session persistence review | `5e9f6dcc8069a9f4fdd38bcfb90484f3e3c0c193` | prove correction save rollback and success |
| held-state review | `b958449cc2c0b0433789f684a06d3c21cdd8c3d6` | prove corrected testimony remains held |
| double-door proof | `b4a99e6bd113a4f86f4ab4642fb3c22c58fdb11b` | prove pursuit, correction, and interruption end to end |
| ordered proof review | `a5fbb260ec042e0c1c6a3ebf9e5f054d730658a7` | bind decisions to the immediately following view |
| ADR-0043 reconciliation | `776841b2befcbd119be5119ee89c8c6fbac44c20` | correct stale status metadata in a separate commit |
| Task 7 lint correction | `c5c85808048cfcd5c41d6c6717dcea4bcf8a4e10` | use the promoted wall boundary fields without changing the fixture |

## What shipped

- Encounter owns strict `Known(position) | Unknown` location testimony. It
  writes tagged canonical payloads, reads legacy untagged coordinates as known,
  and rejects malformed or current-unknown sight holdings during load.
- `play/intel` remains channel-oriented opaque storage. It has no location,
  geometry, or truth API.
- Fight-time views keep current known sight in `Seen` and held known sight in
  `Remembered`. Held unknown sight remains persisted but is not actionable.
- Remembered entries contain stale knowledge only. They have no concealed
  standing or reach facts, cannot be attacked, and use paths ending on the
  exact remembered cell.
- `behavior.Basic` chooses a visible standing player before remembered
  knowledge. With no such visible target, it moves one cell toward the closest
  reachable remembered player, breaking equal distances by subject ID.
- After a successful driven move, encounter uses only the mover's held Intel,
  arrival cell, and complete movement percept to correct absent exact-cell
  memories to unknown. Public `Step` and free-roam `Pump` do not independently
  correct arrival knowledge.
- Encounter-owned `IntelDelta.Corrected` makes correction observable through
  direct and enclosing drive paths. Session mirrors known/unknown location
  state, remembered view data, and correction identifiers without deciding
  behavior.
- Session preserves load-act-save atomicity: failed persistence discards the
  in-memory correction and publishes nothing; success persists `Held + Unknown`
  and returns the observer/subject correction pair.

## Acceptance mapping

| Acceptance area | Passing coverage |
|---|---|
| tagged known/unknown, legacy coordinates, origin, malformed and contradictory input | `TestLocationPayloadRoundTrip`, `TestDecodeLocationPayloadReadsLegacyKnownPosition`, `TestDecodeLocationPayloadRejectsMalformedOrContradictoryShapes`, `TestLoadRejectsMalformedOrUnknownSightLocation`, `TestLoadAcceptsHeldUnknownSightLocation` |
| current-known projects into `Seen`, never `Remembered`; held-known projects into `Remembered`; stale confidentiality, exact-cell routes, deterministic order, unreachable and unknown handling | `TestMonsterViewCarriesStaticFactsAndSeen` (including the explicit empty-`Remembered` assertion), `TestMonsterViewProjectsHeldKnownSightIntoRemembered`, `TestMonsterViewHeldUnknownProjectsIntoNeitherCollection`, `TestMonsterViewHeldKnownUsesStalePosition`, `TestMonsterViewRememberedSortsIDsIndependently`, `TestMonsterViewRememberedSupportsOriginCell`, `TestMonsterViewRememberedUnreachableHasEmptyPath` |
| visible-first selection, closest selection across multiple visible players, reachable remembered choice, ID tie-break, never attack memory, no knowledge pass | `TestAttacksTheClosestStandingPlayerWhenInReach`, `TestPicksTheClosestOfTwoEquallyReachableTargetsDeterministically`, and remembered cases under `TestBasicSuite`, including `TestPrefersSeenOverRemembered`, `TestChoosesClosestReachableRemembered`, `TestRememberedTieBreaksByIDRegardlessOfSliceOrder`, `TestNeverAttacksRememberedTarget`, and `TestNoKnowledgePasses` |
| exact-arrival correction, lawful percept exception, mover ownership, deterministic multi-subject correction, persistence | remembered-arrival cases under `TestMonsterTurnSuite`, `TestRememberedArrivalCorrectionPersistsUnknown`, and `TestDrivenArrivalCorrectionPropagationMatrix` |
| session twins, deep copies, explicit unknown, sorted correction IDs, load-act-save rollback/success | `TestMonsterViewAdaptersCarryRememberedPathsByValue`, `TestBehaviorRoundTripsRememberedRoute`, `TestHeldUnknownSightProjectsExplicitUnknownLocation`, `TestProjectIntelCorrectionsSortsObserverThenSubject`, `TestSessionMonsterArrivalSaveFailureRollsBackCorrection`, `TestSessionMonsterArrivalPersistsCorrection`, `TestMalformedSightTestimonyFailsSessionLoadBeforeProjection` |
| discriminating double-door pursuit, no concealed live cell, resolved ghost pass, visible interruption | `TestSessionDoubleDoorGhostPursuit`, `TestSessionDoubleDoorVisibleInterruptsGhostPursuit` |

## Provider compatibility corrections

The published encounter contract exposed three stale session fixtures. Their
resolutions preserved the original assertions instead of weakening provider
validation:

1. `attack_test.go` failed because a shared fixture persisted `intel.Sight`
   with `Payload:nil`; encounter v0.38.1 lawfully rejects malformed sight
   testimony. The shared fixture was updated to canonical known testimony
   without weakening the unreadable-participant assertion.
2. `offers_test.go` used the same malformed nil sight fixture for stale/self
   holdings; it now uses canonical known testimony while retaining stale/self
   filtering assertions.
3. `onemap_test.go` expected legacy untagged `{"x","y"}` output; the provider
   now emits canonical tagged `{"state":"known","x","y"}` while legacy input
   remains readable. The assertion retains coordinate and no-room checks.

## Observed deviations and review discoveries

- The initial encounter `rulebooks/dnd5e/encounter/v0.38.0` tag at
  `4d0f1eff0ca1d2b686b122604958dfec8ebabd7b` was superseded by encounter
  `v0.38.1`. Independent Sol review found that driven correction deltas could
  be silently lost through `noticeDown` and `Record` paths. The correction
  algorithm did not change; the review made every nested delta explicit and
  added a propagation matrix. `v0.38.1` is the final encounter dependency
  recorded for the completed slice.
- The plan's bare focused regular expressions did not select testify suite
  methods. Suite-qualified focused commands were used during TDD; the final
  fresh module and repository gates exercise the complete suites.
- The double-door draft's geometry allowed reacquisition of concealed Billy.
  The final scene closes every unintended seam edge, moves the concealed cell
  around the lawful sight corner, and binds correction/interruption assertions
  to adjacent recorded decisions rather than an arbitrary history match.
- Correction-only deltas may project an empty observer entry in session
  `Discovered`. This did not affect correction delivery and was intentionally
  deferred rather than expanding this slice.

## Source-of-truth review corrections

Independent documentation review kept ADR status metadata canonical as
`Accepted`; shipped implementation evidence remains in this record rather than
in the status field. ADR-0046 now records why Intel-owned location semantics,
nil/fade/delete as an unknown-location representation, and behavior- or
session-owned decoding/correction were rejected in favor of encounter
ownership.

The exported encounter location godoc now distinguishes
`Known(position) | Unknown` content from Intel `Current | Held` currency while
retaining the existing rejection of `Current + Unknown`. The acceptance table
also names the proofs that current-known testimony projects only into `Seen`
and that behavior selects the closest target when multiple visible players are
available. These review corrections change documentation and godoc only.

## Final verification

Fresh Task 8 verification produced the following evidence:

| Scope | Command | Result |
|---|---|---|
| encounter | `go test -race ./... -count=1` | PASS: encounter, workbench, and dungeonspec |
| encounter | `golangci-lint run ./...` | PASS: `0 issues` |
| encounter | `go mod tidy`; `git diff --exit-code -- go.mod go.sum` | PASS: no module-file change |
| behavior | `go test -race ./... -count=1` | PASS |
| behavior | `golangci-lint run ./...` | PASS: `0 issues` |
| behavior | `go mod tidy`; `git diff --exit-code -- go.mod go.sum` | PASS: no module-file change |
| session | `go test -race ./... -count=1` | PASS: session and workbench |
| session | focused double-door `go test -race` | PASS: pursuit and visible interruption |
| session | `golangci-lint run ./...` | PASS: `0 issues` after the separately committed Task 7 selector correction |
| session | `go mod tidy`; `git diff --exit-code -- go.mod go.sum` | PASS: no module-file change |
| root | `git diff --check` | PASS |
| root | `./scripts/verify.sh rulebooks/dnd5e/encounter` | PASS: build, 811 tests, vet, gofmt, lint, transcripts |
| root | `./scripts/verify.sh rulebooks/dnd5e/session` | PASS: build, 492 tests, vet, gofmt, lint, transcripts |
| root | `make test-all` | PASS: every tracked Go module |
| root | `make lint-all` | BASELINE FAILURE before changed modules: nine pre-existing `goconst` findings in `dice` tests |
| root | `make pre-commit` | BASELINE FAILURE: documented core coverage-parser defect after format, tidy, Core/Events lint, and race tests pass |
| root | `./scripts/check-decisions.sh` | PASS: all 48 ADRs summarized |

The documentation review round freshly reran the encounter race suite, lint,
tidy/module diff, and `verify.sh` rows above; all passed. It also reran the ADR
index, link-resolution, formatting, diff, and exact-scope audits rather than
reusing their earlier results.

The first session lint run found:

```text
session/monster_turn_test.go:933:11: QF1008: could remove embedded field
"Boundary" from selector (staticcheck)
    if wall.Boundary.From == from && wall.Boundary.To == to {
```

`git blame` attributed the line to Task 7 commit `b4a99e6`. Billy approved the
narrow scope expansion, and `c5c8580` applied Staticcheck's promoted-field form
`if wall.From == from && wall.To == to {` in its own commit. Both focused
double-door race tests, the full session race suite, session lint, tidy/module
diff, gofmt, and the session verification script then passed.

`make lint-all` stops in the first module, `dice`, on nine repeated-string
findings across `lazy_test.go`, `notation_test.go`, and `pool_test.go`; those
files also have no feature or Task 8 diff. `make pre-commit` reproduces the
known `docs/how-to/run-tests.md` defect: package output for `core/mock` reaches
the numeric `bc` input, producing syntax errors and a misleading `76.5%`
coverage failure even though Core reports `88.9%` and its race tests pass.

Initial uses of the empty Task 8 module cache also failed under sandboxed DNS
while resolving pinned dependencies. Identical commands passed after approved
dependency access; no source or module file changed in response.

## Final scope audit

- `play/intel` has no feature diff and remains payload-opaque.
- `correctArrivedLocations` has one production caller, the driven `Move` path
  in `executeTurnIntent`; public `Step` and free-roam `Pump` do not call it.
- No `replace`, `go.work`, or `go.work.sum` exists in the tracked worktree.
- The feature adds no sticky target commitment, route utility, hiding,
  invisibility, sound, or deception behavior.
- New public location, delta, remembered-view, and session projection contracts
  have godoc; the three Task 8 Go diffs are comments/formatting only.
- Every acceptance criterion maps to tests exercised by the passing module race
  suites and repository-wide test sweep.
- Every new repository-relative Markdown link resolves. The new files introduce
  no external URL.
- The Task 8 source set contains only the approved ADR, decision index,
  encounter/session godoc, design status, and implementation record files.
  ADR-0043 status metadata was committed separately as `776841b`; the approved
  Task 7 lint correction was committed separately as `c5c8580`. No README,
  progress ledger, runtime file, module file, tag, push, or publication changed.
