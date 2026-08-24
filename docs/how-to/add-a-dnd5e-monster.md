---
name: add a D&D 5e monster
description: Human + agent contract for legal, supported monster content and its current factory/registry/round-trip path
updated: 2026-08-23
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
   released under CC BY 4.0.

Do **not** copy monster statistics, descriptive text, abilities, or other clauses
from a closed Monster Manual or another unlicensed book. A monster also appearing
in the SRD does not make the book's additional wording or clauses open: use the
SRD version only. Do not treat a third-party API, wiki, video, search result, or
model memory as the legal source unless its own provenance and license are
verified.

### Required SRD 5.1 notice

Do not invent, abbreviate, or paraphrase an SRD attribution. WotC's SRD 5.1
Legal Information instructs users to include this exact statement in their own
work:

> This work includes material taken from the System Reference Document 5.1
> (“SRD 5.1”) by Wizards of the Coast LLC and available at
> https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is
> licensed under the Creative Commons Attribution 4.0 International License
> available at https://creativecommons.org/licenses/by/4.0/legalcode.

For any SRD-derived contribution, preserve that statement verbatim in the
repository-root `NOTICE` file. If `NOTICE` does not exist, the content PR must
create it. A distributed source archive, game build, or other work containing
the SRD-derived material must also carry the exact statement in its included
legal notices/credits; keeping it only in a PR description is not sufficient.
Do not add any other attribution regarding Wizards beyond the statement above,
per the SRD's own instruction.

Record adaptations separately from the verbatim attribution. In the same
`NOTICE`, use a separate “Modifications to SRD-derived material” heading and
identify conversions, renaming, omissions, or other changes. Repeat the relevant
change note next to the factory without adding another Wizards credit, for
example:

```go
// Adaptation: converted movement and attack ranges from feet to hex counts;
// omitted clauses not supported by the current rulebook.
```

Also repeat the modification summary in the PR so reviewers can check fidelity,
but do not rely on the PR as the distributed notice. This repository guidance
is not legal advice. If the source terms or required notice cannot be satisfied
confidently, use original content or stop for human review.

## 2. Inventory every clause

Make a table before editing:

| Source clause | Current implementation evidence | Lane | Test |
|---|---|---|---|
| one attack-roll action | `combat/actions.AttackProfile` | content | factory + round trip |
| on-hit prone, STR save negates | `ConditionApplication` + `SaveGate` + condition registry | content | declaration + resolution |
| multiattack sequence | no sequence profile/machine | new mechanic — stop | separate design |

A content-only monster may use only behavior the current rulebook can express.
Do not silently omit or approximate unsupported clauses.

Current action capabilities:

- melee and ranged attack delivery in feet;
- precomputed attack bonus;
- ordered typed damage pools;
- automatic or save-gated on-hit conditions whose refs the condition registry
  can build;
- no multiattack/sequence compatibility object;
- no save-area, healing, recharge, legendary/lair, or spellcasting profile
  before its machine exists.

## 3. Use the current files

For `Example Beast` / `example-beast`:

| File | Required edit |
|---|---|
| `rulebooks/dnd5e/refs/monsters.go` | canonical monster ref |
| `rulebooks/dnd5e/refs/monster_actions.go` | one content ref per authored action |
| `rulebooks/dnd5e/refs/*_test.go` | full identities and uniqueness |
| `rulebooks/dnd5e/monster/monsters/example_beast.go` | factory with direct definitions |
| `rulebooks/dnd5e/monster/monsters/example_beast_test.go` | stats and complete definitions |
| `rulebooks/dnd5e/monster/monsters/registry.go` | ref-to-constructor entry |
| `rulebooks/dnd5e/monster/monsters/registry_test.go` | registry expectation |

Do not create `monster/actions` or a `go.mod`. The shared contract is the leaf
package `rulebooks/dnd5e/combat/actions` inside the existing D&D module.

## 4. Author definitions directly

```go
mustAddAction(m, combatActions.Definition{
    Ref:  *refs.MonsterActions.ExampleBeastClaw(),
    Name: "claw",
    Attack: &combatActions.AttackProfile{
        Category: combatActions.AttackCategoryWeapon,
        Delivery: combatActions.AttackDelivery{
            Melee: &combatActions.MeleeDelivery{ReachFeet: 5},
        },
        AttackBonus: 3,
        Damage: []damage.Damage{{
            Dice: "1d4", Type: damage.Slashing, FlatBonus: 1,
        }},
    },
})
```

Monster profiles normally omit `Ability` and `Weapon`: stat-block attack and
damage numbers are already precomputed. Use feet for delivery. Keep every
damage pool in authored order.

For an on-hit condition:

```go
OnHit: []combatActions.ConditionApplication{{
    Ref:  *refs.Conditions.Prone(),
    Save: saves.NewSaveGate(abilities.STR, 11),
}},
```

Parameters belong to the referenced condition package. A condition save must
negate on success. Do not put executable functions or lifecycle on a definition.

## 5. Prove storage and behavior

At minimum:

```go
before := NewExampleBeast("example-1").ToData()
raw, err := json.Marshal(before)
require.NoError(t, err)

var persisted monster.Data
require.NoError(t, json.Unmarshal(raw, &persisted))
loaded, err := monster.Load(context.Background(), &persisted)
require.NoError(t, err)
require.Equal(t, before, loaded.ToData())
```

Also mutate the value returned by `loaded.Actions()` and prove a second read and
`ToData()` are unchanged. Assert every action's full ref, name, category,
delivery, attack bonus, ordered damage, and condition declarations.

When traits are present, use:

```go
loaded, err := monstertraits.LoadMonster(ctx, &persisted)
require.NoError(t, err)
require.NoError(t, monstertraits.AttachMonster(ctx, loaded, bus, roller))
```

Test actual chain behavior for each trait, not JSON presence alone.

## 6. Validate

```bash
cd rulebooks/dnd5e
go test -race ./refs ./monster ./monster/monsters ./monstertraits
golangci-lint run ./refs/... ./monster/... ./monstertraits/...
go test -race ./...
golangci-lint run ./...
cd ../..
git diff --check
```

## Done checklist

- [ ] Provenance and required notices are approved.
- [ ] Every source clause is inventoried.
- [ ] Unsupported mechanics stopped for separate design.
- [ ] Monster and each authored action have canonical content refs.
- [ ] Factory authors complete `actions.Definition` literals directly.
- [ ] Distances are feet and damage pools preserve order.
- [ ] No executable action object, loader config, adapter, or multiattack placeholder exists.
- [ ] JSON/load/ToData and deep-clone behavior are tested.
- [ ] Applicable condition/trait behavior is tested on the real path.
- [ ] Registry constructs the matching monster ref.
- [ ] Focused and full D&D tests/lint pass.
