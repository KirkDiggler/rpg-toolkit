# dnd5e encounter composition — Design (the WHAT), wave 1: free-roam

**Status:** PROPOSED
**Module:** `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter` (package `encounter`, own go.mod)
**Why:** `brainstorm.md`. **How:** `plan.md` (after this is approved).

## Scope

The free-roam exploration encounter — the dungeon's ambient scene.
Members join, move continuously, and perceive; player activity pumps
the clock; on each tick the world acts back (monster deciders on their
own intel) and the story accrues. Declared endings close the encounter
into an Outcome with per-member carry-forward. One aggregate
persistence pair at the rpg-api seam.

**Non-goals (wave 1):** combat and everything combat needs — budding a
combat encounter, range/awareness joins, `play/interrupt` windows,
initiative, the resolution axis (#916); perception *checks* gating
testimony (needs the `rulebooks/dnd5e` parent — wave 1 is sight-only
LoS, so the parent is deliberately NOT a dependency yet); real monster
behavior (`behavior/` integration — wave 1 ships a `Decider` seam and a
fixture wanderer); multi-encounter coordination and entity routing
(campaign layer); timers/turn structure in free-roam (none exists);
split-party scenes; generic (non-dnd5e) composition.

## Composition laws (C1–C8)

This is NOT a play/ leaf — it composes them. Its own laws:

- **C1 (public pieces only)** — depends only on published versions of
  `core`, `play/clock`, `play/intel`, `play/record`, `tools/spatial`
  (v0.8.0+ managed seam), `world` (journal/graph for concealed-structure
  knowledge, since the concealment wave), `dice` (PARSE-ONLY: notation
  validation for persisted roll traces — no Roller, no randomness, so C8
  still holds), stdlib. NO `events` import for its own logic
  (see C4), NO `rulebooks/dnd5e` parent (arrives with perception
  checks), NO `play/interrupt` (arrives with the combat wave), NO old
  `encounter`.
- **C2 (courier, not oracle)** — every world-coupling flows through the
  composition explicitly: it surveils percepts *into* intel, it hands
  deciders *only* `HeldBy(themselves)` views, it appends record beats
  from returned deltas. No piece ever sees the world directly.
- **C3 (one aggregate at the seam)** — `ToData() EncounterData` embeds
  the leaves' Data types verbatim as fields; `LoadEncounter` calls the
  leaf loaders inside itself. rpg-api stores ONE blob and never touches
  a leaf loader (the persistence-at-the-seam commitment, 2026-08-10).
- **C4 (bus is quarantined plumbing)** — the composition's results are
  returned values (family law carries). It MAY hold an internal
  `events.EventBus` solely because `spatial`'s room loading requires
  one via `game.Context`; nothing subscribes to it and no behavior may
  depend on it. (Recorded wart: spatial should grow bus-free loading;
  candidate follow-up issue.)
- **C5 (no background time)** — no goroutines, no timers. The clock
  advances only inside `Pump`.
- **C6 (family signatures)** — R3 carries: single `*XxxInput`,
  `*XxxOutput`, error-last, `ErrNilInput` guarded first; persistence
  pair by value.
- **C7 (not concurrency-safe)** — the host serializes per encounter
  (load-verbs-save).
- **C8 (no randomness in the composition)** — deciders may be
  stochastic; the composition itself is deterministic. Verbs are atomic
  (R5 carries): on error, no state changed.

## Types

- `Encounter` — the aggregate: member set, field, `clock` (Tick),
  `intel`, `record` log, declared endings, status. Zero value unusable;
  `NewEncounter(*SetupInput)`.
- `MemberID = core.EntityID`. Members are players AND monsters; the
  only difference is who answers for them (a client or a `Decider`).
- `Member` — `{ID, Kind (player|monster), Room spatial RoomID, ...}`
  read-side value.
- `Field` — rooms + connections. Composition-owned topology encoding
  (spatial v0.8 has `RoomData` per room but no orchestrator/topology
  persistence — the composition owns `ConnectionData{ID, From, To,
  Cost}` and rebuilds the orchestrator + connections on load).
- `Ending` — declared at Setup: `{Key string, Trigger}` where wave-1
  triggers are `ReachedPosition{Member?, Room, Position}` (evaluated by
  the composition after placements change — "the stairs") and
  `External` (the campaign fires it — "withdrew"). Empty key or no
  endings at Setup → error (an encounter that cannot end is the
  liveness hole again).
- `Outcome` — `{Ending Key, At (clock reading), Members []MemberOutcome
  {ID, Room, Position}}`. Small on purpose: the closed encounter
  remains queryable and `ToData`-able as the archive; the Outcome is
  the campaign's summary, not a state copy.
- `Decider` — `interface { Decide(view []intel.Holding) (Intent,
  error) }` with wave-1 `Intent` = `MoveTo{Room, Position}` or `Hold`.
  Deciders are registered per monster member at Join/Setup. The
  anti-wall-hack contract is structural: `Decide` receives ONLY the
  monster's own holdings.

## Verbs

All mutating verbs return Outputs carrying the deltas the host projects
(spatial deltas, per-observer intel deltas, appended record Seqs) —
nothing the caller needs rides anywhere else.

| Verb | Semantics |
|------|-----------|
| `NewEncounter(SetupInput{Field, Members (with placements + deciders for monsters), Endings, Audience rules for record?})` | Builds the aggregate: constructs rooms/orchestrator via the managed seam, places members, runs the initial surveil cycle (first percepts land), appends the opening beat. Validates: ≥1 ending, non-empty member IDs, placements valid. |
| `Join(JoinInput{Member, Room, Position, Decider?})` | The ambient is always there to join. Places via managed seam, surveils the joiner's first percept AND refreshes observers who now see them, appends beat, returns deltas. |
| `Exit(ExitInput{Member})` | Member leaves with carry-forward: removed from field, their final `MemberOutcome` returned; their intel holdings REMAIN in the aggregate (the archive) but the returned carry includes their holdings for the campaign to seed sequels. Encounter auto-closes (`Outcome` with ending `"emptied"`? NO — see close semantics) when membership empties: wave-1 law — emptying WITHOUT a declared ending having fired closes with the reserved ending key `abandoned`. |
| `Move(MoveInput{Member, To Position})` | Continuous player movement (same-room, wave 1; door transitions are a plan task via the managed Transition + PlacementRequired flow). Spatial managed move → surveil cycle for affected observers → record beat → evaluate `ReachedPosition` endings → deltas out. If an ending fires, the Output carries the `Outcome` and the encounter is closed. |
| `Pump(PumpInput{})` | The activity tick: advances `clock.Tick`, then for each monster member (deterministic order): `Decide(HeldBy(monster))` → execute the intent through the same managed seam as players → surveil cycle → record beats. Then ending evaluation. Pacing/quantization of pumps is the HOST's affair (open question recorded in brainstorm §6); the composition just executes one tick per call. |
| `End(EndInput{Ending Key})` | The external trigger: validates the key was declared `External`, closes with the Outcome. |

**Close semantics:** closed = Outcome exists. Every mutating verb on a
closed encounter returns `ErrClosed`. Queries remain live forever (the
archive). Exactly one Outcome per encounter, ever.

## Queries

| Query | Returns |
|-------|---------|
| `View(ViewInput{Member})` | The member's world: their intel holdings (statuses derived — Current/ghost) + their own placement. This is the host's projection source for that player's client. Copy-out. |
| `Story(StoryInput{Audience, AfterSeq})` | `record.SliceFor` pass-through — the narrated log for that audience. |
| `Status()` | Open/closed + the Outcome when closed. |
| `Members()` | Current member list, stable order. |

## Errors

Family style: sentinels, `errors.Is`, verb-context wrapping.
`ErrNilInput`, `ErrNoMember`, `ErrNotMember`, `ErrNoEnding` (Setup with
zero endings / End with undeclared key), `ErrClosed`, `ErrNoField`,
`ErrBadPlacement` (wraps the spatial rejection), `ErrInvalidData`
(LoadEncounter). Exact list may grow in plan; every sentinel
`errors.Is`-tested.

## Persistence

`EncounterData` = `{OutcomeData? (present = closed), Clock
clock.TickData, Intel intel.Data, Log record.LogData, Field FieldData,
Members []MemberData, Endings []EndingData, EverMembers}` — leaves'
Data embedded VERBATIM (C3). *Amended during execution (Task 6):*
`FieldData` holds the composition's OWN room descriptions (id,
dimensions, occluder positions, boundaries, connections) rather than
`spatial.RoomData` — `LoadEncounter` rebuilds the field through the
SAME construction path Setup uses and re-places members at their
persisted positions. Why: spatial's room *loading* requires an event
bus via `game.Context` (the C4 wart — now eliminated entirely: no
`events` anywhere in this module), and `spatial.RoomData` rehydrates
entities as stand-ins that would break subsequent managed moves; the
composition constructs its field deterministically from inputs, so
persisting the inputs plus current placements is the honest minimal
state. `ToData() EncounterData` / `LoadEncounter(EncounterData)
(*Encounter, error)` by value. R8 carries: fresh-built-idle… note the
composition is never truly "idle" (Setup populates it), so the R8 zero
convention applies as: `EncounterData{}` is NOT loadable (`ErrInvalidData`
— an encounter without a field or endings is unreachable); golden-JSON
exact-string pins for a populated open shape and a closed shape;
**mutation-proof pins from the first persistence task** (family
standard). R9 carries: reject unreachable (closed without outcome,
outcome with undeclared ending key, members placed in rooms absent from
Field, connection referencing missing room, duplicate member IDs);
accept every verb-reachable state — leaf-level validation is DELEGATED
to the leaf loaders (LoadEncounter calls them and wraps their
rejections), never duplicated.

