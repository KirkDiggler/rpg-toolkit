# Composition contracts — concrete proposal

Scope: settle the authoring-value and compilation boundary under [R6–R8](design.md), before choosing reference syntax or changing native Rage upkeep. These are proposed contracts, **not implemented APIs or an approved replacement execution plan**. The first-cut source remains unchanged.

## The structural choice

Two viable representations are an ordered flat effect list plus a separate group index, or an ordered named tree that lowers to flat runtime parts. The recommendation is the tree: it directly represents `Raging → Benefits → PhysicalResistance`, permits reuse at each level and avoids a second group-membership index. Existing runtime assemblies remain flat.

**A composition is structure, not an implicit lifetime boundary.** Its members may own state when instantiated, but merely putting them in a group creates no additional clock, shared bag, activation, ending behavior or runtime identity. Ending scope belongs to explicit host placement.

## 1. Canonical resolved values

Working Go data-shape names:

```go
type Definition struct {
    Effect      *EffectSpec
    Composition *Composition
}

type EffectSpec struct {
    Kind   generic.KindName
    Params json.RawMessage
    When   *machine.Guard
    Amount *generic.Amount
}

type Composition struct {
    Members []Member
}

type Member struct {
    Key         string
    Name        string
    Description string
    Definition  Definition
}
```

This is the **canonical resolved data shape**, not a requirement for Go callers to hand-write params JSON. Typed effect values project to EffectSpec through their owning kind adapters. Configuration decoding/reference resolution produces the same values. Mechanical validation stays in the one kind table.

Contracts:

- Exactly one of Effect or Composition is present. A zero Definition is missing/invalid, not an empty composition.
- A configured effect has one supported kind and only that kind's supported params/guard/amount. No holder, source, application, condition ref, price, runtime bag or OnTerminal is present here.
- A composition has ordered members. An explicitly empty composition is a valid empty value; a host may require a nonempty grant. Absence is not silently normalized to empty.
- Each member has a nonblank key unique among its siblings. Keys exclude `/`, preserving the current simple separator convention. Same keys under different parents are valid.
- Name/Description are descriptive metadata, never dispatch or identity. They may be absent on general authored values; built-in discoverable definitions provide useful metadata.
- Repeating a mechanical kind is valid. Physical resistance and fire resistance are distinct members because they have different member keys/configuration, not because the compiler invents new kinds.
- A member is either a configured effect or another composition. Finite nesting is authoring structure; it does not introduce nested runtime scopes. Cyclic in-memory composition graphs are errors; reusing an acyclic subtree in two locations is allowed.
- References are **not** a third canonical node type. A resolver must replace references with selected values before mechanical compilation. Catalogue/reference syntax, variants and limits remain the next contract, not an embedded interpreter here.

The existing PartTemplate is close to EffectSpec. PartDeclaration/PartSpec also carry key and terminal scope; those currently conflate unbound definition and placement. They remain inspected implementation seams, not the proposed universal authoring model.

## 2. Paths, identity and order

A member path is a sequence of local keys, for example:

```text
["benefits", "physical-resistance"]
["maintenance", "combat-activity"]
```

The array is the definition address. It is not a runtime instance ID, condition reference or source identity.

- Depth-first traversal follows each declared member list. Maps, labels and catalogue registration order cannot reorder it.
- A root effect has an empty relative path. It is valid without a fake member or catalogue name.
- Reusing a group under local key `gift` produces local paths such as `gift/physical-resistance`; its original catalogue path does not become runtime identity.
- Reusing the same definition under `left` and `right` creates distinct qualified local paths and, when bound, independent state. No state is interned merely because the values share a source.
- The host prefixes these relative paths with its own placement/instance identity when lowering to existing assembly part keys. Runtime keys remain opaque to effects; they are not parsed to reconstruct provenance.
- Grouping does not declare stacking behavior. AddToRoll's explicit contribution-selection policy remains a separate mechanical input.

## 3. Typed rulebook projections

Proposed package direction:

```text
definitions  → effects
conditions   → definitions + effects
mechanics/effects ← rulebook consumers
```

`rulebooks/dnd5e/effects` owns reusable definition/composition values, intrinsic validation and compilation. A proposed `rulebooks/dnd5e/definitions` package owns named rulebook content such as Raging. It does not import condition runtime, session or source budgets. `conditions` consumes these definitions through its current runtime/identity responsibilities. Neutral mechanics knows none of the named content.

The named constructor returns a complete typed value. Its groups and members are real fields:

```go
raging.Benefits
raging.Benefits.PhysicalResistance
raging.Maintenance
raging.Maintenance.CombatActivity
```

The exact constructor input and mechanical types of the maintenance fields follow the separate native-upkeep characterization. They cannot be guessed from the existing holder-turn Duration adapter.

Projection contract:

