# Session SDK — brainstorm

**Status:** open question space. Nothing here is ratified. Kirk's rulings are
marked **RULED**; everything else is a lean or an open.

**Origin:** this started as a scoping question on
[#916](https://github.com/KirkDiggler/rpg-toolkit/issues/916) (suspendable
movement + opportunity-attack-with-option) and turned into something larger the
moment we asked who the customer is. #916 survives as a behavior inside this,
not as a wave of its own.

---

## How we got here

The question was where a suspendable movement resolution should live: the old
`rulebooks/dnd5e/combat` stack, the new `rulebooks/dnd5e/encounter`
composition, or a standalone phase-machine module.

Four findings moved the answer, in order:

1. **`encounter.Move` is a single hop, not a walk.** It calls
   `orchestrator.MoveEntity` once with a destination — no path, no steps, no
   distance limit. The old stack's `combat.MoveEntity` *does* walk cell by cell.
   An opportunity attack is a property of the path, so somebody has to walk it.

2. **Walls are real, and they were already there.** `spatial.Boundary` is an
   undirected crossing between adjacent cells with separate `BlocksMovement`
   and `BlocksLineOfSight` flags; the composition registers them and Atlas
   projects both endpoints into absolute space. A doorway is simply a crossing
   that isn't blocked — which is exactly why a monster standing in one can see
   into both rooms, and why an arrow crosses it.

   Therefore **T3 ("sight never crosses a connection") is not geometry.** It is
   a room-scoped shortcut written when rooms were islands, sitting on top of a
   crossing-based model that does not care which room a cell belongs to.
   Anchoring made it obsolete. Retiring it is a perception wave: field-level
   line of sight computed over Atlas, which is the only thing in the system
   that sees every room at once. Spatial cannot own it — its boundaries are
   registered *within* a room, and a doorway crossing spans two.

3. **The composition has no entities.** `Member` is `{ID, Kind, Room}`. No hit
   points, no sheet, no conditions, no damage, no attack. So
   opportunity-attack-with-option — #916's headline — cannot be built there
   today: the window would open, the player would answer "I swing," and there
   is nothing to swing with.

4. **The customer's loop is the actual design constraint.** rpg-api wants:
   load data (encounter, monsters, players), load it into the encounter, take
   actions, get data back, save. Measured against that loop, step two is
   missing entirely.

---

## The shape

**RULED — the encounter is the world; an SDK above it is the table.**

The encounter stays what it is: geometry, placement, perception, story,
endings. No entities, no bus, no combat. Its aggregate discipline (one
`ToData`, reject-never-crash, W1–W5 validated identically at both seams) is
worth protecting, and stuffing character sheets into it is what would dilute
it.

Above it sits a **session manager** that holds what isn't the world: the roster
of live entities, the bus they attach to, the resolution pipeline, and
interrupt custody.

**RULED — it is a composer of composers, in the AWS-SDK sense.** Not one client
with eighty methods. Encounter is its own thing; players are their own thing.
The manager owns wiring and lifetime, never domain logic — the same trick the
encounter already pulls with clock, intel, record, and spatial. **When combat
gets heavy inside it, combat becomes its own package** with its own laws and
tests, and the manager just ensures it plays nicely with the supporting cast.

### What this buys immediately

Path movement stops being an encounter feature — the manager walks a path by
calling `encounter.Move` one cell at a time. The single-hop `Move` was the
right primitive all along.

Two things then come free, with no new encounter code:

- **Sight-per-step.** `Move` already refreshes sight and returns deltas on
  every call, so walking a path yields "Alice just saw the ogre" at step three.
- **Endings mid-path.** `Move` already evaluates `ReachedPosition` and returns
  an outcome; the manager sees it and stops walking. Reaching the exit on step
  three means steps four and five correctly never happen.

---

## The customer contract

**RULED — repositories, implemented by the game server, trading in data.**

```go
mgr, err := session.NewManager(&session.Config{
    Characters: charRepo,   // GetCharacter / SaveCharacter
    Encounters: encRepo,    // superseded — see design.md
    Monsters:   monRepo,    // superseded — see design.md
})  // refuses to construct without everything it needs

out, err := mgr.Move(ctx, &session.MoveInput{
    Encounter: "enc-123", Member: "alice", Path: path,
})
// out.Pending != nil  →  alice owes an answer

out, err := mgr.Answer(ctx, &session.AnswerInput{...})
```

`GetCharacter` returns `character.Data`; the manager hydrates. The game server
stays dumb storage and reconstitution stays in the toolkit, where the laws are.
**RULED — same seam as everywhere else.**

**RULED — stateless per call.** We are turn-based, and this simplifies a lot.
Each verb loads what it needs, acts, saves, and drops everything. The bus and
the live entities exist only for the duration of one call: conditions attach,
do their work, and go away, with durable state living in the character blob
where it belongs. No stale in-memory state, no sticky sessions, horizontal
scaling for free.

The consequence worth stating plainly: **rpg-api never holds a domain object.**
They implement two or three interfaces once and then call verbs with IDs. That
is strictly simpler than today, where they hold the encounter and juggle its
blob.

It also makes suspension nearly invisible to them. Load → walk → freeze → save
happens inside one call; the answer arrives days later as another call that
loads, resumes, and saves. From outside, a verb returned `Pending` and later
didn't.

