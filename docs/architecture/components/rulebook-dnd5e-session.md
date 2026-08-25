---
name: rulebooks/dnd5e/session module
description: The game server's single integration surface — ID-based verbs over repositories, with compiled combat offers regenerated at execution and inner runtime types held off the host boundary
updated: 2026-08-25
confidence: high — selector, boundary, blocker-matrix, no-mutation, and full module gates are executable tests
---

# rulebooks/dnd5e/session module

**Path:** `rulebooks/dnd5e/session/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session`
**Grade:** A-

The host's one point of contact with live D&D 5e play. A game server supplies
key-value repositories for sessions, encounters, and characters, plus event,
dice, and unplayed-turn capabilities. It then calls verbs with IDs and stores
data; it never holds a runtime encounter, character, clock, resolution, or
compiled offer.

## Layer and boundary

```
rpg-api ──implements──▶ SessionRepository / EncounterRepository / CharacterRepository
   └────────calls──────▶ session.Manager
                              │
                    encounter composition
                              │
              ┌───────┬───────┼────────┐
            clock   intel   record   spatial
                              │
                    D&D 5e resolution/rules
```

The encounter composition is the world: geometry, placement, perception,
story, clocks, and endings. Session is the host seam and courier. D&D rules
stay in rulebook packages; session loads their data, invokes them, saves dirty
aggregates, and projects seam-owned values.

S2 is mechanical. `boundary_test.go` parses every exported declaration and
rejects inner toolkit runtime types. The explicit exceptions are four stable
or persisted values: `spatial.Position`, `character.Data`,
`encounter.EncounterData`, and `monster.Data`; each has a written reason.
`sentinels_test.go` closes the AST test's error-channel blind spot by proving
reachable inner sentinels do not escape through `errors.Is`.

Every write verb remains **load → validate/select → act → save → return**. The
manager caches no session state between calls (S1).

## Current combat offer contract

`Afford(session, member)` is the server-authored action surface. On an active
turn it returns exactly one current compiled variant for each supported verb:

| Verb | Selector target | Compiled data | Final execution input |
|---|---|---|---|
| Attack | member | complete priced `actions.Definition`, matching resolution cost/readied payer, full-ref `AttackRef`, one preflight row per live candidate, and one strictly preflighted raw resolution cast | echoed ID + selected member |
| Move | path | readied actor sheet and remaining movement; path price is unknown until execution | echoed ID + path |
| EndTurn | none | clock-derived selector only | echoed ID |

The first production wave has no magic and no speculative generic action
executor. Dodge, Dash, Disengage, features, items, reactions, spells, spell
slots, concentration, and magical target kinds do not become clickable offers
through this contract.

A compiled Attack definition includes a defensive clone of the **actual**
`SpendProfile` before selector generation. The matching resolution cost holds a
second clone. Changing the price therefore changes the selector, and execution
cannot select one costless weapon definition and independently compile another
priced definition. The collision guard compares RFC 8785-canonical variant
bytes, so equivalent embedded JSON object order is recurrence rather than a
false collision.

Attack candidates are every current live sight holding (`CurrentVia` non-empty)
except the actor, sorted by member ID. Stale memories and undisclosed members
are absent. Out-of-reach candidates remain present with their target-specific
shortfall. Missing position for a live candidate fails closed. Projection
copies each candidate `Why`, so caller mutation cannot alter internal preflight
state. The same private target-preflight function is used by Afford and
regenerated Attack execution; an injected-refusal regression proves they move
together. Offer compilation also gathers every roster member's raw sheet once
and validates the cast through the same public strict loaders/attach APIs
resolution uses. A missing/unreadable candidate keeps its row with `UNREADABLE`;
an unreadable non-target dependency disables Attack globally while preserving
other candidate reach facts. Selected execution reuses that exact raw cast and
never refetches participant data. Normal target/reach changes therefore refuse
as `ErrStaleDeclaration`; `ErrOutOfReach` remains final defensive resolution
validation.

`AttackRef.Ref` is the complete catalog identity (`dnd5e:weapons:longsword`),
not a bare ID. The same helper projects Declaration, AttackOutput, and the
Struck/Missed beat.

