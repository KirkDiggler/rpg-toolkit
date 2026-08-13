---
name: rulebooks/dnd5e/session module
description: The game server's single integration surface — a manager composing the encounter behind repositories, with inner types held off the boundary so the insides can be replaced without the host changing
updated: 2026-08-13
confidence: high — written alongside the implementation (#938, #943); every claim below is pinned by a test in the module
---

# rulebooks/dnd5e/session module

**Path:** `rulebooks/dnd5e/session/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session`
**Grade:** A-

The host's one point of contact with the toolkit. A game server implements two
repositories and an event stream, and thereafter holds no domain object: it
names things and calls verbs.

## Where it sits

```
rpg-api ──implements──▶ repositories (Get/Save)
   └────────calls──────▶ session.Manager
                              │
                         encounter (the world)
                              │
              ┌───────┬───────┼────────┐
            clock   intel   record   spatial
```

The encounter composition is the **world** — geometry, placement, perception,
story, endings. This module is the **table**: entity roster, event fan-out,
resolution, and interrupt custody. It holds wiring and lifetime, never rules.

## Why it exists

Not to add capability. To make the toolkit's insides replaceable without the
host changing a line, so waves of internal rework cost rpg-api a version bump
rather than a migration.

That is a falsifiable claim, and it is gated two ways:

- **`boundary_test.go`** parses the package's own source and fails if any
  toolkit type is reachable from an exported declaration. Verified against a
  real leak, not a synthetic one; carries a meta-pin so it cannot silently stop
  biting. Its allow list has three entries, each with a written reason — all of
  them persistence shapes the host already stores, never a domain type.
- **`gorelease-dnd5e-session`** in `compat.yml` gates every release.

The boundary test has one known blind spot, closed by hand: sentinel errors are
not types in signatures, so the composition's sentinels are translated and a
test asserts the inner ones are unreachable through returned errors.

## Verb surface

| Verb | Purpose |
|---|---|
| `StartSession` | Begin a session in a copy of an authored world |
| `Join` / `Exit` | Enter and leave |
| `Move` | Walk a path, cell by cell |
| `Traverse` | Cross a connection |
| `End` | Close through a declared external ending |
| `Answer` | Resolve an open window and resume what was waiting on it |
| `Atlas` / `Status` / `View` / `Story` / `Pending` | Reads |

Every verb is **load → act → save → return**. Nothing is cached between calls,
which is what lets several servers serve one session with no coordination —
pinned by two managers interleaving writes over shared stores.

## Design decisions worth knowing

**Move takes a path, not a destination.** The composition's own `Move` is a
single hop; walking is what makes the cells in between real, which anything
triggered by *entering a cell* depends on. `len(Steps) < len(Path)` is the
honest signal that something stopped the walk — not an error, because the
movement that happened is what was asked for up to the point the world changed.

**Adjacency is delegated to spatial's grid**, never hand-rolled: axial (1,1) is
cube distance 2 but Chebyshev distance 1, and that substitution has shipped as
a real defect here before.

**Write order is chosen for its failure mode.** The encounter is written first
and the session second, because the two half-failures are not symmetric.
World-without-window is a stoppage in which every persisted fact is true;
window-without-world would resume past cells nobody walked — corruption that
looks like progress. `SaveReport` names both halves, and resume re-validates
against the world it actually loads, so even the survivable failure becomes a
clean rejection rather than a wrong walk.

**The converter layer is isolated in `convert.go`** and guarded structurally:
every exported field on an inner type must be carried across or listed with a
reason. A hand-written converter's real failure is silently dropping a field
when an inner type grows one.

## The interrupt spine

A resolution can stop, persist, and resume — a walk today, anything later.

**The walk is a re-enterable phase machine.** It holds nothing across a
suspension: an explicit phase index and the path live in a value that is written
to storage and read back, so a window outlives the process that opened it. A
straight-line loop could only have suspended by parking a goroutine, and a
parked goroutine is a resolution that dies with the process.

**Custody is `play/interrupt`'s**, not this module's. It poses windows,
validates answers, and persists a ledger. This module supplies the phase
discipline and the projection; `interrupt.LedgerData` crosses the boundary as a
stored shape, never `interrupt.Window` or `interrupt.Option` in a signature.

**The freeze is structural.** World-changing verbs open through
`openForChange`, which refuses while a window is open; `Answer` opens through
`openForWrite`, because a frozen session is what it exists to operate on.
Forgetting the freeze requires actively choosing the other opener. Read verbs
stay available — what is frozen is change, not observation — and the rejection
is a typed `*FrozenError` naming the window and its audience, so a blocked
caller is not sent hunting for what the error already knew.

**`Prompt` carries the moment, never the mechanism.** No field names which
checkpoint fired. Clients render what the player sees and branch on
`OptionKind`, so new checkpoint kinds arrive without any client learning a new
reason code — which is the whole reason S5 insists on one suspension shape.

**Enumeration is sorted, not iteration-ordered** (C8): what a checkpoint
enumerates is a function of persisted data, so a reloaded resolution poses its
windows identically.

## Known gaps

Three are the composition's, filed rather than worked around, because
re-deriving any of them here would put a second implementation of a rule
outside the module that owns it:

- **#940** — every beat is addressed to every member, so the fan-out is correct
  to contract and over-broad in game terms.
- **#941** — story tags are coarser than beats, so event kind is currently read
  from the payload: the one place this module interprets a body it does not own,
  and it fails *silently* if that shape changes.
- **#933** — `Members()` reports a member's room but not their cell, forcing a
  full snapshot per walk.

In each case, fixing it where the rule lives fixes this module for free.

## Deferred by design

`CharacterRepository` arrives with entities, not now: `character` lives inside
the large `rulebooks/dnd5e` module, and declaring it early would take a
permanent dependency on combat, conditions, and spells for a repository nothing
calls. Adding a `Config` field later is compatible; removing one a host has
implemented is not.

A session-level `Frozen` blob is deferred for the same reason, and the design
expected one. The frozen resolution rides in the window that suspended it,
because today one checkpoint opens one window and the state belongs to that
window. Shared state across several windows of one resolution — plausible when
reactions land — is an additive field at that point, and inventing it now would
commit a persisted shape before anything could tell us whether it is right.
