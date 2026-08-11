---
name: add a D&D 5e monster
description: Human + agent contract for legal, supported monster content and its current factory/registry/round-trip path
updated: 2026-08-10
---

# Add a D&D 5e monster

This is the recommended first rulebook contribution **when every rule clause can
be composed from behavior that already ships**. It is an operator contract for a
human supervising an agent, not permission to translate an arbitrary stat block
and hope the engine understands it.

Read the [D&D 5e rulebook mental model](../../rulebooks/dnd5e/README.md) and the
[nearest monster guide](../../rulebooks/dnd5e/monster/README.md) first.

## Human + agent contract

### The human provides and approves

- the creature concept and allowed source;
- the source version, URL or bibliographic reference, license, attribution, and
  whether the content was adapted;
- the exact clauses that must be represented (not merely its name and headline
  statistics);
- the decision to stop when a clause needs new mechanics;
- final review that the result is faithful without including closed text.

### The agent must

- inspect current code and tests before describing a capability;
- make a clause-by-clause support inventory and show it to the human;
- use the current paths in this guide, not a proposed package layout;
- never silently omit, approximate, or claim support for an unsupported clause;
- keep a content-only change separate from new mechanic behavior;
- add construction, registry, and applicable serialization/load tests;
- report the source, changes, commands, and limitations in the PR.

If provenance is unclear or any required rule clause is unsupported, the agent
stops. “The source is on a wiki/API” and “a similarly named field exists” are
not sufficient evidence.

## 1. Establish provenance before writing code

Allowed inputs are:

1. **Original content** written by the contributor, with no copied protected
   expression or closed stat block.
