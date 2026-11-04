---
name: mechanics modules
description: The live generic resources module and its boundary with rulebook recovery rules
updated: 2026-11-03
---

# Mechanics

The current module in this layer is
[`mechanics/resources`](../../../mechanics/resources/). The
[checked module map](../../../knowledge/map.md) is the module census.

## Resources

`Resource` holds an ID, a current balance and a maximum. It supports spending,
restoring and setting the balance. `Counter` holds a count with an optional
limit. `Pool` groups resources and counters by ID. The module imports the
standard library only in production; it has no event bus or rulebook dependency.

It does not decide what a balance means, what an action costs, or when a rest
restores it. Those are the rulebook's rules.

## The live consumer

[`combat.RecoverableResource`](../../../rulebooks/dnd5e/combat/recoverable_resource.go)
wraps a generic resource and attaches the rulebook's recovery behavior to rest
events. Characters persist these resources; class features and spellcasting
spend and recover them through the existing rulebook paths.

The API consumes the session/rulebook modules and receives resources as a
transitive dependency. Absence of a direct API import is not evidence that a
module is unused.

## Effects and the state machines

[`examples/effects`](../../../examples/effects/) is a prototype composer with
its own event-driven machine. [`mind/machine`](../../../mind/machine/) is the
separate bus-free machine used by encounter scheduling. They remain separate
implementations; a possible shared base is not a current dependency.

The prototype is not a published replacement for the retired legacy mechanics
APIs. Promotion belongs to the
[composable-conditions story](https://github.com/KirkDiggler/rpg-project/issues/703).
D&D's active conditions, features and proficiencies remain in the rulebook.

## Verification

```sh
cd mechanics/resources
go test -race ./...
```

Run `scripts/check-knowledge-map.sh` from the repository root when module
membership changes. Retirement evidence and historical context are linked from
[rpg-project#853](https://github.com/KirkDiggler/rpg-project/issues/853); earlier
ADRs and journey records are not current import guidance.
