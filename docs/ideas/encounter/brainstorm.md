# dnd5e encounter composition — Brainstorm (the WHY)

*2026-08-10/11. The play/ family is complete (clock, intel, record,
interrupt — all v0.1.0) and `tools/spatial` v0.8.0 shipped the managed
membership seam. This is the design dialogue for the thing they were all
built for: **composing a better encounter**. Kirk and the director
session; decisions recorded with reasoning. The normative WHAT is
`design.md`; the HOW is `plan.md`.*

## What this is

Journey 051's founding taxonomy: **an encounter is a composition with an
outcome** — `Setup → play → Outcome` — sitting between Tools and
Rulebooks, with campaigns composing encounters by consuming outcomes.
This triplet designs the first real composition: the **free-roam
exploration encounter**, the dungeon's ground state, where the party
moves, perceives, and the world moves back.

## Decisions and their reasoning

### 1. Born in the rulebook, not generic

Kirk's call: the composition lives in the rulebook's namespace — a
generic encounter framework may exist someday, "we do not need to lead
with that." The play/ leaves earned generic-hood by being truth-blind
bookkeeping; the composition is where *rules* live, and the only rules
we have are dnd5e's. A generic framework with one consumer would be an
application pretending to be a tool — the original sin from journey
051, inverted. Generic extraction is a shelf a second rulebook pays
for. Bonus: the parked `encounter/v2`-vs-takeover bridge question
dissolves — the new module is `rulebooks/dnd5e/encounter`, no name
collision; old `encounter` dies by delete-and-tag-pin when consumers
migrate.

**Module topology**: `rulebooks/dnd5e/encounter` is its **own Go
module** (own go.mod, own tags), not a package inside `rulebooks/dnd5e`.
Two reasons beyond parallelism (the platform team is concurrently
building the resolution axis inside `rulebooks/dnd5e` — toolkit #916 —
and the one-PR-per-module law forbids two teams in one module): the 051
build law — *the assembled default may use only public pieces* — is
**enforced by a module boundary** and merely promised by a package
boundary; and old-encounter deletion stays surgical.

### 2. Free-roam leads, combat follows

The first wave is the exploration encounter, not combat. It has an
outcome ("found the stairs," "withdrew," "combat erupted") — Kirk:
"it has an outcome and that's what we can see in action" — it composes
all four... all *three* shipped leaves it needs (clock, intel, record)
plus spatial, and it requires **zero suspendable resolutions**, so it
doesn't wait on the resolution axis (#916). It is also, at last, the
long-queued first consumer: monsters moving on `clock.Tick`.
`play/interrupt` deliberately does NOT appear in wave 1 — its first
consumer is the combat wave. Empty shelves.

### 3. Outcome = declared endings + per-member carry-forward

`Setup` declares the ways the encounter can end; play runs until one
fires; the Outcome is *which ending fired* plus a compact per-member
carry-forward. The rejected alternative — outcome derived by reading
the record — makes "ended" a matter of interpretation; declared endings
make it an event. The Outcome stays **small**: the closed encounter
itself remains readable (queries + `ToData`) as the archive; the
Outcome is the summary the campaign consumes, not a copy of the state.

**The quest layer sits above and is not ours.** Kirk's scenario — the
relic delve: "fought bravely but never located the relic… heads to
town, takes an urgent protection quest, comes back later" — proved the
boundary: encounter endings are scene-sized and local ("withdrew,"
"cleared"); *quest* progress is the campaign folding outcomes into a
longer arc. The relic can pull the party back five times; the encounter
machinery never knows a relic exists.

### 4. Two kinds of "leave" — pause is free, departure is an ending

- **Pause** (process sense): players close the Discord activity
  mid-scene and return tomorrow. The same encounter resumes — and this
  is free *by construction*: the host runs load-verbs-save, so the
  encounter is a persisted aggregate rehydrated on every RPC. Pause is
  invisible to the model; no status exists for it.
