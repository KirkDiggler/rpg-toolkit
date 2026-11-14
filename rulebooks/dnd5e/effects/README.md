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

[`conditions.RagingBenefits`](../conditions/raging_benefits.go) declares four typed
benefit parts. Native [`Raging`](../conditions/raging_definition.go) compiles that
same declaration source into its existing recipe's outputs:

| Output | Configuration |
|---|---|
| Damage bonus | Fixed amount or live Barbarian-level scaling; melee weapon attacks using Strength |
| Resistance | Bludgeoning, piercing and slashing |
| Advantage | Strength saving throws |
| Advantage | Strength ability checks |

The named recipe separately supplies its activity trackers, activation grace,
duration and endings. Those rules are not hidden inside the benefit bundle or its
handlers. Another composition can reuse the four benefit parts—or author one of
them alone—while selecting concentration instead of native Rage upkeep.

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
| `add_to_roll` | Query-only | Yes | One dice amount, roll domains, explicit polarity and contribution-selection policy |
| `concentration` | Subscribed | Yes | Positive applied damage requests a CON follow-up; zero HP ends maintenance |
| `duration` | No separate output | Yes | Positive holder-turn-end count and explicit activation-grace choice |
| `round_duration` | No separate output | Yes | Positive anchored-round maximum, explicit expiry/reset reasons; missing events do not extend it |
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

## Compile reusable definitions before choosing placement

`Compile` accepts a typed leaf, a Go `Composition`, or resolved `DefinitionData`.
The canonical data contains exactly one EffectSpec or CompositionData; compositions
have ordered named members. No condition, catalogue, holder, application or terminal
scope is required just to compile content. An explicit empty composition is empty;
an absent definition is an error.

```go
compiled, err := effects.Compile(&effects.CompileInput{
    Definition: effects.Resistance{Types: []damage.Type{damage.Slashing}},
})
// Handle err before choosing placement.
placed, err := effects.Place(&effects.PlaceInput{
    Definition: compiled.Definition,
    Mounts: []effects.Mount{{Key: "ward", OnTerminal: generic.EndPart}},
})
```

The root effect has an empty relative definition path; the mount supplies its local
key. Nested compositions keep depth-first declared paths. Sibling keys are unique
and exclude `/`; acyclic subtree reuse is allowed and copied, while cycles refuse.
Compiled Data/Leaves views own their nested configuration. Group labels such as
`maintenance` have no implicit lifetime meaning.

Placement covers every leaf exactly once with explicit scope. Missing, overlapping
or invalid paths/scopes refuse without a partial result. Kind-owned intrinsic
validation is separate from placement validation, so neither fabricates facts for
the other. `CompileParts` remains the first-cut leaf-and-scope adapter over those
same validators while condition/application consumers move to definition placement.

## Optional catalogue and explicit derivation

`DefineEntry` projects a typed definition into ordinary authored entry data through
that same compiler. `CatalogueData` is the strict configuration document and
`BuildCatalogue` validates its complete entry set, including unused definitions,
before returning an immutable catalogue. No global registration or network lookup
occurs. `DescribeCatalogue` returns owned entry/member metadata in declared order.

`Resolve` accepts inline content without a catalogue, or an authored `use` naming
a definition ref and a separate member-path array. Empty path selects the root.
Definition refs use the `definitions` namespace, not a running condition identity;
Go can discover built-ins through `refs.Definitions`. The wire spelling is one
string ref—historical object-ref fallback is not part of this format.

A use can include explicit whole-member replacements. Every target must already
exist; root, duplicate and overlapping paths refuse. Replacement keeps local
key/order/metadata and replaces the complete configured value, never merges
params/guards/amounts or mutates an instance. `Derive` exposes the same operation
for already-resolved canonical values. Source catalogues and other consumers stay
unchanged. Forward references and acyclic same-entry member reuse are valid;
actual cycles and missing paths are errors, not partial definitions.

See the executable [reference](reference_test.go) and [derivation](derive_test.go)
contracts and their [fixed-bonus configuration](testdata/definitions/fixed-rage-use.json).
A resolved value contains no reference or live catalogue handle when handed to
compilation/binding.

## Typed construction before runtime binding

A Go author supplies values, then explicitly compiles them. No participant IDs,
handwritten JSON, subscriptions or panic-based initializer are needed:

```go
parts, err := effects.CompileParts(&effects.CompilePartsInput{
    Parts: []effects.PartSpec{
        {
            Key: "roll",
            Effect: effects.AddToRoll{
                Amount: effects.SignedDice{Notation: "1d4", Polarity: effects.Positive},
                Rolls: []contributions.RollKind{contributions.RollKindAttack, contributions.RollKindSavingThrow},
                Selection: generic.SelectionRule{Mode: generic.OldestApplicable, Group: "bless"},
            },
            OnTerminal: generic.EndPart,
        },
        {Key: "rest", Effect: effects.EndOn{Beat: effects.BeatRest, Reason: "rest"}, OnTerminal: generic.EndAttachment},
    },
})
```

