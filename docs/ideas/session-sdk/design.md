# Session SDK — design

**Status:** ratified through wave 2. Waves 0, 1 and 2 shipped
(`encounter/v0.4.0`, `session/v0.1.0`, `session/v0.2.0`); waves 3–5 remain the
plan of record.
Where implementation corrected the design, this document carries the correction
*and the reason* rather than a quiet rewrite — those reasons are the most
reusable thing here.

Derived from `scenes.md`, which was written before opening the old code. Where
this document and the scenes disagree, the scenes win — they are the contract;
this is the shape that serves them.

**Module:** `rulebooks/dnd5e/session`, its own Go module with its own `go.mod`,
its own semver, and its own `gorelease` gate from the first tag.

---

## What it is

The **session manager** is the game server's single point of contact with the
toolkit. It composes composers: the encounter (which itself composes clock,
intel, record, and spatial), characters, monsters, and — when combat gets heavy
enough to deserve it — a combat package of its own.

It holds wiring and lifetime. It holds no domain logic. The moment a rule lives
here rather than in the module that owns that rule, this document has been
violated.

```
        rpg-api  ──implements──▶  repositories (Get/Save)
           │
           └──calls──▶  session.Manager
                             │
                    ┌────────┼────────────┬─────────────┐
                    ▼        ▼            ▼             ▼
                encounter  character   monster      combat
                    │                                (later)
          ┌─────────┼─────────┬────────┐
          ▼         ▼         ▼        ▼
        clock     intel    record   spatial
```

## What it is not

- **Storage.** The server owns that, behind repositories.
- **Authoring.** Encounters, maps, and monster definitions come from content
  pipelines, not from here.
- **Transport.** gRPC, protos, and streams are rpg-api's.
- **Presentation.** It returns facts rich enough to narrate; it never narrates.

---

## Laws

**S1 — The manager holds no domain state.** Every verb loads what it needs,
acts, saves, and drops everything. No caches, no handles, no live sessions. The
game is turn-based; this costs little and buys horizontal scaling, no sticky
sessions, and the elimination of every stale-in-memory-state bug class.

**S2 — No inner type crosses the boundary.** SDK signatures reference
SDK-owned types and stable value types (`spatial.Position`) only. Never
`encounter.MoveOutput`, never a combat result. The instant one leaks, replacing
that module becomes a breaking change and the version-bump promise is void.

**S3 — Repositories trade in data.** They accept and return persistence
shapes. Hydration happens inside, where the laws are. The server stays dumb
storage.

**S4 — Every verb is load → act → save → return.** No setup call, no teardown,
no caller-visible ordering. A verb is complete when it returns.

**S5 — `Pending` is the only suspension vocabulary.** Every pause — perception,
reaction, whatever comes later — surfaces in one shape and resolves through one
`Answer`. A caller cannot tell from the shape of its code what stopped the
world.

**S6 — Failure names its pieces.** A partial save is an error that says what
landed and what didn't. Never a silent shrug, never a bare wrapped error.

**S7 — A frozen resolution is data.** It survives a process restart because it
was never anything else. No goroutine parked, no stack held, no handle to
reclaim.

**S8 — Construction is total.** The manager refuses to exist without everything
it needs. No lazy discovery at call time, no nil-panic three verbs later.

---

## Repositories

**They are called repositories, not ports and not adapters** — a naming
correction Kirk made before the first tag, and worth recording because it is the
kind of thing that gets re-litigated. "Port" reads as a TCP port or as porting
software, and carries hexagonal-architecture baggage nobody here signed up for.
"Adapter" is worse: an adapter sits *between* two parties that cannot speak
directly, and there is no such gap here — the game server implements our
interface and calls us. What these things do is fetch and store records by
identity. That is a repository, so that is the word.