- **Departure** (narrative sense): the party walks out. An ending
  fires, the Outcome is produced, the encounter **closes**. Returning
  later is a **sequel**: a new encounter seeded from the carry-forward.
  The slogan: **you leave places, not encounters — encounters end,
  places persist.** Continuity is data, not a kept-alive process: the
  survivors' intel carries (they remember you), the place carries
  (campaign-owned), the story carries (record). The gap belongs to the
  campaign — "you were gone three days" means the campaign may advance
  the world before seeding the sequel, which a frozen suspended
  encounter could never express.
- **Closed** simply means *has an Outcome*. Terminal, always; no zombie
  third state. "You cannot go back" is campaign fiction (the tunnel
  collapsed), never an encounter property.

### 5. Chained encounters; the ambient free-roam is the ground state

Waking the goblins **ends nothing and changes no gear inside one
encounter** — Kirk chose chaining over mode-shift: "the former is more
composable." The free-roam encounter is the dungeon's **ambient scene**:
lazily created on first entry, long-lived, always there to join.
Combat encounters **bud off** from it and members **flow**: A and B
exit the ambient into the combat; C, two doors down, keeps free-roaming;
when the combat closes, its outcome flows survivors back into the
ambient. The ambient's own endings are delve-scale (party leaves the
dungeon, delve complete, TPK).

**Vocabulary law**: *members exit; encounters close.* Carry-forward is
per-member (intel is per-observer anyway — A's ghost-goblin memories
travel with A). An encounter closes when an ending fires or its
membership empties.

**Concurrency is core, not shelf.** "Someone else free roaming 2 doors
down is still free roaming" — multiplayer forbids freezing C because A
and B started a fight. Old encounter's god-aggregate did exactly that
freeze. The dungeon hosts N concurrent encounters; the campaign layer
routes entities between them.

**The join rule is symmetric**: "will join when they get within range" —
joining a combat is proximity (spatial) + awareness (C *heard* the
fight: a `Report` into C's intel; so did the goblin reinforcements in
the side room). Players, monsters, and reinforcement waves all join by
the same rule. Nobody is teleported into initiative by fiat. (Combat
wave's work — recorded here because it shaped the member model.)

### 6. Continuous players; activity-pumped time

Free-roam has no turns — the live game already lets players walk the 3D
dungeon freely. Kirk: "continuous for players. we just need them to
trigger a tick so the monsters and other things have space to move
around." So: **players move whenever; player activity pumps the clock;
the world thinks on the tick.** Each tick the composition runs its
courier cycle — surveil percepts into intel (fades fire, ghost-goblins
form), let monster deciders act on *their* intel (the anti-wall-hack
contract, realized), append record beats. No background ticker process
exists anywhere — honest, because the host is request-driven; and
DM-authentic — an idle table is a frozen world, which for a Discord
game is a feature. Open design-doc question, deliberately unanswered
here: **tick pacing under four concurrent movers** (the pump should
accumulate activity into tick-sized quanta rather than 4×-speeding the
monsters).

### 7. Wave-1 scope (the skeleton)

One ambient free-roam encounter; membership `Join`/`Exit` exist (the
ambient's "always there to join" requires them) but no mid-scene
combat budding; sight-only perception (LoS via spatial boundaries — no
perception checks yet, so no `rulebooks/dnd5e` parent dependency in
wave 1); a minimal `Decider` seam with a fixture wanderer standing in
for `behavior/`; endings = position-trigger (reached the stairs —
composition-evaluable via spatial) + external (campaign fires
"withdrew"). Shelves, labeled: combat budding + range/awareness joins
(combat wave, consumes #916 + interrupt); perception checks gating
testimony (needs the rulebook parent); multiple concurrent encounters
coordination (campaign layer's job, not this module's); split-party
scenes; town-as-encounter.

## Links

- `docs/journey/051-*` (the taxonomy this realizes), `052-*` (bus
  observational; enumeration)
- `docs/ideas/play/{clock,intel,record,interrupt}/` — the leaves'
  triplets; their laws inform but do not bind this module (it is a
  composition, not a leaf — its own laws are in `design.md`)
- toolkit #916 — the resolution axis (combat wave's provider)
- `tools/spatial` v0.8.0 managed seam (PR #913) — the courier's
  movement substrate
