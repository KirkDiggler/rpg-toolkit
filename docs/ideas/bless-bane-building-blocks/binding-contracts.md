# Native binding and state — implementation mapping

This maps accepted R8–R10 onto the inspected runtime. It does not introduce new Rage eligibility or gameplay rules. The [native evidence](native-maintenance-evidence.md) is the baseline; [the replacement plan](effect-first-plan.md) supplies executable task contracts. Concrete names here are lead implementation choices under the agreed boundaries, not additional operator rulings.

## 1. One error-returning compiler facade

The existing typed leaf interface evolves from a private part-template adapter into a pure definition projection:

```go
type Definition interface {
    DefinitionData() (DefinitionData, error)
}
```

`DefinitionData` is the accepted R9 canonical union of `EffectSpec` or `CompositionData`; members contain named ordered `DefinitionData` values. The `Data` suffix distinguishes the canonical representation from the typed Go projection. Typed leaves, typed named groups/whole definitions, and canonical data implement this facade. No projection method is retained in a running instance.

`Compile(CompileInput{Definition})` projects once into that canonical model, validates the complete structure and every intrinsic effect, and returns an owned opaque CompiledDefinition. Configured data uses the same path. There is no Go → JSON → Go round-trip, fake participant ID or fabricated catalogue entry merely to compile.

Generic Go composition projection and canonical-data traversal explicitly reject cyclic structure. Repeated acyclic subtrees are copied under their distinct local paths. Arbitrary Go projection code is subject to the pure finite-projection contract, not treated as a sandboxed scripting language.

The compiler reports allowed placement scopes from each kind. It does not choose a scope while validating intrinsic parameters. Existing PartTemplate/PartDeclaration validation is split at that boundary rather than weakened.

## 2. Explicit placement lowers to the existing part contract

A pure placement operation consumes compiled values and mounts:

- local placement key;
- member path selecting a root/group/leaf;
- explicit terminal scope.

Every compiled leaf must be covered exactly once. Missing coverage, overlapping mounts, invalid paths, duplicate placement keys or a scope the kind does not support are errors. A root leaf uses an empty definition path and receives its runtime key from placement; it needs no fake authoring member.

The result is owned `PartDeclaration` data. The placement key prefixes the selected leaf's relative suffix. The operation binds no actual holder, subscribes to nothing and spends nothing. Conditions/application adapters assign roles and then use the existing binding/custody machinery.

| Consumer | Benefit placement | Maintenance placement |
|---|---|---|
| Native Rage | Same local holder; EndPart | Same local holder; EndAttachment |
| Concentration ring | Recipient; EndPart | User/maintainer; EndApplication |
| Activity ward | Selected resistance on recipient; EndPart | Recipient-maintained linked application; EndApplication |

The named native adapter owns its mapping; the generic compiler does not recognize magic group names. A linked ApplicationDefinition maps explicit placements to existing user/recipient roles and produces the existing ApplicationSpec for PrepareApplication. The complete intended target universe and delivery protocol remain unchanged.

## 3. Real, independently reusable Rage maintenance

The typed Raging definition contains:

| Field/group | Existing or required meaning | Native configuration |
|---|---|---|
| Benefits | Existing damage/resistance/advantage meanings | Existing fixed/live scaling choice and Strength melee weapon guards |
| Maintenance.CombatActivity | Existing combat_activity part, with local end scope permitted | First holder end is graced; attack attempts/misses or positive applied damage sustain |
| Maintenance.Duration | New round_duration vocabulary part using existing machine round operations | Ten rounds from first positive observed anchor; duration_expired / clock_reset reasons |
| Maintenance.Endings | Ordinary EndOn values | Holder zero HP → unconscious; rest → rest; combat end → combat_ended |

The new RoundDuration part is not a new clock. It consumes the existing holder turn-end fact and the existing machine AnchorRound/DeriveTurnsActive/round guards. It owns its anchor/derived progress. CombatActivity owns its flags/grace. No state bag is shared between them.