Handle `err` at construction. `parts.Parts` contains owned canonical declarations,
validated through the same kind table as data authoring. Omitting EndOn omits rest
expiry: AddToRoll has no hidden lifetime. Native [Bless](../conditions/blessed.go)
and [Bane](../conditions/baned.go) use this path; Bane chooses Negative polarity and
its independent group. Their descriptor factories return errors through preparation,
creation and loading rather than compiling fallible content at package initialization.

Typed DamageBonus, Resistance, Advantage, Concentration, Duration, CombatActivity,
RoundDuration and UntilEmpty values
select the existing meanings as well. `CompileOutputs` projects only payload
parts through those same builders for a consumer that already owns a recipe. It
refuses maintenance rather than dropping it, and requires no fake holder identity.

## Upkeep is not one shared counter

CombatActivity owns attack/positive-applied-damage flags and optional first-end
grace. RoundDuration owns the first positive round anchor and derived elapsed
progress; it does not count received events. Missing or negative rounds do not
anchor/reset it. An active anchored duration expires at its configured elapsed
boundary; a positive backwards round produces its explicit reset reason.

Place activity before the anchored duration when inactivity must win a simultaneous
end. The existing assembly gates sibling reactions on the first accepted terminal
request while payload outputs remain observable. Both parts support local attachment
or shared application ending; placement is explicit. A one-round policy ends on its
first positive round observation, not a fabricated round-zero boundary.

RoundDurationInfo and DescribeMaintenance keep anchored progress separate from
holder-turn-end counters. These definitions use the existing machine's round
operations and the same native occurrence vocabulary, not another clock.

## Authoring and binding are rulebook APIs

`effects.PartTemplate` is unbound meaning: kind, params, and supported When/Amount
values. `ValidatePartTemplate` reports the kind-owned class and capabilities
without fake participant IDs. The [canonical library](../conditions/testdata/effects/library.json)
reuses leaves in ordered benefit sets; the ring and scroll select the same four
benefits but different maintenance.

A Go-authored application uses `conditions.CompileComposition` with a
`CompositionSpec{Name, Coordinator, Benefits, Maintenance}`. Benefits and maintenance
are independent typed `PartSpec` lists. It returns the existing immutable catalogue
containing that one definition ID; role binding and delivery use the same path as
the JSON library. Both compilers converge on the same expanded-definition validator,
not parallel runtimes. Changing maintenance or mutating the input cannot rewrite a
compiled or active grant. [The typed raging-ring example](../conditions/typed_composition_example_test.go)
shows the whole definition, then uses the existing source/host fixture to exercise
payment, idle turns, reload and concentration cleanup. It is an executable rulebook
consumer, not native inventory acquisition or a live save-window roll.

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

`DescribeRoll` returns unresolved signed dice and explicit selection policy. It does
not roll them or subscribe another roll handler. `AddToRoll` stores dice only in
Amount; params carry polarity, supported attack/save domains and SelectionRule.
The neutral selector chooses already-applicable contributions in persisted order:
AllApplicable requires no group, and OldestApplicable chooses one per group. An
excluded contribution remains held under its own lifetime. The recipient's roll
path evaluates only selected dice once; information uses the same decisions.
Definition validation shares the native contribution-dice parser, so mixed pools
or fixed modifiers cannot pass construction only to fail at resolution.

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
| Canonical values, projection, compilation and placement | [definition_data.go](definition_data.go), [definition_projection.go](definition_projection.go), [definition_compile.go](definition_compile.go), [placement.go](placement.go) |
| Intrinsic meaning and scope capabilities | [effect_validation.go](effect_validation.go), [kinds.go](kinds.go) |
| Catalogue, reference resolution and owned derivation | [catalogue.go](catalogue.go), [catalogue_data.go](catalogue_data.go), [reference_resolve.go](reference_resolve.go), [derive.go](derive.go) |
| Typed leaf values / first-cut adapters | [definitions.go](definitions.go), [compile_parts.go](compile_parts.go), [compile_outputs.go](compile_outputs.go) |
| Independent declarations and binding | [parts.go](parts.go), [part_validation.go](part_validation.go) |
| Payload part example | [resistance_part.go](resistance_part.go) |
| Maintenance parts | [concentration_part.go](concentration_part.go), [duration_part.go](duration_part.go), [round_duration.go](round_duration.go), [activity_part.go](activity_part.go), [end_parts.go](end_parts.go) |
| Query-only roll contribution | [add_to_roll.go](add_to_roll.go) |
| Shared Rage benefits / native lifecycle | [raging_benefits.go](../conditions/raging_benefits.go), [raging_definition.go](../conditions/raging_definition.go) |
| Code-authored application composition | [composition.go](../conditions/composition.go) |
| Linked ownership and delivery | [application.go](../conditions/application.go), [application_binding.go](../conditions/application_binding.go) |

Run from the owning module:

```sh
cd rulebooks/dnd5e
go test ./effects ./conditions ./contributions
go test -count=1 -v ./conditions -run 'TestAuthoredEffectsCookbookSuite|Example_dataAuthoredEffects'
```
