# play/intel — Brainstorm (the WHY)

*2026-08-09/10. Axis two of the encounter reset (journey 051), chosen as
the deep-design lane while `play/record` runs as the compact mechanical
lane. Design dialogue between Kirk and the director session; decisions
recorded with their reasoning and rejected alternatives. The normative
WHAT is `design.md`; the HOW will be `plan.md`.*

## What this axis is

Journey 051 called it **Knowledge** — "what each observer knows; the one
part of the old module worth admiring." The dialogue renamed it twice and
the renames are the design: it stores **intel** — per-observer,
channel-sourced, possibly false, possibly stale holdings about the world.
The module never sees the world and cannot verify anything.

## Decisions and their reasoning

### 1. Bookkeeper, not map-reader

The fork: does the module compute visibility itself (importing
`tools/spatial` — the old `perception.VisibleHexesAt` shape), or does it
only record testimony the composition delivers? Kirk settled it with the
two-owners framing: **the stage owns the physics of perception** — LoS is
just the sight channel's physics, and hearing/smell propagation belong in
`tools/spatial` too when they arrive (per-channel wall properties
generalizing today's `BlocksLoS`). **Intel owns the memory of
perception.** The composition is the courier between them.

Why this shape won: the leaf law survives pure (core-only deps — no
spatial version coupling, no returning diamond risk); geometry stays
confined to the field instead of escaping into a second module; and the
module is closed over channels it has never heard of — when the stage
grows hearing physics, intel changes zero lines. The old module's own
history is the evidence: its perception code was best exactly where it
was pure (View/memory/first-sight) and hairiest exactly where geometry
coupled in. The courier burden this creates for the composition is not
new work — old `Move()` did the same orchestration, tangled inside the
aggregate. And it is the same move Clock made: the composition reports
world-coupled inputs (displacement there, percepts here); the leaf
bookkeeps and returns deltas.

(Aside from the naming journey: "stage," rejected as the toolkit's name
because a stage is a location, found its true home — it is exactly the
right metaphor for the field axis.)

### 2. Beliefs, not truth — deception is native

Kirk's reframe: "knowledge is what they believe... I could see a player
being charmed without knowing it." That sentence is the module's charter.
Because intel never sees the map, it *cannot verify* — so illusions,
disguises, lies, and charm-planted falsehoods need no special machinery:
they are ordinary testimony whose content happens to be false. The
map-reading shape could never do this (belief derived from geometry is
always mechanically true). Emergent correctness for free: when a charm
ends, the false holdings persist until fresh testimony contradicts them —
which is how minds actually work.

This settled the rich-vs-lean holding question: holdings carry the
observed **payload** (opaque snapshot), not just subject identity —
because the content of a belief is the point, and the ghost-goblin-at-
last-seen-position (the old module's most game-feel-critical behavior) is
just the ordinary case of belief diverging from world.

### 3. Perception rolls stay outside

Intel holds no dice (same law as clock). The rulebook rolls perception —
or reads a passive score — and the composition gates or degrades the
testimony by the result: failed check, nothing lands; partial success,
"something moved" instead of the full fact. Intel stores what got
through, indifferent to why. Channel "physics" thus live wherever they
belong: the stage for physical senses, the rulebook for supernatural ones
(charm, telepathy, divination). Intel treats every channel identically.

### 4. Subjects are opaque and caller-chosen — identity is testimony

Kirk's probes shaped this: "there could be more than one goblin" (keying
by entity identity would force the composition to resolve whether the
heard goblin and the seen goblin are the same — leaking truth the
observer doesn't have); "sound coming from a direction would be keyed off
the location" (the crash behind the door is intel about *the place*);
"whichever it is I would think overwrite that belief" (opening the door
overwrites the sound-holding with sight — which connects *because* the
location was the subject). So: the composition names what each report is
about, at the fidelity the observer has actually earned — a place, an
individuated entity, a believed identity ("lord-varen" for the
doppelganger). Subject choice can itself be false or vague; it is
testimony like everything else. This converges with the proven model: the
old `View.Memory` was hex-keyed — location-as-default-subject,
generalized.

### 5. Overwrite per (observer, subject); currency per channel

One current holding per (observer, subject), latest testimony wins,
channel + `At` kept as provenance. Currency is a per-holding set of
sustaining channels (`CurrentVia`): a subject tracked by sight AND
tremorsense doesn't fade when only sight loses it. Status is derived,
never stored: CURRENT if any channel sustains, HELD otherwise — mapping
one-to-one onto the old VISIBLE/REMEMBERED wire contract.

### 6. Two verbs: Surveil and Report

Sight is sustained (complete percepts arrive continuously; fading is
derivable by diff); a crash of pots is discrete (it arrives already a
memory; nothing to fade). Per the clock precedent — when semantics
differ, verbs differ: **`Surveil`** carries the complete-current-percept
contract (first contact / refreshed / faded), **`Report`** lands discrete
testimony directly as HELD. Rejected: a mode flag (flippable per call,
incoherent) and a channel registry (setup ceremony the family avoids).
Kirk's probe "can we Observe something false?" confirmed falsehood is
orthogonal to the verb: illusions and disguises are *surveilled*
falsehoods (the decoy tank of the register), rumors and charm-plants are
*reported* ones. No verb can guarantee truth; one that could would need
world access.

### 7. Fold, not log — and the story lives in Record

Kirk spotted the event-sourced shape ("we derive what we know from
folding the events") and the design inverts it deliberately: **fold
eagerly, persist the state, drop the events.** A retained log grows with
time (every move is a full percept) and the load-verbs-save host
re-serializes per RPC; holdings are bounded by world (observers ×
subjects) with overwrite as built-in compaction — the same reasoning that
rejected the event-sourced clock. The log axis already exists: **Record**
is the retained, ordered, audience-projected story, and intel's returned
deltas are precisely its story beats. Kirk's framing sealed it: "I really
just want to tell the story... seeing that change as we uncover and
gather updated intel." The family's answer: intel is the state of each
mind, record is the story as told, the host wire is the live performance
— and full event sourcing (rebuild intel by replaying record) remains
available *by composition* for the streaming future, imposed on no one.

### 8. Deciders read their own intel — the anti-wall-hack contract

The old `buildPerception` fed monsters the actual world. In the new
family a decider consults `HeldBy(monster)` and nothing else: monsters
are exactly as ignorant, as deceivable, and as staleness-prone as
players. Stealth against a monster = not generating percepts for the
composition to surveil into it. No new mechanism anywhere.

## The naming journey

knowledge → **belief** (Kirk: "there is more in my head than knowledge" —
and knowledge implies truth) → **intel** (Kirk's buddy's noodle — the
same friend whose behavior work will consume this module through
deciders). The espionage register carries every law natively: *bad
intel*, *disinformation*, *stale intel*, *collection channels*,
*provenance* — and everyone who touches intel already knows not to
confuse it with truth. It also upgraded the verb: testimony is
*reported*; sustained collection is *surveillance*. Rejected along the
way: `view` (sight-shaped — the exact bias being escaped; UI/DB
collisions), `memory` (allocator collision), `mind` (implies the
reasoning that deliberately lives in deciders), `ken` (the one clever
word in a plain-word family).

## Links

- `docs/journey/051-encounter-reset-application-to-toolkit.md` — the
  seven axes; this is axis two (lane 1 of two)
- `docs/ideas/play/clock/` — the family's precedent triplet; the laws
  inherited here were proven there
- Old module evidence: `encounter/perception/` (View.Memory hex-keyed
  precedent; pure-vs-geometry-coupled contrast), `encounter/npc.go`
  `buildPerception` (the wall-hack this design retires)
