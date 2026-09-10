# Monster Ghost Pursuit — Implementation Record

**Implemented:** 2026-09-01

**Design:** [design.md](design.md)

**Plan:** [plan.md](plan.md)

**Decision:** [ADR-0047](../../adr/0047-encounter-owns-location-knowledge.md)

## Issue 201 mainline integration releases

These are the releases that completed Issue 201's mainline integration, not
claims about the latest module versions.

| Module | Toolkit PR | Merge SHA | Tag object | Tag / peeled SHA |
|---|---|---|---|---|
| encounter | [#1344](https://github.com/KirkDiggler/rpg-toolkit/pull/1344) | 0b04f73763cb3746c312d2f410029d2b916afa5c | 10e568b05abee5c88c1875742ad113576aa9eca9 | [v0.40.0](https://github.com/KirkDiggler/rpg-toolkit/tree/rulebooks/dnd5e/encounter/v0.40.0) / 0b04f73763cb3746c312d2f410029d2b916afa5c |
| behavior | [#1361](https://github.com/KirkDiggler/rpg-toolkit/pull/1361) | 5c840df5543fcf03d849ad5f0e264788e580e8bf | ff4c2d0b8745fe1a19d3308e043c7b191b6374e5 | [v0.3.0](https://github.com/KirkDiggler/rpg-toolkit/tree/rulebooks/dnd5e/behavior/v0.3.0) / 5c840df5543fcf03d849ad5f0e264788e580e8bf |
| session | [#1363](https://github.com/KirkDiggler/rpg-toolkit/pull/1363) | 6e5bd90e1978dbc260ffcfb7f8245cae34071022 | dc38ccc66074e1d38ef4316bc3bb20ee08bdc1e9 | [v0.42.0](https://github.com/KirkDiggler/rpg-toolkit/tree/rulebooks/dnd5e/session/v0.42.0) / 6e5bd90e1978dbc260ffcfb7f8245cae34071022 |

Each annotated tag peels exactly to the listed merge. The green CI / auto-tag
runs are Encounter [33361822357](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/33361822357)
/ [33361822363](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/33361822363);
Behavior [33464548092](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/33464548092)
/ [33464548109](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/33464548109);
and Session [33483863848](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/33483863848)
/ [33483863783](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/33483863783).
Each merge/tag comparison has no go.mod or go.sum difference.

Session v0.42.0 pins the completed Issue 201 graph: Encounter v0.40.0,
Behavior v0.3.0, D&D 5e v0.123.0, and Resolution v0.25.0. D&D 5e and
Resolution are Session's retained external providers. Subsequent unrelated
mainline work minted newer Encounter and Session tags (including v0.42.1);
that does not invalidate this integration or Session v0.42.0's pinned,
verified graph.

## Historical side publications

These side tags preserve provenance; none is a final/current Issue 201 release.

| Module | Tag | Tag object | Peeled commit |
|---|---|---|---|
| encounter | v0.38.0 | 9e5f282182bb3e4936d3d3d60269f836663bd003 | 4d0f1eff0ca1d2b686b122604958dfec8ebabd7b |
| encounter | v0.38.1 | bfc049a8f333a2bb8a24db6a39b63f02e12ccfad | d556014ee9ad2c951e768ef853756ec06fe495d4 |
| encounter | v0.39.1 | ea816ad346a92093c503b7d353c17e15b8980c9f | 5bc5799fd92ca612cb5d7ef0f64657e1b208ae68 |
| behavior | v0.2.0 | 634e6c5df7c5f82f2be56af0459cdf3abdba3211 | d8b89c8811959cfd8027adb0c43ee9fccc73b880 |
| behavior | v0.2.2 | 76cf55c33e3e16607e8d8be40a78fc539006086e | 0b491311cc6a162db1a1555e274ec6af079669e6 |
| session | v0.36.0 | f71c91110a7a2e410a8a8839ee4f5a4fb1eba79c | a5fbb260ec042e0c1c6a3ebf9e5f054d730658a7 |

Encounter v0.38.1 and Session v0.36.0 contain the correct runtime behavior
but predate the final public API documentation. This is timing history, not
alternative release authority.

## What shipped

- Encounter owns strict Known(position) | Unknown testimony. It writes tagged
  canonical payloads, reads legacy coordinates as known, and rejects malformed
  or current-unknown Sight holdings on load.
- Intel stays opaque. Current known sight is Seen; held known sight is
  Remembered; held unknown persists but is not actionable.
- Remembered entries are stale knowledge only: no concealed standing/reach data,
  no attackability, and an exact remembered-cell path.
- behavior.Basic selects the nearest visible standing target first. If none is
  visible it selects the nearest reachable memory, with a temporary subject-ID
  tie-break, and moves one cell. Lawful refreshed sight interrupts remembered
  pursuit on the next call.
- Driven exact-cell arrival uses the mover's held Intel, arrival cell, and
  complete percept to correct absent knowledge to held unknown without reading
  a concealed live position. Step and free-roam Pump do not do this.
- Encounter IntelDelta.Corrected makes corrections observable through direct
  and enclosing drive paths; Session mirrors state, paths, and correction IDs
  while preserving load-act-save atomicity.

## Implementation provenance

| Task | Commit | Result |
|---|---|---|
| strict location testimony | 7eed97a9ef65486c1507545f35be98c75d7353c5 | typed known/unknown |
| strict location review | a11c4a876dd2ed98353f15d5f1c6de1abb53103c | strict payload validation |
| encounter-owned deltas | 0a79e6a9b772a6c3ff9b20f7d2dcb16cfc6a9fad | value-projected deltas |
| remembered views | 67050d5aa892d14cef26050fa4a57d5702640827 | held known exact-cell targets |
| remembered view review | edf03f94c4a68501ab072e3b327222e946d62cf5 | unknown, stale, origin, ordering, and unreachable cases |
| driven arrival correction | 4d0f1eff0ca1d2b686b122604958dfec8ebabd7b | correct ghosts on arrival |
| behavior fallback | f73602773157c35e584fedb28a69b2ac15d051db | visible-first remembered fallback |
| propagation review | d556014ee9ad2c951e768ef853756ec06fe495d4 | nested correction propagation |
| behavior review | d8b89c8811959cfd8027adb0c43ee9fccc73b880 | pursuit boundaries |
| session projection | 8085052c2498163e1cf9b20b971b114b2426874c | mirror knowledge/correction |
| session persistence | 5e9f6dcc8069a9f4fdd38bcfb90484f3e3c0c193 | rollback and success |
| held-state review | b958449cc2c0b0433789f684a06d3c21cdd8c3d6 | corrected testimony remains held |
| double-door proof | b4a99e6bd113a4f86f4ab4642fb3c22c58fdb11b | end-to-end proof |
| ordered proof review | a5fbb260ec042e0c1c6a3ebf9e5f054d730658a7 | decision/view ordering |
| ADR-0043 reconciliation | 776841b2befcbd119be5119ee89c8c6fbac44c20 | accepted status |
| Task 7 lint correction | c5c85808048cfcd5c41d6c6717dcea4bcf8a4e10 | wall-boundary selector |

## Provider compatibility and review decisions

The Session provider review corrected two fixtures without weakening encounter
validation: offers_test.go replaced a malformed nil Sight fixture with canonical
known testimony while retaining stale/self filtering; onemap_test.go updated its
legacy coordinate output expectation to canonical tagged known output while
retaining legacy-input compatibility.

Resolution's MemberID repair belongs to
[rpg-toolkit#1324](https://github.com/KirkDiggler/rpg-toolkit/pull/1324),
which adopted D&D v0.122.2 and minted Resolution v0.24.4.

PR #1363's review retained flat []IntelCorrection for this scoped release.
A complete IntelDelta → Discovery projection, EndTurnOutput.Discovered, and
converter field-audit coverage are explicitly deferred Session API debt, not
silently implied by the shipped flat projection.

## Verification

Integration verification passed go test -race ./... -count=1 and golangci-lint
run ./... in Encounter, Behavior, and Session; the focused session double-door
race proof; go mod tidy plus clean module diffs; verify.sh for Encounter and
Session; make test-all; and check-decisions.sh.

make lint-all had a dispositioned baseline failure before changed modules: nine
pre-existing goconst findings in dice tests. make pre-commit had its known core
coverage-parser defect after format, tidy, Core/Events lint, and race-test
portions passed. Neither baseline was changed or hidden by Issue 201.

## Project boundary

[rpg-project#305](https://github.com/KirkDiggler/rpg-project/issues/305) and
[rpg-project PR #306](https://github.com/KirkDiggler/rpg-project/pull/306) are
design input, not toolkit authority. Before rpg-project#201 can close, the
remaining evidence is one local-game observation: break sight through the
second door, see the monster pursue the last-seen carpet cell rather than
hidden truth, and confirm it does not repeat that stale move after correction.
