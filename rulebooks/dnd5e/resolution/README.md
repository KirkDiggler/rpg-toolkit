# resolution

The one place an event bus exists.

A bus is created for a single interaction, everything passed in is attached
to it, the interaction runs, and the bus dies with the call. The encounter
composition raises interactions and stays bus-free; rules packages are step
machines over data; the session seam names IDs. This module is
[ADR-0038](../../../docs/adr/0038-resolution-owns-the-bus.md) implemented —
the full contract, laws R1–R7, and the design rationale live in this
package's godoc ([doc.go](doc.go)), which is canonical. This file is the
orientation.

## One call

```go
out, err := resolution.Resolve(ctx, &resolution.Input{
    World:        worldData,                 // encounter.EncounterData
    Participants: []resolution.Participant{  // every sheet, as data (R3: everyone)
        {Character: heroData},
        {Monster: skeletonData},
    },
    Machine: resolution.NewSave(&resolution.SaveInput{
        SaverID: "hero", Ability: abilities.STR, DC: 12,
    }),
})
```

What happens, in order:

1. **validate** — nil input, no machine, duplicate or ambiguous participants: refused
2. **load the world** — `encounter.LoadEncounter`; deciders pass through
3. **create THE bus** — one per call, shared by everyone in it, wrapped in an
   instrumented surface
4. **attach everyone, sorted by ID** — each sheet through its own view of the
   surface, each effect through a view stamped with its ref
5. **drive the machine** — it yields `Gather`, resolution folds the chain on
   *its* bus and hands back the folded result; it yields `Done`
6. **teardown** — revoke every subscription this call granted, newest first
7. **return data** — the world, the dirty sheets (and only those), the
   outcome, and the full registration list

Nothing survives step 7. Not the bus, not a loaded sheet, not a subscription.

## What comes back

`Output.Hooks` is the registration list: every subscription granted, in
order, stamped with the participant and the effect that made it. It answers
three questions that used to be unanswerable:

- what is attached — *before* anything runs
- what a given effect attached — including **nothing at all**: a zero-hook
  effect is an assertable fact, not silence
- what teardown must revoke — resolution trusts no effect to clean up after
  itself

Attribution is by construction, not interrogation: a `ConditionBehavior`
cannot name itself, and does not need to — whoever routed the ref to build it
stamps the surface before calling `Apply`.

## The vocabulary, deliberately incomplete

`Gather | Done`, and no others. The ADR seals
`Gather | Pose | Request | Done`, but that table was written when exactly one
machine existed — so each case lands with the caller that forces it: `Pose`
with the walk machine, `Request` with concentration. Sealing an enumeration
against hypotheticals is the ADR-0007 mistake.

`Gather` is opaque on purpose: its fields are unexported, so a machine cannot
build one — it calls a constructor here naming the chain it wants, and the
closure that touches the bus is built on resolution's side. "The machine
never sees the bus" is a compile error, not a discipline.

## Where this sits on the migration

The end state (ADR-0038, Consequences): character and monster are rules
vocabulary over pure data, and resolution is the only attacher. A condition
was never really *inside* a character — the sheet carries its persisted JSON,
and the bus belongs to whoever runs the attach loop.

That inversion has landed (#985, #986, #989). The entity loaders come in two
halves now: `character.Load` and `monstertraits.LoadMonster` turn data into a
sheet with no bus anywhere in the call, and `character.Attach` /
`monstertraits.AttachMonster` put that sheet on the bus **this** package made —
the sheet's own keeper first, then each effect through a view stamped with its
ref. The five self-subscriptions that used to pin the bus inside a constructor
are a sheet-keeper attachable, so they show up in the registration list under
the participant that made them rather than as the anonymous zero-Ref entries
this section used to warn about.

Two consequences, worth knowing before you read a failure here:

- **Resolution is strict.** A persisted effect blob this build cannot route
  fails the whole resolution, naming the blob. The legacy path logged it and
  carried on — which is how a sheet came back one effect short and was
  persisted in that state (#948). `Resolve` hands sheets back to be saved, so
  forgiving here means deleting there.
- **A failed attach is a no-op.** Whatever attached before the failure comes
  back off: by the entity's own contract, and again by this package's teardown
  on the error path. A refused resolution leaves no hooks on a bus and no
  half-written sheet.

What remains is retirement rather than inversion. `character.LoadFromData` and
the monster three-call assembly still exist for their other callers, and the
bus a sheet parks for its verb methods is still parked. Both go with #965 and
#966. See "Where this sits on the migration" in [doc.go](doc.go).

## Reading order

1. [ADR-0038 — resolution owns the bus](../../../docs/adr/0038-resolution-owns-the-bus.md)
2. Journeys [053](../../../docs/journey/053-resolution-seam-attach-responsibility.md)
   and [054](../../../docs/journey/054-the-subscribe-interface-and-the-resolution-seam.md)
   — how the shape was reached
3. [The landing plan](../../../docs/ideas/session-sdk/resolution.md) — why a
   saving throw first, and the PR sequence this module is step one of
4. [doc.go](doc.go) — the contract this package is bound by
