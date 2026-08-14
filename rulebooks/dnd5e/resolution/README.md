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

Today is one inversion short: `Resolve` delegates each load to
`character.LoadFromData` / the monster three-call assembly, which still take
a bus (the `EffectScoper` seam, #982, keeps attribution per-effect through
the delegation). What pins the bus there is not conditions but the entity's
own five self-subscriptions — the sheet reacting to the world. The remaining
migration PR inverts the loop into this package, drops the bus from the
entity loaders, and extracts those handlers into a sheet-keeper attachable.
See "Where this sits on the migration" in [doc.go](doc.go).

## Reading order

1. [ADR-0038 — resolution owns the bus](../../../docs/adr/0038-resolution-owns-the-bus.md)
2. Journeys [053](../../../docs/journey/053-resolution-seam-attach-responsibility.md)
   and [054](../../../docs/journey/054-the-subscribe-interface-and-the-resolution-seam.md)
   — how the shape was reached
3. [The landing plan](../../../docs/ideas/session-sdk/resolution.md) — why a
   saving throw first, and the PR sequence this module is step one of
4. [doc.go](doc.go) — the contract this package is bound by
