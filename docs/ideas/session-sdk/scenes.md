# Session SDK — acceptance scenes

**Written before opening the old code.** That is the point of this document.
If these scenes can be written without consulting what exists, the surface is
shaped by what the game needs; the mapping onto whatever we already have is
labor, not design. If we had written them the other way around, we would have
renamed the old stack and called it a boundary.

Every scene is SDK calls and what comes back. No internals appear — not the
encounter's types, not combat's, not the bus, not a checkpoint. Where the right
answer isn't obvious, it is marked **OPEN** rather than guessed.

---

## Scene 0 — Standing the manager up

The game server owns storage and nothing else. It implements the ports; the
manager refuses to exist without them.

```go
mgr, err := session.NewManager(&session.Config{
    Characters: charRepo,   // character.Data
    Encounters: encRepo,    // encounter.EncounterData
    // no NPC repo yet — monsters are session-scoped; see design.md
    Sessions:   sessRepo,   // windows, frozen resolution, session NPCs
    Events:     stream,     // optional: multiplayer fan-out
})
```

- Missing any required port → construction fails with a named error. No lazy
  discovery at call time, no nil-panic three verbs later.
- Repositories trade in **data**, not domain objects. Reconstitution happens
  inside, where the laws are.

- Every port operation is **get-by-id or put-by-id**. No queries, no scans, no
  joins. The game's store is Redis and the access patterns are key-value; the
  SDK must never require more than that.
- **No content port.** The authored tomb is handed in at `StartSession`, since
  the server already knows where its content lives and that lookup happens once
  per session rather than once per verb.
- **One repository per data type**, never one that saves everything. Clock and
  intel ride inside the encounter because they are *parts of* it; an encounter
  is something a session *points at*, so it gets its own repo. This keeps each
  type's storage strategy the server's business — an encounter held in memory
  on a live server and checkpointed periodically is invisible from here — and
  keeps writes proportional to what actually changed.

---

## Scene 1 — The party enters the tomb

The tomb is authored content that already exists in storage. Entities are
loaded *into* it.

```go
out, err := mgr.Join(ctx, &session.JoinInput{
    Encounter: "enc-123",
    Entity:    "char:alice",
    Room:      "hall",
    Position:  spatial.Position{X: 2, Y: 3},
})
```

- The manager reads the encounter, reads Alice, places her, writes both back.
- `out.Saved` names what was persisted. If Alice saved and the encounter didn't,
  that is an error naming both — not a silent partial.
- rpg-api never held Alice. It named her.

Repeat for the rest of the party and for what's already lurking — an NPC joins
the same way a character does, since `Member.Kind` is what tells the manager
which repository the ID resolves against. **OPEN:** whether an encounter's
resident NPCs are joined by the server or come with the authored content. The
tomb probably knows its own ogre; a wandering one probably doesn't.

---

## Scene 2 — Alice walks into something

The headline scene, and the one that justifies the whole design: **the pause is
born from perception.**

```go
out, err := mgr.Move(ctx, &session.MoveInput{
    Encounter: "enc-123",
    Member:    "alice",
    Path: []spatial.Position{{3,3}, {4,3}, {5,3}, {6,3}},
})
```

Alice walks. At step three the hall opens up and the ogre comes into view. The
world stops.

```go
out.Steps       // three entries — she got to {5,3}, not {6,3}
out.Discovered  // per observer: alice now holds the ogre
out.Pending     // alice owes an answer: continue, or stop here
out.Saved       // encounter + alice, both written, mid-path
```

- The remaining step is **not** discarded. It is frozen, and it is in the blob.
- Nobody else acts. The world is frozen until the answer arrives.
- rpg-api did not compute the pause and could not have: only the world model
  knows what she just saw.

**OPEN:** the shape of `Pending`. It has to carry who owes the answer, what the
choices are, and enough for a client to render the moment — without leaking why
the resolution stopped.

---

## Scene 3 — Answering tomorrow, from a different process

```go
// New process. Same storage. Nothing was held in memory anywhere.
mgr2, _ := session.NewManager(cfg)

p, err := mgr2.Pending(ctx, &session.PendingInput{Encounter: "enc-123"})
// p[0].Audience == "alice"

out, err := mgr2.Answer(ctx, &session.AnswerInput{
    Encounter: "enc-123",
    Window:    p[0].Window,
    Option:    "stop",
})
```

- Alice stops at `{5,3}`. The frozen step is discarded, the window closes,
  everything saves.
- Had she answered `continue`, resolution resumes at step four — the *same*
  resolution, not a new move.

This is the scene that pins the whole discipline: a resolution survives a
process restart because it was never anything but data.

**Rejections that must hold:** answering a closed window; answering with an
option that wasn't offered; someone other than Alice answering Alice's window;
any other verb on this encounter while the window is open.

That last rejection is how "process pending first" is *enforced* rather than
advised — and the error carries the open window and its audience, so the caller
learns what to do from the rejection instead of having to ask a second time.