2. **Content under a license compatible with this repository's use**, with all
   required attribution and change notices preserved. The primary official
   source for 2014 D&D 5e open content is the
   [System Reference Document 5.1 PDF](https://media.wizards.com/2023/downloads/dnd/SRD_CC_v5.1.pdf),
   released under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

Do **not** copy monster statistics, descriptive text, abilities, or other clauses
from a closed Monster Manual or another unlicensed book. A monster also appearing
in the SRD does not make the book's additional wording or clauses open: use the
SRD version only. Do not treat a third-party API, wiki, video, search result, or
model memory as the legal source unless its own provenance and license are
verified.

For adapted SRD 5.1 content, preserve an attribution record in the PR and add a
short source comment next to the factory when values are derived from it. At a
minimum record:

```text
Source: System Reference Document 5.1 by Wizards of the Coast LLC
Source URL: https://media.wizards.com/2023/downloads/dnd/SRD_CC_v5.1.pdf
License: CC BY 4.0 — https://creativecommons.org/licenses/by/4.0/
Changes: <identify renaming, omitted clauses, or other adaptation>
```

This is a minimum repository guardrail, not legal advice. If the source's terms
cannot be satisfied confidently, use original content or stop for human review.

## 2. Choose the lane clause by clause

Make a table before editing:

| Source clause | Current implementation evidence | Lane | Test |
|---|---|---|---|
| e.g. one melee attack | `monster/actions/melee.go` + loader | content composition | factory + action round trip |
| e.g. “on hit, target is grappled” | no current action/rule path | new mechanic — stop | separately scoped rule test |

### Content-composition lane

A content-only monster can currently compose these verified surfaces:

- canonical identity through `refs.Monsters`;
- `monster.Config`: ID, name, ref, HP/max HP, AC, six ability scores, and an
  optional proficiency bonus (zero defaults to 2);
- walking/flying/swimming/climbing/burrowing speed through `SetSpeed`;
- generic melee, ranged, multiattack, and bite action objects in
  `monster/actions`, subject to the limitations below;
- targeting through `SetTargeting`: closest, lowest HP, or lowest AC;
- immunity and vulnerability trait JSON through current `monstertraits`
  helpers, with load/apply/round-trip tests.

A **simple first fixture** should use one generic melee or ranged attack, no
special trait text, and default closest targeting. This keeps the contribution
about content composition rather than inventing rule semantics.

### Supported data is not the same as supported clause behavior

Be explicit about current limitations:

- `monster.Data` has senses and proficiency fields, but the built-in factory
  surface has no setters for them. It also has `Features` and `Inventory`
  fields that the current base load/`ToData` path does not hydrate/preserve as
  complete runtime behavior. Do not add such clauses in a factory-only PR.
- CR, XP, size, creature type, alignment, languages, saving-throw
  proficiencies, legendary/lair actions, recharge, spellcasting, and encounter
  difficulty are not part of the current built-in factory contract.
- generic melee/ranged actions publish attack events. The top-level encounter
  module resolves their snapshot attack bonus/damage through its resolver;
  the actions are not standalone hit-and-damage functions.
- ranged normal-vs-long range disadvantage is not implemented by the generic
  ranged action; it only gates against long range.
- `BiteConfig.KnockdownDC` serializes, but knockdown-on-hit is not implemented.
- Pack Tactics currently subscribes as a no-op; ally adjacency does not grant
  advantage.
- Undead Fortitude currently rolls but does not change HP and cannot enforce
  its critical-hit exception.
- comments in some existing factories mention Pack Tactics or Undead Fortitude
  without attaching a completed behavior. A comment is not capability evidence.

Do not “support” a source by dropping one of its material clauses. Either use an
original/simple creature whose contract fits the current engine, or take the
new-mechanic lane.

### New-mechanic stop/scope lane

When any required clause is unsupported:

1. stop the monster content edit;
2. open or confirm a separate issue and board item for the mechanic;
3. define the narrowest rule owner, inputs/outputs, event-chain behavior,
   persistence/reload consequence, and a real-path rule test;
4. implement and ship the mechanic in its own reviewed change;
5. return to the content contribution only after the published/current rulebook
   surface can express the whole clause.

Do not put encounter composition into the monster factory and do not make the
host interpret a rule. If encounter selection, population, placement, or CR
budgeting is the missing feature, scope it at the composition layer rather than
pretending it is a stat-block behavior.

## 3. Use the current files

For a creature named `Example Beast` with slug `example-beast`, the current
change set is:

| File | Required edit |
|---|---|
| `rulebooks/dnd5e/refs/monsters.go` | Add the unexported singleton ref and the `refs.Monsters.ExampleBeast()` method. Use lowercase hyphenated ref IDs. |
| `rulebooks/dnd5e/refs/refs_test.go` | Assert the new ref's module, type, and ID (add a monster namespace table if needed). |
| `rulebooks/dnd5e/monster/monsters/example_beast.go` | Add `NewExampleBeast(id string) *monster.Monster`, source comment, supported stats/actions, and speed. |
| `rulebooks/dnd5e/monster/monsters/example_beast_test.go` | Assert ID, ref, name, stats, ability scores, speed, action IDs/types, and any supported attached trait data. |
| `rulebooks/dnd5e/monster/monsters/registry.go` | Map the full `refs.Monsters.ExampleBeast().String()` to the constructor. |
| `rulebooks/dnd5e/monster/monsters/registry_test.go` | Add the ref to the expected registry list; the existing test proves ref → constructor → same ref. |

Do not create `rulebooks/dnd5e/monsters` for this task. That flatter location is
a proposed follow-up only. Do not add a `go.mod`: all of the paths above are
packages inside the existing `rulebooks/dnd5e` module.

Only touch `monster/actions`, `monstertraits`, their refs/loaders, or encounter
code after taking the separately scoped new-mechanic lane.

## 4. Follow the canonical simple fixture

Use the existing one-action bandit as the worked **structural** pattern. Do
not copy its distance literals: the older factories use feet-shaped values such
as `Reach: 5` and `RangeNormal: 80`, while the current generic action configs
and `PerceptionData.Distance` interpret reach/ranges as hex counts. New content
must use current units (5 feet = 1 hex) and pin them in tests.

- factory: [`monster/monsters/bandit.go`](../../rulebooks/dnd5e/monster/monsters/bandit.go)
  (`NewBanditMelee` is the smallest example);
- direct assertions: [`bandit_test.go`](../../rulebooks/dnd5e/monster/monsters/bandit_test.go);
- discoverability: [`registry.go`](../../rulebooks/dnd5e/monster/monsters/registry.go);
- registry correctness: [`registry_test.go`](../../rulebooks/dnd5e/monster/monsters/registry_test.go).

The minimum shape is:

```go
func NewExampleBeast(id string) *monster.Monster {
    m := monster.New(monster.Config{
        ID:            id,
        Name:          "Example Beast",
        Ref:           refs.Monsters.ExampleBeast(),
        HP:            9,
        AC:            12,
        AbilityScores: shared.AbilityScores{
            abilities.STR: 10, abilities.DEX: 12, abilities.CON: 11,
            abilities.INT: 3, abilities.WIS: 10, abilities.CHA: 6,
        },
    })
    m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
        Name: "claw", AttackBonus: 3, DamageDice: "1d4+1",
        Reach: 1, DamageType: damage.Slashing,
    }))
    m.SetSpeed(monster.SpeedData{Walk: 30})
    return m
}
```

The values above are an illustrative original fixture, not a published D&D
stat block. Verify dice notation, range units, and damage vocabulary against
current action tests. In these action configs, reach/ranges are hex counts even
though `SpeedData` is feet; existing older comments and values are not uniformly
reliable, so the new test must pin the intended current behavior.

### Prove the full applicable round trip

A no-trait fixture still needs to prove action hydration:

```text
factory
  → ToData
  → JSON marshal/unmarshal (when persistence is claimed)
  → monster.LoadFromData(ctx, data, bus)
  → actions.LoadMonsterActions(loaded, data.Actions)
  → monstertraits.LoadMonsterConditions(...) when conditions exist
  → loaded.ToData
```

Assert that ID/ref, current and max HP, AC, ability scores, speed, targeting,
action ref/config, and supported trait JSON survive as applicable. Use a real
event bus and clean up the loaded monster. For a trait, assert its actual chain
behavior as well as JSON presence; serialization alone does not prove a rule.

The encounter hydration cascade already performs these loading steps, but a
rulebook content test should keep the factory contract understandable without
requiring a full encounter fixture. If the contribution is also intended for
authored dungeon specs, the registry test is mandatory because dungeonspec
validation calls `monsters.ByRef`.

## 5. Validate

Run from the repository root unless the command changes directory:

```bash
# Focused rulebook packages touched by a simple monster
cd rulebooks/dnd5e
go test -race ./refs ./monster ./monster/actions ./monster/monsters ./monstertraits

# Full owning module
go test -race ./...
golangci-lint run ./...

# Return to repository root for repository gates
cd ../..
git diff --check
make pre-commit
```

`make pre-commit` currently exercises Core and Events; it does not replace the
D&D 5e module commands above. The installed Git hook exits successfully for a
documentation-only commit and checks changed Go packages for code changes.

Also open every changed Markdown link or run the repository-relative link check
used by the PR. There is currently no dedicated Markdown/link target in the
Makefile, so do not report one as if it existed.

## Done checklist

### Provenance

- [ ] Human approved an original or permissively licensed source.
- [ ] PR records source, URL/reference, license, attribution, and adaptations.
- [ ] No closed Monster Manual or unverified aggregator content was copied.

### Capability and scope

- [ ] Every source clause appears in the support inventory.
- [ ] Every included clause has current code and real-path test evidence.
- [ ] Unsupported clauses stopped or moved to separately scoped mechanic work;
      none were silently omitted or described as shipped.
- [ ] Factory contains definition only; rule resolution and encounter
      composition remain with their current owners.

### Current integration

- [ ] Canonical ref and ref test exist.
- [ ] Current `monster/monsters` factory and focused test exist.
- [ ] Registry and expected registry list include the ref and construct the
      matching ref.
- [ ] Factory `ToData` preserves the intended definition.
- [ ] Base load plus action load (and condition load/apply when applicable)
      reconstructs the runtime monster.
- [ ] `ToData` after load preserves applicable ID/ref, mutable HP, stats, speed,
      targeting, actions, and supported traits.
- [ ] An intended authored encounter path can resolve the ref through
      `monsters.ByRef`; no future `dnd5e/monsters` path is assumed.

### Verification

- [ ] Focused package tests pass with the race detector.
- [ ] Full `rulebooks/dnd5e` tests and lint pass.
- [ ] `git diff --check`, Markdown link sanity, and `make pre-commit` pass (or a
      concrete environment blocker is reported).
- [ ] Diff contains no package move, new module, unrelated behavior, or local
      `replace` directive.