Native maintenance order is activity, anchored duration, then explicit ending policies. The existing assembly's first accepted terminal request gates sibling reactions; it therefore preserves inactivity taking precedence over simultaneous elapsed/reset clock endings. The two new characterization cases name those collisions explicitly rather than infer parity from consecutive-round success.

RoundDuration behavior:

- A positive round first observed on a holder turn end becomes the anchor; zero/negative means no usable round.
- Elapsed progress is derived from round minus anchor plus one, not event count. Missing events do not extend the cap.
- A missing round neither resets the anchor nor grants activity grace again; progress retains its last known value.
- A positive round below the anchor ends with clock_reset when activity has not already ended the instance.
- A live instance expires at round-anchor ≥ rounds-1, before continued-upkeep updates.
- Active restore refuses invalid anchor/progress/state; completed required maintenance cannot be revived.

UntilEmpty and concentration still require application custody where their meaning needs it. Allowing local scope for native activity/round-duration does not make every kind valid in every host.

## 4. Native condition identity and current saved representation

Retain `dnd5e:conditions:raging`, origin attribution and by-ref native admission. Borrowed effects do not acquire that condition identity. Rage activation/payment and its prohibition/eligibility checks remain with their current feature/character owners.

The current ComposedCondition wrapper becomes an assembly-backed local condition rather than preserving an unrelated private Rage machine. It stores a frozen local definition (resolved part declarations with explicit placement) and one AssemblySnapshot. It owns no second runtime state.

ComposedData changes deliberately to contain resolved local bindings plus `runtime.parts`; the old single `runtime.state/bag` format is refused by the new loader. Creation may still accept the existing RagingBindings input, but loading an active instance does not regenerate its effects from a latest named definition or catalogue. No historical reader, dual writer, normalization or data reset is introduced. Consumers choose adoption through their dependency pins.

Restore validates complete part coverage, live required maintenance and each kind's state invariants. The new current-format tests replace old-layout assertions with equivalent mechanical/progress/refusal assertions; they do not weaken native behavior to accommodate a codec change.

## 5. Progress comes from the provider, not a private client bag

RoundDuration inspection returns typed owned progress (anchor known/unknown, elapsed/turns-active and declared total). The kind owns interpretation of its saved state. Conditions.DescribeCondition combines that information with its display/origin and supplies the existing user-facing progress text.

Preserve the currently visible “Active for N turns” fact through the existing API fields:

- `character.StatusView` already uses DescribeCondition.Display.Detail;
- the legacy API character converter already maps that detail to Condition.Notes;
- the v2 private sheet already receives the provider's detail.

The web reader changes to the sole current assembly-backed envelope, treats bindings/part bags as opaque, and stops reading `runtime.bag.turns_active`. ConditionBadge renders the provider-supplied detail rather than calculating elapsed time or maintaining another copy of the counter. Other unmigrated condition readers remain intact. No proto field is required by the inspected paths.

This is an additional web adoption deliverable, not covered by the earlier finding that linked payload parts were opaque. The replacement plan tests actual notes/detail and the current reader together, and includes the relevant local-stack/native observation boundary without claiming native ring inventory.

## 6. Three authored consumers, one implementation

The named `definitions.Raging` constructor exposes actual typed benefit/maintenance fields. Its projection, catalogue export and native local adapter consume those same values. Remove the asymmetric public RagingBenefits/private-ragingRecipe path once the new parity tests are green; keep no shadow implementation to make examples pass.

The code and configuration examples use the accepted catalogue/member derivation contract for a fixed-bonus ring, and unchanged selections for the activity ward. Each gets its own source, role bindings, instance keys and state. No new item-name handler or condition-specific selection loop is permitted.

## Refusals carried into the plan

- Undefined/ambiguous nodes, missing/cyclic refs and invalid whole-member replacements refuse before binding/payment.
- Missing/overlapping placement or unsupported scope refuses rather than silently selecting a host policy.
- Missing live state does not become fresh creation; catalogue edits do not rewrite an active grant.
- Missing round information does not become round zero or a guessed reset.
- An old web/data reader is updated with the provider; the visible progress fact is not dropped or reconstructed from another private representation.
- No release or operator environment reset follows from these implementation choices; normal owning/consumer gates, review and separate merge authority still apply.