- A typed effect, typed group or complete named definition can project to the canonical Definition above. The compiler-facing projection is pure: no lookup, attachment, payment or running-state access.
- A group projects its actual fields into ordered Member values with stable keys. It may specify keys/order/metadata once; it does not reconstruct another copy of the mechanical configuration in a parallel catalogue.
- The catalogue obtains its generic representation through that same projection. It does not own a second JSON version of Rage.
- Go users discover parts through fields. Dynamic tooling enumerates generic members. GetPart is not required on the primary typed surface.
- Constructing a named definition returns caller-owned values. Derivation uses an explicit owned copy when nested mutable values are shared; ordinary Go shallow assignment is not advertised as a deep clone.
- Compilation detaches all slices, guards, amounts and parameter storage. Editing authoring values afterward cannot change a compiled definition or running instance.

Whether the Go entry point accepts a projection interface or an explicitly projected Definition is a remaining surface choice for the compiler API, not permission for a second mechanical compiler. Recommended ergonomics: accept a leaf, group or complete typed definition through one error-returning compile entry point; projection errors propagate through it, and only canonical data is retained.

## 4. Compilation contract

Conceptual operation: **Compile(unbound definition) → owned compiled definition or error.**

Inputs contain no participants, runtime IDs, catalogue requirement, payment or live world. The configured value may arrive through typed projection or as already-resolved canonical data. Both enter the same intrinsic validator.

The compiler:

1. Validates the complete finite structure: exactly-one node shape, unique sibling keys, valid member paths and no cycles.
2. Validates every leaf through the owning kind, including parts that might later be unselected for a particular action.
3. Produces ordered qualified leaves with their relative paths and detached configuration. Group structure/metadata remains available to authoring/discovery; runtime lowering does not assign groups their own state machines.
4. Reports kind-owned capabilities/placement requirements without inventing participant IDs or guessing a terminal scope.
5. Returns no partial compiled result on error.

A compiled value is not an active effect and does not decide whether a target may receive it. The host still performs admission and binding.

The compiler stores no live callbacks or catalogue handles. Saving a running instance continues to retain resolved definitions plus its own state; restoring does not re-resolve a reference against a latest catalogue.

## 5. Placement and lifetime contract

The same configured upkeep may end a local condition in one host and a linked application in another. Therefore a reusable effect/composition cannot silently carry its original binding.

Host placement supplies:

- a local placement key and exact instance identity;
- holder/maintainer/recipient roles and immutable provenance;
- explicit terminal routing into the existing EndPart / EndAttachment / EndApplication mechanisms;
- the existing condition/application persistence and custody context.

The kind validates that the chosen placement is supported. Separating intrinsic validation from placement validation does not weaken the latter. Native Rage's final mapping/state contract is still a required design step.

A grouping member such as `benefits` or `maintenance` has no magic compiler meaning. A complete named definition/host adapter explicitly assigns the appropriate groups to placement. An EndOn leaf ends what its placement says; it does not infer “end every sibling group” from where a friendly name happens to appear.

**This is not a proposal for a new public generic binder yet.** It is the refusal boundary the revised binder/lowering contract must obey. Current BindPart and ApplicationSpec expose the actual places that need reconciliation.

## 6. Concrete conformance cases

| Input / operation | Required result |
|---|---|
| One resistance definition, no catalogue or condition | Intrinsically valid compiled leaf at empty relative path; no instance created |
| A benefits group containing damage/resistance/save/check leaves | Ordered qualified leaves, no automatically added rest/concentration/duration |
| Two sibling members named `resistance` | Error, not last-writer-wins |
| `benefits/resistance` and `other/resistance` | Distinct valid local paths |
| Two members referencing the same already-resolved subtree value | Independent qualified leaves; no shared live state when instantiated |
| Empty explicit composition | Valid empty compiled value; host grant requirements remain separate |
| Definition with neither or both Effect/Composition | Error before any binding |
| Malformed leaf late in a group | Whole compilation refuses; earlier members are not partially activated |
| Cyclic Go composition structure | Explicit structural error, not unbounded recursion |
| Typed Raging field vs resolved configuration selection | Equivalent canonical mechanical value; no original Rage identity/lifetime attached |
| Modify caller-owned data after compile | No effect on compiled/saved values |
| Name a group `maintenance` without placement | No implicit lifetime rule or authorization to end an application |

## Still separate

The next reference contract must address concrete catalogue refs, member addressing, variants/parameter choices and how a reusable group is configured without a generic deep-patch language. In particular, a fixed-bonus ring and live-Barbarian-scaled native Rage must not be conflated merely because both reuse Rage benefits.

The native-maintenance contract must characterize real round anchoring, missing/reset clocks and terminal ordering before deciding the actual typed maintenance fields and lowering. Those investigations do not block agreeing on the definition/composition boundary, but they do block claiming a complete replacement execution plan.
