# D&D 5e effects

**The generic engine supplies the grammar; this package supplies D&D's words.**
[`mechanics/effects`](../../../mechanics/effects/README.md) composes state machines,
outputs, independent parts and linked lifetimes. This package gives those tools
meanings such as damage bonus, resistance, advantage and concentration. It does
not own named conditions, targeting, item charges, inventory or storage.

Start with [the package contract](doc.go), then follow Rage below. The
[rulebook-local cookbook](../conditions/authored_example_test.go)
shows the public API in use; implementation/release evidence lives in
[the composition record](../../../docs/ideas/data-authored-effects/implementation.md).

## Three things that are easy to confuse

| Thing | What it describes | Owner |
|---|---|---|
| Output | A contribution while a machine state is active: kind, applicability guard, amount and parameters | Generic `Output`; this package interprets its kind |
| Part | An independently running recipe, with its own state and terminal scope | Generic assembly; this package builds D&D parts through `BindPart` |
| Application | A coordinator and concrete recipients sharing a lifetime, with exact ownership and provenance | [`conditions`](../conditions/application.go) |

An output does not need its own lifetime. Several outputs can be active in the
same state. Independent maintenance parts do need independent state: duration
counting must not compete with concentration for the first matching transition.

## Rage is a consumer, not a primitive

[`conditions/raging_definition.go`](../conditions/raging_definition.go) constructs
four outputs:

| Output | Configuration |
|---|---|
| Damage bonus | Fixed amount or live Barbarian-level scaling; melee weapon attacks using Strength |
| Resistance | Bludgeoning, piercing and slashing |
| Advantage | Strength saving throws |
| Advantage | Strength ability checks |

The named recipe also supplies its activity trackers, activation grace, duration
and endings. Those rules are not hidden inside the damage or resistance handlers.

For the damage output:

1. `Bind` supplies a vocabulary bound to immutable origin and display name.
2. The composer validates the output and attaches it while its state is active.
3. `attachDamage` listens to the outgoing damage chain for the output's holder.
4. The shared assessment evaluates the action facts, guard and amount.
5. If applicable, the handler contributes a damage component with its source.
6. `Assess` uses that same evaluation to explain the contribution.

`attachResistance` follows the incoming damage chain instead. It contributes
resistance multipliers for the configured matching damage types, leaving the
actual damage fold to combat. Neither handler branches on the name Raging.

The useful boundary is **a mechanical meaning in code, a selection and
configuration in data**. A new damage type selection does not need a new
resistance handler. A genuinely new mechanical meaning does need an implementation
and tests.

## The vocabulary

[`kinds.go`](kinds.go) is the single kind table for validation, execution and
information. These are its current surfaces, not a promise that every output is
also a standalone part:

| Kind | Output | Independent part | Configuration / responsibility |
|---|---|---|---|
| `damage_bonus` | Subscribed | Yes | Fixed/class-scaled scalar amount; optional frame-fact guard |
| `resistance` | Subscribed | Yes | Nonempty, valid, unique damage types |
| `advantage` | Subscribed | Yes | Saves/checks, or attacks explicitly by/against the holder; optional supported frame-fact guard |
| `roll_modifier` | Query-only | Yes | Unresolved dice, roll domains, selection group and sign |
| `concentration` | Subscribed | Yes | Positive applied damage requests a CON follow-up; zero HP ends maintenance |
| `duration` | No separate output | Yes | Positive holder-turn-end count and explicit activation-grace choice |
| `combat_activity` | No separate output | Yes | Holder attack/positive-applied-damage activity; independent first-end grace |
| `end_on` | No separate output | Yes | Rest, combat-end, turn-start or dropped-to-zero beat, reason and terminal scope |
| `until_empty` | No separate output | Yes | Actual last-member loss, not empty initial delivery |

Attack advantage uses `on: attack` and an explicit `side: by_holder` or
`side: against_holder`. Against-holder execution requires the local attachment's
exact identity and generation plus matching configured facts in the action frame.
It does not swap actor and target or query another creature's live sheet. Native
Reckless Attack acquisition/activation remains its existing condition/feature.

Kinds own their chain and stage. Authors do not select arbitrary subscriptions
or chain positions through kind parameters. Unknown kinds, unsupported parameters
and invalid combinations are refused rather than ignored.

## Resistance without the rest of Rage

This is a current resistance part declaration, not a complete item definition:

```json
{
  "key": "physical-ward",
  "kind": "resistance",
  "params": {"types": ["bludgeoning", "piercing", "slashing"]},
  "on_terminal": "part"
}
```

