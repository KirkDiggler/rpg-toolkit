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

`RoomInput.Origin`; gridless leaves the composition (a gridless
declaration is a room-list defect — squares stay); W1 (one geometry
per field), W2 (no overlap, touching legal), W3 (doorways kiss,
per-family adjacency), integral origins for hex. All at
`NewEncounter`/Setup validation, ordered per design. New fixtures: the anchored two-room hex field (asymmetric:
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
one-defect row plus nil-origin-per-room plus stored-`"gridless"`
rejection. Rich golden grows origins on
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
immune), `Absolute(*AbsoluteInput)` (nil-input, unknown-room,
out-of-bounds rejections; legal for any in-bounds position), and
`Locate(*LocateInput)` — the reverse bridge (absolute → room + local;
void positions between rooms reject; round-trip pin
`Locate(Absolute(r,p)) == (r,p)` over every cell of the asymmetric
fixture). Copy-out immunity pins: mutate returned atlas slices,
internal state unchanged.

Mutants: doorway ordering (sorted by connection ID — swap to insertion
order); bridge subtracting instead of adding the origin (dies on any
asymmetric fixture); Locate answering the wrong owner for a kissing
pair (each doorway cell locates to its own room — pin); Atlas computed
from live member state instead of construction data (place a member,
Atlas unchanged — pin as a test).

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

Reviewer findings and mid-build corrections, per family practice.

### T1 — anchors and the W-laws (commits 9244b3c, ab9e3f6, 8ca5efa, 60e52ac)

Two review rounds (sonnet APPROVE; Opus FIX_REQUIRED → APPROVE), 288 →
309 tests, 13 mutants killed.

- **Mixed-family fixtures collided with W1.** Three v0.2 tests declared
  square+hex rooms in one encounter — the exact thing W1 now forbids.
  Resolutions: the negative-axial connection test became all-hex; the
  grid-shape reload test split into one encounter per family (the
  "both shapes in the SAME encounter" property is what the law
  outlaws, so its heir is two per-family round-trips plus the W1
  rejection pin); `TestGoldenJSONRich` was rewritten all-hex
  deliberately — hex is the family the game ships — after confirming
  the square/grid-absent byte pin still lives in the Open/Closed
  goldens.
- **Blocker (Opus): a negative room dimension panicked `NewEncounter`**
  through `makeslice` — a panic is not a rejection (R5). Closed by
  rejecting non-positive dimensions in room legality.
- **Soundness (Opus): fractional square origins defeated W2.** Two 5x5
  rooms at (0,0) and (0.5,0.5) interpenetrate ~81% while their integer
  cell sets stay disjoint; a distance-0 "doorway" was reachable too.
  **Origin integrality is therefore required for ALL families**, not
  just hex (design amended). W2's disjointness is only sound over
  integral anchors.
- **Three fixture-blindness gaps**, each closed with a witness verified
  to kill its mutant: a Chebyshev-on-axial substitution passed every
  hex fixture (none used the axial (1,1) case where the formulas
  diverge — cube distance 2, not 1); W2 weakened to adjacent-in-slice
  pairs survived (the only 3-room fixture was a positive control); the
  hex **R**-span boundary was unobservable because every hex fixture
  separated on Q alone.
- **W2 rewritten to interval intersection.** Rooms are boxes, so two
  overlap iff both axis intervals intersect — exact, O(1) per pair.
  Measured 560ms/144MB → 318µs/12KB on a 1000x1000 pair (spatial's own
  docs recommend such spans). Cell enumeration was deleted here and
  re-derived in T3 for Atlas.
- **Representable origins.** `Trunc(x)==x` passes for ±Inf and every
  magnitude past 2^63, so absurd origins collapsed to int64-min bounds
  and produced a **wrong verdict** through the public API (rooms
  1e19 apart rejected as overlapping; +Inf accepted as an anchor).
  Closed with an exact int round-trip guard.
