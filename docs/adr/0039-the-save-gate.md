# ADR-0039: The Save Gate — Contestability Is Data, DC Formulas Are a Closed Enum

**Date:** 2026-08-14
**Status:** Accepted (Kirk, 2026-08-14)

## Context

Kirk's framing, which this ADR makes concrete: *"something can declare it can
be countered with a saving throw, have the ability(s) that can be checked,
what the outcome of a saving throw [is] (half or full negation), and the
modifiers could be added to the chain."*

Five cases in the current roster need a save to gate a consequence, and they
disagree on every axis a design might fix:

| Case | Gates | DC | Recurrence |
|---|---|---|---|
| Wolf bite → prone | a condition | static 11 | none |
| Open Hand Flurry → prone | a condition | static (8 + prof + WIS) | none |
| Ghoul claws → paralyzed | a condition | static | save each turn-end to end |
| Undead Fortitude | a discrete outcome (1 HP) | **5 + damage taken** | none |
| Bless concentration | revoking grants | **max(10, damage ÷ 2)** | per hit |

Settled before this ADR, and not reopened by it: the *mechanism* is
[ADR-0038]'s `Request` step — an effect on the bus can contribute to a chain
but cannot gate a consequence, so gating is a follow-up interaction whose
result feeds back. Every save resolves through `SavingThrowChain` (no effect
rolls its own d20 — the Undead Fortitude implementation that does, #977, is a
bug, not a precedent). This ADR designs the **declaration** only.

Three facts, learned by shipping the resolution module, moved the decision:

1. **Pre-execution inspectability stopped being aspirational.** The
   instrumented surface's registration ledger made "this effect attached
   zero hooks" an assertable fact. A declaration-free design would make
   *contestability* the new unobservable — rebuilding the exact blindness
   just paid for.
2. **The house already has the pattern.** `resolution.Gather` is exported
   data with unexported workings: machines name what they want through a
   constructor, and resolution alone interprets. A save gate can be built
   the same way.
3. **The derived-DC space is closed.** Of the five cases, exactly two DCs
   derive from the triggering event, and both are named rules in the PHB.
   "Derivation" is not an open modelling problem; it is two enum cases.

## Decision

**A consequence that can be contested declares a `SaveGate` — pure data —
and resolution turns the declaration into a `Request`.**

```go
type SaveGate struct {
    // Abilities the saver may use. One entry for most gates; the wolf's
    // knockdown is STR where the monk's Flurry is DEX — same gate shape,
    // different ability, which is why the ability lives in the declaration
    // and not in a "knockdown" concept.
    Abilities []abilities.Ability

    // DC is where the number comes from — see the closed enum below.
    DC DCSource

    // OnSuccess is what a successful save buys: Negated (nothing happens)
    // or Half (half damage). A gate producing a condition uses Negated.
    OnSuccess SaveEffect

    // Recurrence is WHEN the save happens: None (on application) or
    // EndOfTurn (save at the end of each of the saver's turns to end the
    // effect — the ghoul's paralysis). Deliberately a separate axis from
    // OnSuccess; conflating "what success does" with "when you may try"
    // was the most likely modelling mistake here.
    Recurrence Recurrence
}
```

**`DCSource` is a closed enum of named formulas, not a function:**

- `DCStatic(n)` — the common case; the wolf's 11
- `DCFivePlusDamageTaken` — Undead Fortitude's `5 + damage taken`
- `DCHalfDamageFloorTen` — concentration's `max(10, damage ÷ 2)`

**The extension discipline: a new `DCSource` case must cite a RAW rule** —
the same price extending the step vocabulary pays (an ADR). This is the line
that keeps the enum from becoming a formula language: 5e's designers already
closed this set; we inherit their closure instead of inventing an open one.

Consequences of the shape:

- **Content stays data.** A monster's stat block carries its gates the way it
  carries `damage.Type`; authoring a gated bite requires no Go. The wolf's
  `KnockdownDC: 11` becomes `SaveGate{[STR], DCStatic(11), Negated, None}`.
- **"What can I resist?" is answerable from data** — for the client UI, for
  encounter tuning, and before execution. A stat block cannot lie about
  whether a save exists (#962's founding complaint).
- **Modifiers arrive by construction**: the emitted `Request` resolves
  through the save machine, which folds `SavingThrowChain` — so Raging,
  Bless, and friends contribute without the gate knowing they exist.
- **#927 takes a dependency, not a definition.** `Damage.Save` becomes "a
  damage pool may carry a `SaveGate`"; the gate is defined here. The ooze is
  the proof they are separable: it needs multi-pool damage *and* a gate,
  independently.

## Options considered and rejected

- **Pure static data (`DC int`)** — dies on the DC axis at a *known* date:
  Undead Fortitude (#977) and concentration are both already in the roster.
  Choosing it schedules a re-design for the third consumer.
- **No declaration; machines request saves in code** — derived DCs become
  trivial, but "can this be saved against?" becomes unanswerable without
  executing, monster authors are pushed into Go, and stat blocks can lie
  again. Rejected for cutting against both the ledger and the data-authoring
  direction.
- **Data with a function escape hatch (`FromEvent(fn)`)** — covers
  everything, but the `fn` arm is the seam where a data model starts
  becoming a language, which is [ADR-0038]'s stated reason for deferring the
  effects DSL. The closed enum keeps the coverage and deletes the seam.
- **No gate concept; contests are two-step machines authored as such** —
  conceptually the purest, and rejected for this codebase's content model:
  every monster becomes code, and no UI or tuning question is answerable
  from the stat block.

## Sequenced out, deliberately

- **Condition immunity does not exist and is not designed here.** The gate's
  first consumers need it eventually (undead are immune to the *poisoned
  condition*, not just poison damage; the ghoul's paralysis exempts elves),
  but it is orthogonal to the gate's shape and becomes its own issue.
- **The `Request` step itself** lands with the first consumer (#962, the
  knockdown lane), per the one-case-at-a-time rule the vocabulary already
  follows.

## First consumers

#962 instantiates the gate monster→player (wolf bite, STR, static DC,
condition). Open Hand Flurry is the mirror image player→monster (DEX,
derived-static DC) and lands with the monk's level-3 content. Undead
Fortitude (#977) is the first `DCFivePlusDamageTaken` consumer and retires
the roll-your-own-d20 bug when it converts.

[ADR-0038]: 0038-resolution-owns-the-bus.md
