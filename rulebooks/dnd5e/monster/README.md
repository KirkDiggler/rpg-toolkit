# D&D 5e monsters: current guide

This directory owns monster sheets and persisted content inside the single
`rulebooks/dnd5e` module. For active turn behavior, start with the active
[`encounter`](../encounter/README.md) composition; the retired `Monster.TakeTurn`
and runtime action-object path no longer exist.

## Current packages

| Package | Responsibility |
|---|---|
| `monster` | Runtime sheet, `Data`, direct shared action-definition storage, pure `Load`, `SheetKeeper`, and `ToData` |
| `monster/monsters` | Built-in stat-block factories and the canonical ref registry |
| `monstertraits` | Condition-style trait loading and attachment |
| `combat/actions` | Shared inert `Definition` and typed attack profiles used by every producer |
| `refs` | Canonical monster, monster-action-content, and trait refs |

There is no `monster/actions` package. Actions are definitions, not runtime
objects with `Activate`, subscriptions, or self-managed lifecycle.

## Definition, behavior, and resolution

These are separate responsibilities:

```text
monster factory                 encounter behavior          resolution
Definition literals + refs  -> TurnIntent by action ref -> NewAction(profile arm)
```

A factory authors complete `combat/actions.Definition` values directly. The
active encounter composition carries only opaque identity, name, maximum range,
and delivery kind in `ActionView`. Session selects a definition by exact content
ref. Resolution dispatches by the populated profile arm and never switches on a
monster-action ref.

Monster stat-block definitions use precomputed attack bonus and damage numbers,
so optional character evidence such as `AbilityContribution` and
`WeaponContext` is normally nil. Distances are authored in feet. Preserve
ordered typed damage pools.

Multiattack is intentionally unsupported until a sequence profile and machine
exist. Factories retain their component attacks; they do not carry a compatibility
multiattack object.

## Conditions and traits

On-hit conditions are declared as `actions.ConditionApplication` values. The
condition registry creates behavior from the ref and opaque parameters;
resolution handles optional saves and publishes the prepared behavior. The
condition owns subscriptions and lifecycle.

Factory-time traits such as immunity and vulnerability remain persisted JSON
loaded by `monstertraits`. Loading persisted traits uses `AddLoadedCondition`
and does not dirty an unchanged sheet. A live `ConditionAppliedEvent` reaches
the monster's `SheetKeeper`, applies the condition, appends it, and marks the
sheet dirty.

## Construction and loading

A factory returns a ready-to-serialize sheet:

```text
monster/monsters constructor -> *monster.Monster -> ToData / JSON
```

Pure reload validates and deep-clones definitions directly:

```go
m, err := monster.Load(ctx, data)
```

The composition entry point remains convenient when traits also need attaching:

```go
m, err := monstertraits.LoadMonster(ctx, data)
err = monstertraits.AttachMonster(ctx, m, bus, roller)
```

`Monster.Actions()` and `ToData().Actions` return deep clones. Mutating their
refs, damage properties, condition parameters, or save abilities cannot alter
the stored sheet.

## Adding content

Use [`monsters/bandit.go`](monsters/bandit.go) for a simple melee/ranged
structural example, [`monsters/wolf.go`](monsters/wolf.go) for a save-gated
condition, and [`monsters/skeleton.go`](monsters/skeleton.go) for distinct melee
and ranged definitions. Each authored attack needs its own content ref under
`refs.MonsterActions`; implementation refs such as generic `melee`, `bite`, or
`ranged` are not valid identities.

Follow the full [monster contribution guide](../../../docs/how-to/add-a-dnd5e-monster.md)
for provenance, capability inventory, files, and verification commands.