`BindPart` turns it into a local recipe whose output uses the **same resistance
meaning** as Rage. It does not grant Rage's damage bonus, advantages or activity
upkeep. The payload is active until its owning attachment is removed; it has no
self-expiry transition.

An application can put this payload on one creature and concentration on another.
The recipient owns the resistance behavior; the coordinator owns the shared
lifetime. Ending the application removes its exact linked attachment, not every
condition whose name happens to match. Duration and rest policy are separate
parts, not implicit concentration behavior.

See [`maintenance_composition_test.go`](../conditions/maintenance_composition_test.go)
for resistance with and without concentration and preservation of independent
Raging. This is toolkit composition proof, not a native item-acquisition API.

## Authoring and binding are rulebook APIs

`effects.PartTemplate` is unbound meaning: kind, params, and supported When/Amount
values. `ValidatePartTemplate` reports the kind-owned class and capabilities
without fake participant IDs. The [canonical library](../conditions/testdata/effects/library.json)
reuses leaves in ordered benefit sets; the ring and scroll select the same four
benefits but different maintenance.

The public `conditions` path is:

1. `CompileDefinitions` validates the entire library, including unused entries.
2. `ValidateDefinitionReference` checks a source's selection without world state.
3. `BindDefinition` supplies user, recipient, invocation and immutable origin,
   producing copied ApplicationSpec data.
4. `PrepareApplication` checks the actual participant universe and exact replacement
   authority without paying or delivering.
5. The source pays; the host creates the owner, delivers/finalizes recipients and
   checks the existing application boundary.

Active effects retain their resolved declarations on reload. Changing the authoring
library cannot rewrite an already-paid grant. Tests live beside the implementation
in external `conditions_test`, with a small source budget—not an item engine or
an examples/items dependency.

## Information is not a second rules engine

`Assess` and attachment execution share kind meanings, guards and amounts.
A known mismatch is not applicable. Missing required facts are `Depends` for
information and an explicit refusal when execution cannot answer. An absent query
can describe an effect without claiming its benefit applies.

`DescribeRoll` returns unresolved signed dice and selection facts. It does not
roll them or subscribe another roll handler. The recipient's roll path selects
contributions and evaluates them once.

`ProjectHeldEffects` captures configured target facts as detached values.
`AssessTargetHeldEffects` reads those values, not live target behavior; absent
detail is unknown and an explicit empty list is known-none. Actual execution
checks the applying output against authoritative captured identity/configuration.
`Bound.Definition()` supplies the complete authored output from its neutral owner,
while CurrentOutputs still controls availability during end reporting.

`InspectPart` and `DescribeMaintenance` expose maintenance facts without asking
callers to interpret private bags. A UI should render those facts, not recalculate
countdowns or infer concentration from a condition name.

## Identity, payment and lifetime stay separate

- **Holder:** the creature on which this local behavior acts.
- **Origin:** immutable provenance supplied by the caller; not a stacking address.
- **Application and attachment address:** exact ownership/replacement identities,
  supplied by conditions rather than inferred by a kind.
- **Payment:** an activation/source concern; no effect handler spends item charges
  or spell slots.
- **Saved state:** the condition owns its codec and snapshots. These vocabulary
  functions do not perform repository I/O.

An item's user, the recipient, the maintaining creature and the originating item
are not interchangeable identities.

## Source map

| Concern | Read |
|---|---|
| Public vocabulary and assessment input | [types.go](types.go), [query.go](query.go) |
| Kind registration and shared evaluation | [kinds.go](kinds.go) |
| Typed event-chain attachment | [attach.go](attach.go) |
| Fact/beat binding and amounts | [vocabulary.go](vocabulary.go), [amounts.go](amounts.go) |
| Independent declarations and binding | [parts.go](parts.go), [part_validation.go](part_validation.go) |
| Payload part example | [resistance_part.go](resistance_part.go) |
| Maintenance parts | [concentration_part.go](concentration_part.go), [duration_part.go](duration_part.go), [end_parts.go](end_parts.go) |
| Query-only roll contribution | [roll_modifier.go](roll_modifier.go) |
| Named Rage composition | [raging_definition.go](../conditions/raging_definition.go) |
| Linked ownership and delivery | [application.go](../conditions/application.go), [application_binding.go](../conditions/application_binding.go) |

Run from the owning module:

```sh
cd rulebooks/dnd5e
go test ./effects ./conditions ./contributions
go test -count=1 -v ./conditions -run 'TestAuthoredEffectsCookbookSuite|Example_dataAuthoredEffects'
```
