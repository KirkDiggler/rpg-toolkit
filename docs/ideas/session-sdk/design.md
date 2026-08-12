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

One interface per aggregate, so a server that has no monsters yet can still
construct a character-creation-only manager.

```go
type CharacterRepository interface {
    GetCharacter(ctx context.Context, id string) (*character.Data, error)
    SaveCharacter(ctx context.Context, data *character.Data) error
}

type EncounterRepository interface {
    GetEncounter(ctx context.Context, id string) (*encounter.EncounterData, error)
    SaveEncounter(ctx context.Context, id string, data *encounter.EncounterData) error
}

type MonsterRepository interface { /* symmetric */ }
```

Note the deliberate exception to S2: **port signatures reference the inner
modules' `Data` types**, because the server persists exactly those bytes. This
is the one place inner types are legal, and it is why they are `Data` types —
persistence shapes are the slowest-changing surface we own, and they already
carry a compatibility discipline. Domain types stay hidden.

`NotFound` must be distinguishable. Ports return a sentinel the manager can
test, so "no such encounter" is a clean rejection rather than a mystery.

---

## Construction

```go
mgr, err := session.NewManager(&session.Config{
    Characters: charRepo,
    Monsters:   monRepo,
    Encounters: encRepo,
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

The manager's own durable state — the interrupt ledger and the frozen
resolution bytes — needs a home. It rides in the **encounter's** blob rather
than a third one, because its lifetime is exactly the encounter's and because
a save that spans two blobs is a save that can half-fail.

**OPEN:** this means the encounter's `EncounterData` grows a field for state it
does not interpret. That is a real cost against the aggregate purity we spent a
wave hardening, and the alternative — a `SessionData` blob of our own — trades
it for a second thing to keep consistent. Decide before the first tag.

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