## Blocker matrix

Blockers are per verb rather than an incomplete declaration list:

| Refusal | Attack | Move | EndTurn |
|---|---|---|---|
| not active | blocked before sheet load | blocked before sheet load | blocked |
| downed | blocked | blocked | clock-valid EndTurn remains compiled |
| unreadable character | `UNREADABLE`, empty ID | `UNREADABLE`, empty ID | unaffected |
| unreadable Attack | `UNREADABLE`, empty ID | independently compiled | unaffected |
| unreadable target/cast participant | compiled but unavailable; candidate/global `UNREADABLE`, rows retained | independently compiled | unaffected |
| no budget / no available target | compiled but unavailable, non-empty ID | independently compiled | unaffected |

Every blocker keeps its fixed target kind and has empty ID, absent AttackRef,
and empty candidates. Attack and turn-clock Move strictly load the actor once;
`combat.IsDown`, declaration compilation, pricing, and execution all use that
same sheet snapshot. World-clock Move keeps its independent standing gate.
EndTurn has no character-sheet, standing, or economy gate.

## Selector trust boundary

Declaration IDs are deterministic opaque selectors, not authorization tokens
or idempotency keys. They hash the RFC 8785 canonical selector document with
full SHA-256/base64url under the `v1.` prefix. Attack's variant is its validated,
priced complete definition; Move and EndTurn use sealed variant strings.
Collision detection fails closed.

The mutating verb reloads current state and regenerates only its relevant
compiled offer before mutation. Attack rebuilds its priced definition, targets,
and cast; turn-clock Move rebuilds only Move from the shared actor
blocker/readying path and does not read Attack price, target view, or resolution
participants; EndTurn remains its direct clock-only offer.

- Attack and EndTurn require a non-empty ID.
- Turn-clock Move requires its current Move ID.
- World-clock Move requires an empty ID because world Afford is an empty list.
- A non-empty turn Move ID that arrives after transition to world is stale and
  cannot become a free move.
- Unknown, mismatched, now-unavailable, or target-invalid selection returns
  `ErrStaleDeclaration` before dice, movement, writes, or story.
- A repository returning a `SessionData.ID` different from the requested key is
  `ErrBadRepository`; selector scope always uses the requested canonical key.
- Omission returns `ErrNoDeclarationID`.

The intentional pre-selection precedence is explicit: basic identity/path
validation, `NotYourTurn`, and `Downed` remain the real earlier verb refusals.
EndTurn also preserves `NotInFight`/`NotYourTurn` before selector matching.
Resolution retains final defensive validation after selection.

## Persistence and failure ordering

Attack passes the selected readied payer and priced definition into resolution.
Payment can dirty the attacker even on a miss; damage can dirty the target.
Dirty sheets are saved before the resulting beat is recorded because standing
consults those stores. `SaveError` names every durable aggregate when a later
world save fails, preventing unsafe retries.

Move pays the complete selected path before entering its first cell and saves
the readied walker only after the walk succeeds. A path-level overrun remains
`ErrCannotAfford`; an unavailable Move declaration is stale earlier. World Move
never loads or mutates the economy.

## Known/deferred minors

- Turn speed is seeded from base sheet speed; condition-adjusted haste/slow
  belongs at a rule-capable speed projection.
- `ErrBadCost` is a defensive programmer-facing resolution translation while
  some offer-build consistency failures (for example a live candidate missing
  position) remain wrapped internal errors. Aligning those sentinel shapes is
  deferred; selector execution does not require a new public distinction.

## Verification

```sh
cd rulebooks/dnd5e/session
go test ./... -count=1
golangci-lint run ./...
../../../scripts/verify.sh rulebooks/dnd5e/session
```

Key regressions cover priced selector identity, selector recurrence/collision,
fixed input shapes, stale no-mutation behavior, shared target preflight,
per-verb blockers, world/turn Move transition, sheet-free EndTurn, full-ref
Attack identity, S2 signatures, and inner-sentinel non-leakage.