- **The strict kiss comparison is pinned, not asserted.** A comment had
  claimed `dist != 1` was unfalsifiable; Opus constructed the
  falsifier (square endpoints stay fractional-tolerant, so a 0.5
  distance is reachable), so the pin exists and the rationale was
  corrected.
- **Accepted as provably unobservable:** W1 compared adjacent-in-slice
  pairs only is indistinguishable from full pairwise once gridless
  leaves and equality ranges over two values (transitivity). Full
  pairwise kept as the correct form, with the reasoning in-code.

### T2 — persistence and the unified Load seam (commits 327f5dc, d59e386)

Two review rounds (both APPROVE), 309 → 327 tests.

- **Setup and Load now share one validation implementation.** Load
  converts wire shapes to typed inputs (resolving only wire-only
  concerns: grid string, origin presence, endpoint presence) and then
  calls the same `buildValidRoomGrids`/`validateConnectionInputs`
  Setup calls; ~150 lines of parallel validation deleted. Proof it is
  structural rather than conventional: disabling the shared validator
  once kills pins on **both** seams.
- **Gridless fully excised** from the runtime paths — the load path
  rejects the stored string as an unrecognized shape, and the
  `GridShapeGridless` branches in `buildRoomGrid`/`gridShapeToData`
  are gone (the latter was a write-only wire value the module could
  emit but not load).
- **F1, the sibling of T1's origin overflow (Opus):** `Width`/`Height`
  were unbounded, so `MaxInt64`-wide rooms wrapped the interval sums
  and two rooms overlapping over ~9.2e18 cells were accepted as
  disjoint — through both public seams. Closed in-wave (not deferred)
  with `maxRoomSpan`/`maxAnchorCoord` = 2^30, documented as overflow
  defense rather than a gameplay limit.
- **Load errors spoke in Setup's voice** ("newencounter:" prefixes
  reaching persisted-blob rejections). The verb prefix moved out of
  the shared validators to each caller's wrap.
- **Presence errors could name an unvalidated ID** (`room "" missing
  origin`). A wire-shape ID pre-pass now runs before conversion, so
  the ID defect is reported first.
- **Golden law upheld:** `outcome.at` (omitempty) was exercised by no
  golden — the closed fixture now pumps a tick before closing. The
  rich golden's anchors were re-translated so both axes are nonzero
  and distinct per room, which makes the golden itself catch an X/Y
  transposition (previously only the round-trip test could).
- **Verified clean by the Opus pass:** 36 adversarial raw-JSON blobs at
  the trust boundary (no panics, no wrong verdicts); byte-identical
  round-trips **after a real story** (joins, moves, a traverse, three
  pumps with deciders acting) with behavior identity on reload; 2100
  golden/marshal runs with zero flakes; partial-object presence
  semantics consistent with the v0.1/v0.2 precedents.

### T3 — Atlas and the bridges (commit 7104dd2)

327 → 547 tests. Cell enumeration re-derived (not resurrected) and
proven against spatial's own `IsValidPosition` by a 200-case property
test across both families and dimension parities.

- **Rulings settled during dispatch:** occluded cells are **owned**
  (occlusion is walkability, not ownership — both bridges accept
  them); fractional square positions are legal and round-trip
  (SquareGrid is [0,width) continuous, and W2 with integral origins
  keeps ownership unambiguous); Atlas is construction-truth and must
  not move when members do; `Locate` is well-defined only because W2
  gives every absolute cell at most one owner.
- **Atlas's O(total cells) cost** is documented as a caller contract.
  No guard this wave: the 2^30 span bound is overflow defense, and a
  field large enough to matter could not be carried on the wire.
- **Doorway ordering is structurally unobservable** — `connectionsInput`
  arrives pre-sorted from both `NewEncounter` and `LoadEncounter`, so
  removing Atlas's own sort fails nothing. The sort is kept for
  self-contained correctness (room ordering, by contrast, IS
  observable — `fieldInput` is not pre-sorted).
