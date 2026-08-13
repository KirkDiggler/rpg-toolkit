---
name: rulebooks/dnd5e/session module
description: The game server's single integration surface — a manager composing the encounter behind ports, with inner types held off the boundary so the insides can be replaced without the host changing
updated: 2026-08-12
confidence: high — written alongside the implementation (#938 / PR #942); every claim below is pinned by a test in the module
---

# rulebooks/dnd5e/session module

**Path:** `rulebooks/dnd5e/session/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session`
**Grade:** A-

The host's one point of contact with the toolkit. A game server implements two
repositories and thereafter holds no domain object: it names things and calls
verbs.

## Where it sits

```
rpg-api ──implements──▶ ports (Get/Save)
   └────────calls──────▶ session.Manager
                              │
                         encounter (the world)
                              │
              ┌───────┬───────┼────────┐
            clock   intel   record   spatial
```

The encounter composition is the **world** — geometry, placement, perception,
story, endings. This module is the **table**: entity roster, event fan-out,
resolution, and (from wave 2) interrupt custody. It holds wiring and lifetime,
never rules.

## Why it exists

Not to add capability. To make the toolkit's insides replaceable without the
host changing a line, so waves of internal rework cost rpg-api a version bump
rather than a migration.

That is a falsifiable claim, and it is gated two ways:

- **`boundary_test.go`** parses the package's own source and fails if any
  toolkit type is reachable from an exported declaration. Verified against a
  real leak, not a synthetic one; carries a meta-pin so it cannot silently stop
  biting. Its allow list has two entries, each with a written reason.
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
| `Atlas` / `Status` / `View` / `Story` | Reads |

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

**Write order is chosen for its failure mode.** World first, then the session
pointing at it, so a partial failure leaves a collectable orphan rather than a
session pointing at nothing.

**The converter layer is isolated in `convert.go`** and guarded structurally:
every exported field on an inner type must be carried across or listed with a
reason. A hand-written converter's real failure is silently dropping a field
when an inner type grows one.

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
the large `rulebooks/dnd5e` module, and declaring the port early would take a
permanent dependency on combat, conditions, and spells for a port nothing calls.
Adding a `Config` field later is compatible; removing one a host has implemented
is not.
