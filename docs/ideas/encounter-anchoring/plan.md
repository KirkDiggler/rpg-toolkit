# Encounter world anchoring — plan (the HOW)

**Executes:** `design.md` (approved via this PR). **Issue:** #929.
**Module:** `rulebooks/dnd5e/encounter`, branch `feat/929-encounter-anchoring`,
tag `rulebooks/dnd5e/encounter/v0.3.0` on merge.

Family standards apply throughout, non-negotiable: mutation-proof pins
from the first task (build the mutant, watch the targeted test fail,
revert — compiling mutants only); one-defect fixtures with
discriminating message fragments; asymmetric fixtures (rooms differ in
dims AND origins AND endpoint cells, so transposition/cross-wiring
mutants die); black-box `package encounter_test`; scene tests tell one
continuous story with exact transcript pins. The v0.2 archived pins
(`tombwatch_test.go`, `example_tombwatch_test.go`) stay untouched;
the vault chase gets extended, not rewritten.

## Task 1 — anchors and the W-laws at Setup

`RoomInput.Origin`; W1 (one geometry per field), W2 (no overlap,
touching legal), W3 (doorways kiss, per-family adjacency), integral
origins for hex. All at `NewEncounter`/Setup validation, ordered per
design. New fixtures: the anchored two-room hex field (asymmetric:
r1 10x4 at origin (0,0), r2 3x9 at a **negative-axial** origin placing
its endpoint adjacent to r1's — the corrected-golden lesson from v0.2
carried forward). One-defect table rows per rejection class; sibling
touching-is-legal fixture proving W2 rejects overlap, not contact.

Mutants that must die here: origin-ignored-in-projection (W2/W3 checks
that never add the origin pass symmetric fixtures — asymmetry kills);
adjacency-formula (cube distance computed as 2D Manhattan); overlap
check comparing local instead of absolute cells; W1 check comparing
only adjacent-in-slice room pairs.

## Task 2 — persistence and Load

`RoomData.Origin *PositionData`, presence-required (W5); ToData always
writes (declared zero persists explicitly); Load re-runs every W-law
one-defect row plus nil-origin-per-room. Rich golden grows origins on
every room including the negative-axial hex room; exact-string pin
updated. Round-trip: reload's Atlas deep-equals pre-reload's (pin lands
fully in Task 3; the round-trip of origin data pins here).

Mutants: omitempty-on-origin (zero origin vanishing from the blob then
failing presence at load — the golden catches the tag, the round-trip
catches the behavior); load skipping W3 (a hand-built hostile blob with
non-kissing doorway must reject — hostile-blob probe per the v0.2
standard).

## Task 3 — Atlas and the bridge

`Atlas()` (zero-arg read, deterministic ordering per C8, copy-out
immune) and `Absolute(*AbsoluteInput)` (nil-input, unknown-room,
out-of-bounds rejections; legal for any in-bounds position). Gridless
rooms: nil `Cells`, bounds+origin carried. Copy-out immunity pins:
mutate returned atlas slices, internal state unchanged.

Mutants: doorway ordering (sorted by connection ID — swap to insertion
order); bridge subtracting instead of adding the origin (dies on any
asymmetric fixture); Atlas computed from live member state instead of
construction data (place a member, Atlas unchanged — pin as a test).

## Task 4 — the absolute-continuity scene

Extend the vault chase: project every position along the pursuit
(moves, traversal, arrival) through `Absolute`; assert the world-space
path is continuous — consecutive positions at most one step apart in
the field's geometry, **including the doorway step**. Exact projected
transcript pinned. This is the wave's payoff pin: W2+W3+bridge proven
as one property, the exact property the web client renders by.

Mutant: break W3 in the fixture (shift one origin by one cell) — the
continuity assertion must fail at exactly the doorway step, proving the
scene discriminates the law it guards (then revert; Setup validation
also rejects the shifted fixture, so this mutant runs against a
validation-bypassed hand-built encounter — the hostile-probe pattern).

## Task 5 — workbench, Example, docs

Workbench prints the atlas (rooms placed in world space, kissing pair
visible) and gains an `atlas` command; a new sibling Example pins a
projected transcript; `./scripts/verify.sh rulebooks/dnd5e/encounter`
prints it. Module doc comment gains the W-laws. `gorelease` run and
recorded; tag on merge.

## Post-merge (not this PR)

- Comment on rpg-api#793: wiring targets v0.3 for the map surface;
  `internal/components/dungeon` deletes when wiring lands (its
  graduation, completed).
- Memory/handoff updates per session convention.

## Execution addenda

(Reviewer findings and mid-build corrections land here, per family
practice.)