---

## Scene 4 — Alice swings, and rage does its thing

```go
out, err := mgr.Attack(ctx, &session.AttackInput{
    Encounter: "enc-123",
    Attacker:  "alice",
    Target:    "ogre-1",
    With:      "greataxe",
})
```

Alice is raging. Nobody told the manager that; it is on her sheet, and it
attached itself when she was loaded.

```go
out.Result   // hit, and what it cost the ogre
out.Effects  // rage's bonus damage, itemized enough to narrate
out.Saved    // alice, ogre, encounter
```

- The caller never mentions rage, never queries for it, never applies it.
- **OPEN:** how much of the resolution is itemized on the way out. A client that
  wants to say "+2 rage damage" needs the breakdown; a client that just wants a
  number doesn't. Leaning itemized, because the narration is the product.

---

## Scene 5 — The ogre runs, and Bob gets a choice

```go
out, err := mgr.Move(ctx, &session.MoveInput{
    Encounter: "enc-123", Member: "ogre-1", Path: fleeing,
})
// out.Pending → bob owes an answer: take the opportunity attack?
```

Same `Pending`. Same `Answer`. The caller cannot tell from the shape of its
code that one pause came from perception and the other from a reaction — which
is the property that lets us add reaction kinds forever without rpg-api
changing.

```go
out, err := mgr.Answer(ctx, &session.AnswerInput{
    Encounter: "enc-123", Window: w, Option: "attack",
})
// the attack resolves, then the ogre's remaining movement continues or halts
```

- Bob's reaction is spent. **OPEN:** what restores it — with the world frozen,
  "one per round" needs a round, and free-roam has a clock instead.
- **OPEN:** whether monsters answer their own windows synchronously. Leaning
  yes, via the decider the composition already has, so an NPC reaction is the
  degenerate case of the human path rather than a separate mode.

---

## Scene 6 — The encounter ends underfoot

```go
out, err := mgr.Move(ctx, &session.MoveInput{
    Encounter: "enc-123", Member: "alice", Path: fiveSteps,
})

out.Steps    // three
out.Outcome  // the tomb's exit ending fired at step three
```

Steps four and five never happened, and the caller learns that from the length
of `Steps` rather than from an error. Every verb after this rejects: the
encounter is closed.

---

## Scene 7 — Everyone else finds out

Four players share the tomb. Alice's move is *her* call, but it is *everyone's*
event — and what each of them may know about it differs.

```go
// The server supplied a stream implementation at construction.
out, err := mgr.Move(ctx, &session.MoveInput{...})
```

While that call runs, the stream receives:

- **Alice** — she moved, and she now sees the ogre, and she owes an answer with
  these options.
- **Bob and Carol**, in the same room — Alice moved, and she has stopped, and we
  are waiting on her. **Not** her options.
- **Dave**, two rooms away and unable to see the hall — nothing at all, or at
  most that the world is paused.

Same beat, three payloads. This is why the fan-out cannot live above us: the
difference between those three is intel, and intel is a rule.

If the publish fails, the verb still succeeds. Every beat carries a gapless
`Seq`, so a client that missed one notices the hole and re-queries `Story` from
its last known sequence. The log is the truth; the stream is a fast path.

**And nothing is ever announced that was not saved.** Publish happens after the
write lands, so "the client saw it but the database didn't" cannot occur.

---

## Scene 8 — Making a character (later, but shaped now)

Character creation moves behind this SDK eventually. It is out of scope for the
first waves, but the surface should not make it awkward:

```go
out, err := mgr.CreateCharacter(ctx, &session.CreateCharacterInput{...})
```

The reason to write it down now: it is the scene that proves the manager is a
**session** and not an encounter service. If the verb set only ever makes sense
with an encounter ID attached, we named the thing wrong.

---

## What these scenes assert about the surface

1. **Every verb is `load → act → save → return`.** No handles, no sessions to
   keep alive, no order-of-operations for the caller to get wrong.
2. **`Pending` is the only interrupt vocabulary the customer learns.** One
   shape, one `Answer`, regardless of what stopped the world.
3. **Nothing inner is visible.** Add a checkpoint kind, replace the combat
   engine, retire a workaround — none of it reaches the signature.
4. **Failure is named, not silent.** Partial saves report which pieces landed.
5. **`Path`, not a destination.** The caller says where to walk; what actually
   happened comes back as steps. A one-cell path is a legal degenerate case,
   which is how the game moves today.
6. **The return value is for the caller; the stream is for the table.** A verb
   tells the actor's request what happened; the event stream tells everyone
   else, each of them only what they may know. Multiplayer is not something
   the server assembles from return values.

---

## Scenes deliberately not written yet

Town and shopping; quests; downtime and travel; anything party-scoped that
outlives an encounter. They are coming, and question 1 in the brainstorm — what
the manager is actually scoped to — will be answered by whichever of them lands
first.