**Deciders are NOT persisted** — they are behavior, re-registered by
the campaign at load (`LoadEncounter` takes a `Deciders map[MemberID]
Decider` argument alongside the Data… by R3 this makes the signature
`LoadEncounter(data EncounterData, deciders map[MemberID]Decider)`;
the plan may fold deciders into a LoadInput — reconcile there, the law
is: behavior re-attaches at load, state never contains it).

## Acceptance criteria

- **AC1 (the tomb watch)** — one narrative scene test: Setup builds a
  crypt room with pillar boundaries; two players and one goblin (fixture
  wander decider); the goblin crosses behind a pillar → player A's
  holding fades to ghost at last-seen; A moves (`Move`) and `Pump`s —
  the goblin steps, A's percept refreshes; the goblin *sees A back*
  (its own holding — symmetric intel); A reaches the stairs position →
  `ReachedPosition` ending fires → Outcome carries both players'
  positions; the closed encounter still answers `View`/`Story`
  (archive); mid-scene `ToData`/`LoadEncounter` round-trip proves the
  suspended scene survives a process boundary and continues
  identically. Story beats asserted via `Story` at scene end.
- **AC2 (invariants)** — C8/R5 atomicity; `ErrClosed` on every mutating
  verb post-close; exactly-one-Outcome; decider isolation (a decider
  test double records what it was shown — exactly its own holdings,
  nothing else); copy-out on `View`.
- **AC3 (persistence)** — round-trips at open/closed/mid-fade states;
  golden pins; every rejection tested; mutation-proof evidence
  required in-task (aliasing, tag rename, stowaway field, leaf-Data
  substitution).
- **AC4 (compat gate)** — `rulebooks/dnd5e/encounter/**` path + a
  gorelease job in `compat.yml`.
- **AC5 (suite conventions)** — black-box `package encounter_test`,
  testify suites, plain function for AC1.
- **AC6 (error vocabulary)** — every sentinel `errors.Is`-tested.
