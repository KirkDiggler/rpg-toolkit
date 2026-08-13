# Session SDK — design

**Status:** proposed, not ratified. Derived from `scenes.md`, which was written
before opening the old code. Where this document and the scenes disagree, the
scenes win — they are the contract; this is the shape that serves them.

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
        rpg-api  ──implements──▶  ports (Get/Save)
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

- **Storage.** The server owns that, behind ports.
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

**S3 — Repositories trade in data.** Ports accept and return persistence
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

## Ports

**S12 — Ports are key-value.** Every operation is get-by-id or put-by-id. No
queries, no scans, no joins, no sorts. The SDK never asks storage a question it
cannot answer with a key. This is what keeps Redis (the game's actual store)
viable forever and makes it structurally impossible to back into requiring a
relational database. It is a constraint on *us*, not a claim about the server.

**S13 — One repository per data type.** Not one repository per aggregate-graph,
and emphatically not one repository that saves everything.

```go
type CharacterRepository interface { /* character.Data           */ }
type EncounterRepository interface { /* encounter.EncounterData  */ }
type MonsterRepository   interface { /* monster instance data    */ }
type SessionRepository   interface { /* SessionData — only       */ }
```

Each is get-by-id and put-by-id over exactly one shape.

The line is *composition versus reference*. Clock, intel, and record are **parts
of** an encounter — no independent lifetime, no meaning apart from it — so they
ride inside `EncounterData` and always have. An encounter is not a part of a
session in that sense; it is something a session *points at*. Wrapping it would
have confused the two relationships.

Two things follow, and both were lost by an earlier draft that made
`SessionData` a wrapper:

- **Storage strategy becomes per-type and stays the server's business.** An
  encounter that lives in memory on a live server and checkpoints periodically
  is invisible to the SDK — which is exactly what a port boundary is for, and a
  good fit, since a path walk should not round-trip Redis per step. A wrapper
  would have welded the encounter's storage to the session's permanently. S1 is
  unaffected: the *manager* holds no state; what sits behind a port is not the
  manager's concern.
- **Writes stay proportional to what changed.** Opening an interrupt window
  writes a small session blob rather than rewriting every room, connection, and
  member of the tomb. Within a single verb it is one load and one save
  regardless of path length (S4); across verbs, this is what keeps that honest.

The earlier wrapper design existed to make a save atomic — which optimised for
a decision explicitly deferred (see *Observable failure*), and paid for it in
composability. Repo-per-type accepts multi-blob writes and reports their
failures instead.

There is deliberately **no content port.** Authored content — the tomb, a
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
arrive later as an optional capability without breaking anything.

Note the deliberate exception to S2: **port signatures reference the inner
modules' `Data` types**, because the server persists exactly those bytes. This
is the one place inner types are legal, and it is why they are `Data` types —
persistence shapes are the slowest-changing surface we own, and they already
carry a compatibility discipline. Domain types stay hidden.

`NotFound` must be distinguishable. Ports return a sentinel the manager can
test, so "no such session" is a clean rejection rather than a mystery.

### The log wants bounding, not a port

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
    Characters: charRepo,
    Sessions:   sessRepo,
    Events:     stream,   // optional capability
})
```

Missing a required port fails construction with an error naming it (S8). Which
ports are required is a function of which capabilities the config declares —
the open question in `scenes.md` about capability configs lands here.

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

**While a window is open, every other verb on that encounter rejects.** The
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
}
```

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
no stream to build — only a delivery port to attach to the log we already write.

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

### Port

```go
type EventStream interface {
    Publish(ctx context.Context, events []Event) error
}
```

Optional capability: a single-player setup, a test, or a headless simulation
constructs without one and simply produces no stream. This is the first case
that makes `Config` capability-shaped rather than all-required, which settles
the port-granularity open question in `scenes.md`.

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
  because a resumed resolution must resume identically (C8).
- **Resolution is a re-enterable phase machine from the first line of combat
  code** — explicit phase index, no Go stack held across a wait, in-between
  state serializable — even in waves where nothing suspends. `ReactionTrigger`
  happened because attack resolution was a straight-line function.

---

## Open questions carried forward

From `scenes.md`, unresolved and listed here so they are not lost: port
granularity under capability configs; whether monsters are authored into an
encounter or joined by the server; how itemized an attack result should be;
reaction economy without rounds; whether monsters answer their own windows
synchronously; and where the manager's durable state lives.

And the naming test, which only a later wave can settle: if every verb turns
out to need an encounter ID, this is an encounter service and deserves a
different name.