**RULED — atomicity gets no hard decision yet, only observable failure.** Every
verb reports what it persisted and what it didn't; a partial save is an error
that names the pieces rather than a silent shrug. The elegant answer will show
itself once real use cases exist. (The shape it might take: a `WithTx`-style
port the server implements however its database wants, and an in-memory test
implementation no-ops.)

---

## Strategy: wrap first, replace behind it

**RULED — put the existing combat/encounter implementation behind the SDK.**
rpg-api migrates once, mapping to existing functionality internally, and from
then on only ever updates their version of the SDK while we replace the insides
wave by wave.

Two disciplines decide whether that promise survives contact:

- **The surface comes from use cases, not from the old stack.** The failure
  mode is a wrapper shaped like the thing it wraps — rename a result type, ship
  it, and the old shape is the public contract forever. Test: write the
  acceptance scenes purely as SDK calls *before* opening the old code. If they
  can be written without consulting what exists, the surface is honest and the
  mapping underneath is labor.
- **No inner type appears in an SDK signature.** Not `encounter.MoveOutput`,
  not `combat.MoveEntityResult`. The moment one leaks, replacing that module
  becomes a breaking change and the version-bump promise is dead. Stable value
  types (`spatial.Position`) may pass through; results and aggregates may not.
- **`gorelease` gate from day one**, so "rpg-api only bumps a version" is CI
  rather than intention. Every internal replacement wave must come back
  *compatible* or it doesn't ship.

---

## Three mechanisms, not one

The bus argument resolved by splitting what rage actually does:

| Job | Mechanism | Example |
|---|---|---|
| Modify a value during resolution | **chain** (`core/chain`), ordered pipeline | rage adds damage, grants resistance |
| React to a fact that already happened | **bus**, publish/observe | "was I hit? extend the rage"; "turn ended, decrement"; the curse that watches for a priest and removes itself |
| Stop the world and ask a human | **checkpoint**, enumerated, suspension returned as a value | opportunity attack; Shield |

Journey 052's rule was never "no bus" — it was **control flow never rides the
bus.** Rage modifying damage is not control flow. Rage extending itself is not
control flow. Suspension is.

The checkpoint is enumerated rather than published for one reason: a suspension
must resume identically after a process restart. If two effects fire on the
same step, subscription order is whatever the loader happened to re-establish;
enumeration order is a function of persisted data. C8 — the encounter
composition's determinism law, that identical inputs yield identical outputs and
byte-identical blobs — dies under the first and
holds under the second.

**Things live where they are from** — this is preserved, and it is what killed
an earlier proposal for a central watcher registry the encounter would
maintain. Rage lives on the character. A trap's position is map content; its
behavior lives with the trap. The manager walks to them; it does not track
them.

**RULED — the trap is not an interrupt.** It does its thing. So a checkpoint
has four outcomes and only the last suspends: do nothing, halt the movement,
resolve something (damage, a stun), or ask someone. The shape must support the
trap without the trap being a question. (A *stun* or otherwise disabling trap
may reopen this — parked.)

---

## Scope beyond the encounter

**Character creation moves here.** Later: a quest-like thing, a town to shop
in. The encounter is one phase of something longer, which is why the unit is a
*session* rather than a fight.

---

## Open questions

1. **Naming and scope of the unit.** `session` fits the breadth (create a
   character, shop, quest, fight). Every verb sketched so far is scoped by
   encounter ID, which reads more like a stateless service than a session. If
   party-level state (downtime, travel, a campaign clock) is coming, the thing
   that owns it may deserve the name.
2. **Does the manager own resolution, or only orchestrate it?** If it walks
   steps and consults entities, chains and attack resolution live there — and
   `rulebooks/dnd5e/combat` becomes something it calls rather than something
   the encounter grows. Lean: yes, and this is where "what does ideal
   composable combat look like" gets asked, since it is the first place with
   both a world and entities in the same room.
3. **Reaction economy.** With the world frozen, "one reaction per round" needs
   a round, and free-roam has a clock instead.
4. **Cross-doorway threat.** Absolute geometry makes the two endpoint cells
   adjacent, so a monster in the vault threatens a player in the corridor —
   while T3 says neither can see the other. Being attacked unseen is fine in
   5e; *taking* an opportunity attack on something unseen is not. Blocked on
   the perception wave that retires T3.
5. **"Hidden."** A perception check unhides a trap for one character and not
   another, which is exactly intel's per-observer holdings model. Not yet
   designed.
6. **First acceptance scene.** Lean: the interrupt born from perception —
   Alice walks a path, step three refreshes her sight, the ogre comes into
   view, the world freezes, she is asked *continue or stop*, we persist
   mid-path, restart the process, and she answers minutes later. It needs no
   combat, and rpg-api cannot generate that pause itself because only the
   world model knows what she just saw.

---

## What this does to #916

#916 as written cannot ship: its headline (opportunity-attack-with-option)
needs entities the composition does not have, and its framing ("retires the
`ReactionTriggerEvent` workaround") assumed we would be editing the old stack.
The interrupt spine becomes a property of how the session manager resolves,
not a feature bolted onto the encounter.

The lesson that keeps the reordering honest: **build resolution as a
re-enterable phase machine from the first line of combat code** — explicit
phase index, no Go stack held across a wait, in-between state serializable —
even in the wave where nothing suspends yet. `ReactionTriggerEvent` happened
because attack resolution was a straight-line function, not because reactions
came late. Get that right and waves can be ordered by customer need instead of
by architectural fear.