**S12 — Repositories are key-value.** Every operation is get-by-id or put-by-id. No
queries, no scans, no joins, no sorts. The SDK never asks storage a question it
cannot answer with a key. This is what keeps Redis (the game's actual store)
viable forever and makes it structurally impossible to back into requiring a
relational database. It is a constraint on *us*, not a claim about the server.

**S13 — One repository per data type.** Not one repository per aggregate-graph,
and emphatically not one repository that saves everything.

```go
type EncounterRepository interface { /* encounter.EncounterData — the world     */ }
type SessionRepository   interface { /* SessionData             — session state */ }
type CharacterRepository interface { /* character.Data — arrives with entities   */ }
```

**Wave 3 declared `CharacterRepository`, and the deferral paid for itself.**
Waiting meant the interface arrived with a caller to shape it, and the caller
settled a question that would have been guessed a wave earlier: the seam takes
**IDs, not characters**. Verbs name members by ID, `Member.Kind` routes to the
right repository, and `character.Data` appears in exactly one place — the
repository the host implements. It never appears in a verb input. Had the
repository been declared up front, nothing would have forced that distinction
and `Data` would plausibly have leaked into verb signatures where it does not
belong.

The paragraph below is the original reasoning, kept because the rule it states
is the reusable part:

**`CharacterRepository` is deferred to the entities wave, not defined up front**
— a correction the implementation forced. `character` is a package *inside* the
large `rulebooks/dnd5e` module, so declaring it early would take a permanent
dependency on combat, conditions, and spells to satisfy a repository that
nothing calls until entities exist; the free-roam verbs need only member IDs.
The same asymmetric-reversibility rule that governs `NPCRepository` applies:
adding a `Config` field later is compatible, removing one a host has already
implemented is not. Repositories are introduced when something calls them.

Each is get-by-id and put-by-id over exactly one shape.

The save signatures are deliberately not uniform: `SaveSession` takes only the
data because `SessionData` carries its own `ID`, while `SaveEncounter` takes the
key separately because `EncounterData` does not. The alternative — inventing an
ID field on `EncounterData` purely for symmetry, or dropping the one
`SessionData` already has — would put the identity of a record in two places or
none. Passing the key exactly when the payload lacks it keeps a single source of
truth for each.

**An NPC is a shape.** It has its own data, and `Member.Kind` is already the
discriminator that says which shape an ID resolves to — the composition
anticipated this. Monster is one NPC type; shopkeepers, quest givers, and the
priest a curse is watching for are others.

**RULED — NPCs live in `SessionData` for now; `NPCRepository` arrives later.**

The encounter is the life and death of most monsters, and their state after a
session means nothing. So they are session-scoped: they sit in `SessionData`
alongside the windows and the frozen resolution, they vanish when the session
ends, and **there is no cleanup because there is nothing to clean up.**

Not in `EncounterData` — that carries the same purity cost as an interrupt
window would, an aggregate holding entity state it cannot interpret or
validate. `SessionData` is ours, so the eventual migration is confined to this
module.

The reason to start here rather than with the repo is **asymmetric
reversibility**. Adding a repository to `Config` later is a compatible change: a
new field, implemented when it is needed. *Removing* one is not — the server has
already written that implementation. So the cheap direction is to ship without
`NPCRepository` and add it the day a durable NPC exists (a shopkeeper, a quest
giver, a recurring villain who remembers being wounded). Shipping it in the
migration wave and then deciding NPCs were session-scoped would leave us
carrying a repository nobody uses.

**Rejected: delete-on-death.** It loses the corpse. Players loot bodies, and
"the ogre is dead but still lying there" is a state a session should be able to
hold. Vanishing at session close gives that for free; vanishing at death forces
a corpse concept to be invented to get it back.

The line is *composition versus reference*. Clock, intel, and record are **parts
of** an encounter — no independent lifetime, no meaning apart from it — so they
ride inside `EncounterData` and always have. An encounter is not a part of a
session in that sense; it is something a session *points at*. Wrapping it would
have confused the two relationships.

Two things follow, and both were lost by an earlier draft that made
`SessionData` a wrapper:

- **Storage strategy becomes per-type and stays the server's business.** An
  encounter that lives in memory on a live server and checkpoints periodically
  is invisible to the SDK — which is exactly what the repository boundary is
  for, and a good fit, since a path walk should not round-trip Redis per step. A
  wrapper would have welded the encounter's storage to the session's
  permanently. S1 is unaffected: the *manager* holds no state; what sits behind a
  repository is not the manager's concern.
- **Writes stay proportional to what changed.** Opening an interrupt window
  writes a small session blob rather than rewriting every room, connection, and
  member of the tomb. Within a single verb it is one load and one save
  regardless of path length (S4); across verbs, this is what keeps that honest.

The earlier wrapper design existed to make a save atomic — which optimised for
a decision explicitly deferred (see *Observable failure*), and paid for it in
composability. Repo-per-type accepts multi-blob writes and reports their
failures instead.

There is deliberately **no content repository.** Authored content — the tomb, a
monster template — is handed in as a parameter at the moment it is needed,
because the server already knows where its own content lives and that lookup
happens once per session rather than once per verb:

```go
mgr.StartSession(ctx, &StartSessionInput{
    Session:   "sess-123",
    Encounter: authoredTomb,
})
```

If content-fetching becomes real (item catalogs when shopping lands), it can
arrive later as an added `Config` field without breaking anything.

Note the deliberate exception to S2: **repository signatures reference the inner
modules' `Data` types**, because the server persists exactly those bytes. This
is the one place inner types are legal, and it is why they are `Data` types —
persistence shapes are the slowest-changing surface we own, and they already
carry a compatibility discipline. Domain types stay hidden.

`NotFound` must be distinguishable. Repositories return a sentinel the manager
can test, so "no such session" is a clean rejection rather than a mystery.

### The log wants bounding, not a repository

`record.LogData` lives inside `EncounterData` and **is never trimmed** — there
are zero calls to `TrimBefore` anywhere in the composition. So the log grows for
the life of an encounter and every save rewrites the whole story from its first
beat.

The fix is a retention policy, not a new persistence shape. The machinery is
already there and already safe: `TrimBefore` exists, `Seq` is explicitly never
renumbered by it, and queries take a `FromSeq` lower bound — so sequence numbers
stay stable across a trim, which is exactly what a reconnecting client needs.
The write surface is already append-only: `Append` plus `TrimBefore`, entries
never mutated.

**Retention size is a multiplayer-reconnect decision, not a storage decision.**
Under S10 a client that misses events re-queries `Story` from its last sequence;
if it has been disconnected longer than the retention window, that delta is gone
and it must full-resync instead. That is a fine fallback — but it means the
question is "how long can a phone be in a tunnel and still rejoin cheaply,"
*not* "how much history do we want to keep." Size it from the first and the
storage cost falls out; size it from the second and the reconnect behaviour is
discovered by accident.

Two consequences:

- Retention belongs to the encounter as construction config, applied on append,
  so it stays self-contained rather than something the manager must remember.
- `Story` must answer a request for a trimmed sequence with an explicit
  "that range is gone, resync" signal, never a short answer that looks complete.

**RULED — retention is configurable, and the default starts extremely small.**

Small is not a storage economy here; it is a test strategy. A generous window
means the resync path almost never runs, so it stays unexercised until the one
player whose train enters a tunnel finds the bug in it. A tiny window makes
**full resync the common path** — it fires in every dev session, every scene
test, every manual playthrough — so the expensive branch is the well-trodden
one and the cheap delta becomes the optimisation rather than the assumption.
Raise it later from evidence; never start high and hope.

**OPEN:** whether this lands as an encounter change ahead of the SDK or
alongside it. It is small and self-contained enough to ship on its own.

---

## Construction

```go
mgr, err := session.NewManager(&session.Config{
    Sessions:   sessRepo,  // SessionData
    Encounters: encRepo,   // encounter.EncounterData
    Events:     stream,    // required — see "The event stream"
})
```

**Everything in `Config` is required.** Missing any component fails construction
with an error naming it (S8), checked in a fixed order so the first complaint is
deterministic. `CharacterRepository` is absent because nothing calls it yet, not
because it is optional.

**Wave 3 adds `Characters`.** The deferral paid off as intended — the interface
arrived with a caller to shape it instead of a guess to maintain, which is also
why `NPCRepository` is still absent.

But the addition exposes a blind spot in the compatibility gate, and it is worth
recording plainly:

> **`gorelease` will call a new `Config` field compatible, and for a *required*
> field that verdict is wrong.** Adding one compiles everywhere and breaks every
> existing host at runtime, on the first `NewManager` call, because construction
> is total (S8) and the host does not set the field it has never heard of.

This is free right now — rpg-api has not adopted the SDK, so no host is on
`v0.2.0` and there is nothing to break. It stops being free at wave 4, the
migration wave, which is where the version-bump promise starts. So:

**Every required `Config` field must land at or before wave 4.** After that, a
new one is a silent runtime break wearing a green CI check. This is the same
species of gap as the boundary test's sentinel errors — a mechanical gate that
is sound for the thing it inspects and blind to a thing next to it — and it gets
the same treatment: name it, and put the discipline where the gate cannot reach.

This settles the capability-config question `scenes.md` left open, and settles it
by deleting it: there are no optional components, so `Config` never became
capability-shaped. A host that genuinely wants no fan-out passes the
`DiscardEvents` stream we ship — an explicit choice at a call site rather than an
absence the manager has to tolerate everywhere downstream.

---

## The verb surface

Every verb takes a typed `*XInput` and returns a typed `*XOutput` plus `error`
— the house pattern, so fields can be added compatibly forever.

| Verb | Purpose |
|---|---|
| `Join` | Load an entity into an encounter at a placement |
| `Move` | Walk a member along a path |
| `Traverse` | Cross a connection |
| `Answer` | Resolve an open window |
| `Pending` | What windows are open, and who owes them |
| `View` | What a member can perceive |
| `Story` | What has happened |
| `Status` | Open or closed, and the outcome |
| `Atlas` | The static world map |
| `Exit` / `End` | Leave, or close the encounter |
| `Attack` | *(combat wave)* |
| `CreateCharacter` | *(later — the verb that proves this is a session)* |

Read verbs still load and save nothing, but they are still `load → act →
return`: S1 holds, S4's save step is simply empty.

### Movement takes a path

```go
type MoveInput struct {
    Encounter string
    Member    string
    Path      []spatial.Position
}

type MoveOutput struct {
    Steps      []Step     // what actually happened, in order
    Discovered []Sighting // per observer
    Pending    *Pending   // non-nil ⇒ the world is frozen
    Outcome    *Outcome   // non-nil ⇒ the encounter closed
    Saved      SaveReport
}
```

`len(Steps) < len(Path)` is the primary signal, and it is deliberately not an
error: the walk stopped because something happened, and what happened is in
`Pending` or `Outcome`. A one-cell path is a legal degenerate case — that is
how the game moves today, and it is why the encounter's single-hop `Move`
primitive stays exactly as it is.

### Suspension

```go
type Pending struct {
    Window   string   // opaque; stable across a restart
    Audience string   // who owes the answer
    Options  []Option // what they may choose
    Prompt   Prompt   // enough to render the moment
}

type AnswerInput struct {
    Encounter string
    Window    string
    Audience  string // must match; answering someone else's window is a rejection
    Option    string
}
```

`Prompt` carries *what the player is looking at*, never *why the resolution
stopped*. That asymmetry is what lets checkpoint kinds be added forever without
the customer noticing (S5).

**While a window is open, every verb that would change the world rejects.**
Read verbs — `View`, `Story`, `Status`, `Atlas`, `Pending` — remain available,
and must: a client cannot render "waiting on Alice" without asking what the
world looks like, and a freeze that blinded every other player would be a worse
experience than no freeze at all. What is frozen is *change*, not observation.
The
world is frozen — Kirk's ruling — and that is enforced, not merely intended.

### Rejections that must hold

Answering a closed window; answering with an unoffered option; answering
someone else's window; any verb other than `Answer`/`Pending`/read verbs while
a window is open; any verb at all on a closed encounter.

---

## Persistence

**RULED — `SessionData` holds only session state, and references the encounter
rather than containing it.**

```go
type SessionData struct {
    ID        string               `json:"id"`
    Encounter string               `json:"encounter"` // by ID
    Windows   interrupt.LedgerData `json:"windows"`
    Frozen    []byte               `json:"frozen,omitempty"`
    NPCs      []npc.Data           `json:"npcs,omitempty"` // session-scoped; see Repositories
}
```

**Shipped: `ID`, `Encounter`, and `Windows`.** `NPCs` arrives with entities
(wave 3), added the wave that first writes to it — a persisted field nothing
populates is a shape we would be committed to before learning whether it is
right.

**`Frozen` was not needed, and that is worth recording.** This design gave the
session a `Frozen []byte` beside the ledger. Wave 2 found the field redundant:
`interrupt.Window` already carries an opaque payload, one checkpoint opens one
window, and the suspended state belongs to that window rather than to the
session. Shared state across several windows of one resolution is plausible when
reactions land — and at that point `Frozen` is an additive field. Adding it now
would have committed a persisted shape before anything could tell us whether it
was the right one, which is the same rule that deferred `CharacterRepository`,
pointing the same way.

Small, and written only when session state actually changes. The encounter
never learns what an interrupt window is; the session module validates session
state and the encounter validates its own. Each keeps its laws and neither can
be broken by the other's mistakes — which was the goal all along; the wrapper
was simply the wrong way to reach it.

**Rejected: growing `EncounterData` with a window field.** It would spend the
aggregate purity hardened in the anchoring wave on state the encounter cannot
interpret or validate.

**Rejected: `SessionData` wrapping `EncounterData`.** It bought save atomicity —
a decision explicitly deferred — at the cost of per-type storage strategy and
write proportionality (S13).

This also clarifies something the scenes were vague about: **an authored
encounter and a live session are different things.** The tomb is content — a
bare `EncounterData`. Starting a session wraps a copy of it in a fresh
`SessionData`. Content is read once and never written; session state is read
and written every verb.

### Observable failure

```go
type SaveReport struct {
    Written []string // aggregate IDs that landed
    Failed  []string // aggregate IDs that did not
}
```

A partial save returns an error *and* a populated report (S6). No transaction
decision yet — Kirk's call — but the evidence accumulates from the first wave
instead of arriving as a bug report from a player whose monster came back to
life.

---

## The event stream

**RULED — the toolkit owns the events; the game server owns the transport.**
It supplies a stream implementation the way it supplies repositories, and we
drive it. The multiplayer game communicates through those events.

### It already exists

`record.Entry` is an event envelope and has been since `play/record` v0.1.0:

```go
type Entry struct {
    Seq         uint64          // monotonic, gapless, never renumbered
    At          uint64          // provenance timestamp
    Correlation string          // groups cause and effects across entries
    Audience    []core.EntityID // the roster of viewers
    Tags        map[string]string
    Payload     []byte
}
```

Ordering, gap detection, a causality token, delivery scoping, filterable
metadata, content. The composition appends one for every state change. There is
no stream to build — only a delivery interface to attach to the log we already
write.

### Why it cannot live above us

**Audience is a game rule, not transport.** Who may see an event is a function
of intel — who perceived what, through which channel, and when. If the game
server fans events out, it reimplements visibility, and the first bug leaks
hidden information: the unspotted ogre, the trap Bob failed his check on, the
fact that someone is being offered a reaction. Those are rules defects wearing
delivery clothing, surfacing in the layer least equipped to catch them.

### Laws

**S9 — Publish only after the save lands.** Never announce a fact that failed
to persist. "Announced but didn't happen" becomes structurally impossible.

**S10 — The log is the truth; the stream is an optimization over it.** Because
`Seq` is gapless, a client that misses an event sees a hole and re-queries
`Story` from its last known sequence. Publish failure is therefore **not**
fatal to a verb: it is reported in the output and the client heals itself. For
a Discord activity on a phone, this is the difference between a robust game and
a support burden.

**S11 — Events are projected per audience, not merely filtered.** Two viewers of
the same beat may receive different payloads. The pending-window event is the
motivating case: the actor's client needs the options; everyone else needs
"waiting on Alice" *without* them, because options leak — offering an
opportunity attack reveals that a threatener exists and roughly where.

The audience question is answered by *asking the composition*, never by
re-deriving visibility here. That is the load-bearing half: a second
implementation of who-may-know, living outside the module that owns perception,
would eventually disagree with the first, and a disagreement about visibility is
a leak.

**Implementation note:** the composition currently addresses every beat to every
member ([toolkit#940](https://github.com/KirkDiggler/rpg-toolkit/issues/940)),
so today's fan-out is correct with respect to its contract and too generous with
respect to the game. That is deliberately left as the composition's problem —
and it is the layering working, since scoping audiences there fixes the stream
here for free.

### The interface

```go
type EventStream interface {
    Publish(ctx context.Context, events []Event) error
}
```

**RULED — the stream is required, not optional.** An earlier draft of this
document had it as an opt-in multiplayer capability. That was wrong twice over,
and Kirk caught it before the first tag.

Wrong on the game: a verb's return value carries what *that caller* needs to
know, and it structurally cannot carry what happened to everyone else. Single-
player and multiplayer alike render from the stream — it is the live channel, not
a fan-out feature bolted onto one. A single-player client with no stream sees its
own moves and nothing the world does back.

Wrong on the timing, which is the more general lesson: **this package is being
introduced here.** No host has implemented `Config` yet, so requiring the stream
today costs exactly nothing. Requiring it in v0.3.0 would break every host that
took us up on the option. Loosening a rule later is compatible; tightening it is
not — so a rule we are even slightly inclined toward belongs in the first tag,
where the reversible direction is still available. This is the same asymmetry
that deferred `CharacterRepository`, pointing the other way.

A host that truly wants no fan-out passes `DiscardEvents`, which we ship.

`Event` is an **SDK-owned envelope**, not `record.Entry` re-exported — settled
by the proto mapping, and forced independently by S11. See "Shaped for the
wire" below.

## Shaped for the wire

The rulebook has a corresponding proto that the game server implements, so SDK
output types are **proto-shaped by construction**: flat structs, explicit
enumerated kinds, nothing polymorphic, no interface-valued fields, no `any`.

Two consequences worth stating because they are easy to get wrong once and
never fix:

- **`Event` is an SDK-owned envelope**, not `record.Entry` re-exported. S2 wanted
  that anyway, and S11 forces it regardless: with per-audience projection, the
  thing on the wire is genuinely not the thing in the log.
- **`Option` carries a stable machine-readable kind**, not a free-form string
  the client pattern-matches. It is the field clients branch on, so it is the
  field that must never be prose.

## Compatibility

`gorelease` gates every release from the first tag, so "rpg-api only bumps a
version" is CI rather than intention. Every internal replacement wave must come
back **compatible** or it does not ship.

This is what the strangler strategy is buying and it is worth stating as a
falsifiable claim: *after the migration wave, no subsequent wave changes any
rpg-api source file.* If one does, S2 was violated somewhere and the violation
is findable.

### Two promises, not one

Wave 3 forced a distinction the allow-list had been blurring. Its entries looked
like one list of exceptions, but they were always two kinds of thing:

| | Example | Does the host construct it? |
|---|---|---|
| **Contract type** | `spatial.Position` | Yes |
| **Persistence shape** | `encounter.EncounterData`, `interrupt.LedgerData` | No — round-trips it untouched |

These carry different promises. A persistence shape promises **replaceability**:
we can swap what is underneath and the host never notices, which is why only
`interrupt.LedgerData` crosses and `interrupt.Window` never does. A contract type
promises the opposite — it is **shared vocabulary**, a domain noun both sides
name, and a change to it is a breaking change we announce rather than hide.

`character.Data` is a contract type, beside `spatial.Position`. A character is a
thing, not an implementation detail we would want to refactor without telling
the host. Reading it as a grudging exception to the replaceability promise gets
the intent exactly backwards: for this category, surfacing a change is the
correct behavior.

Two facts make it a comfortable one. Its type surface bottoms out in strings and
ints — `skills.Skill` and `classes.Class` are literal aliases to `string`, the
`races`/`abilities`/`proficiencies` types are defined string types, and features
and conditions are already `[]json.RawMessage`. And rpg-api has imported
`rulebooks/dnd5e/character` in 47 files since well before this SDK existed, so
naming it here joins coupling that already exists rather than creating any.

The version-bump claim above is unaffected, because it was only ever about
replaceability. **The allow-list should name both categories rather than reading
as a flat list**, so a future exception is weighed against the right promise.

The direction this points: `character` eventually becoming its own module
alongside `encounter`, with its own version line. That also dissolves the one
real cost of naming it here — today it inherits the release cadence of the
`rulebooks/dnd5e` root module, which has 179 tags. Not a wave-3 blocker; wave 3
consumes `character.Data` from the root module as it stands.

---

## Inside the boundary

None of this is customer-visible; all of it can change under a compatible tag.

- **Chains** (`core/chain`) carry modification during resolution — rage's bonus
  damage, resistance, a fighting style.
- **The bus** carries observation — "was I hit?", "turn ended", the curse
  watching for a priest. Attached for the duration of one verb, since entities
  are loaded per call (S1). Journey 052's rule holds: control flow never rides
  it.
- **Checkpoints** are the only place a suspension is born. Enumerated in an
  order that is a function of persisted data, never of subscription order,
  because a resumed resolution must resume identically — C8, the encounter
  composition's determinism law: identical inputs yield identical outputs and
  byte-identical blobs.
- **Resolution is a re-enterable phase machine from the first line of combat
  code** — explicit phase index, no Go stack held across a wait, in-between
  state serializable — even in waves where nothing suspends. `ReactionTrigger`
  happened because attack resolution was a straight-line function.

### Loading an entity, and the cleanup that must not happen

There is no session process. A verb loads what it needs, attaches it to a bus
created for that call, acts, writes back, and returns; the whole object graph is
garbage the moment the response is written. `Answer` is not a resumption of
anything living — it is the same load-and-attach performed again from persisted
data.

```go
ch, err := character.LoadFromData(ctx, data, bus)  // features + conditions subscribe
...act...
out := ch.ToData()                                 // durable state back to the blob
```

**`character.Cleanup` must not be called in this loop.** Its first statement is
`c.conditions = nil`, and `ToData()` serializes `c.conditions` — so cleaning up
before the save persists a character with **zero conditions**, with no error and
no failed call. Rage, unconscious, a death save in progress: gone. Its other
half, unsubscribing, buys nothing here because the bus dies with the response.
`Cleanup` is built for a long-lived character in a long-lived process, which is
the architecture we do not have.

Skipping it is safe rather than merely tolerable: conditions intercept on the
bus rather than mutating character fields, so there is no modification left
un-reversed. `RagingCondition.Remove` is a pure unsubscribe. rpg-api has been
doing exactly this — load with an inline `NewEventBus()`, `ToData()`, no
cleanup.

**This is why durable condition state must round-trip through the blob**, and it
is forced rather than chosen. Wave 2 established that no Go stack survives a
wait; a loaded character with live subscriptions is exactly that kind of state.
So a suspension drops every entity, and `Answer` rebuilds them from data.
Anything a condition holds that does not survive `ToData()` is lost across a
suspension — silently, and only in the one path hardest to notice.

The cost of all this is real and structural: every verb pays load and attach for
every entity, and `LoadFromData` wires each feature and condition individually.
That is the price of the condition decoupling, and it is the risk this plan
already named. Wave 3 measures it rather than reasoning about it.

---

## Open questions carried forward

From `scenes.md`, unresolved and listed here so they are not lost: whether
monsters are authored into an
encounter or joined by the server; how itemized an attack result should be;
reaction economy without rounds; whether monsters answer their own windows
synchronously; and where the manager's durable state lives.

And the naming test, which only a later wave can settle: if every verb turns
out to need an encounter ID, this is an encounter service and deserves a
different name.
