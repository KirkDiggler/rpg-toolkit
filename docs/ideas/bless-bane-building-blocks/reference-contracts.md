# Catalogue references and explicit variants — R10 proposal

This builds on the accepted [composition contract R9](composition-contracts.md). References are authoring instructions resolved into those owned values; they are not runtime nodes. The proposal is not implemented and does not authorize an API rollout.

## Why this is the next real decision

Native Rage's damage amount is live Barbarian-level scaling. The existing ring fixture deliberately uses fixed +2 on a non-Barbarian recipient. Selecting a catalogue value must not secretly change that configuration or invent a class level.

Two honest ways to express the fixed-bonus variant are:

1. Assemble a group from selected reusable members plus an explicitly configured damage effect. This needs no derivation operation but repeats the group's membership when only one member changes.
2. Select the group, copy it, and replace a whole named member. This preserves group reuse while making the change explicit and validated.

**Recommendation:** support the second as a small, pure authoring operation, alongside ordinary assembly. It replaces complete definition values at existing member paths; it is not a generic field merge, callback, deep JSON patch or mutation of a running condition. This is the concrete use case for the earlier optional override idea.

## 1. Catalogue identity and discovery

- A catalogue entry has a `core.Ref` naming an authored definition, a display name/description, and authored content. Proposed namespace: `dnd5e:definitions:raging`, distinct from the runtime condition ref `dnd5e:conditions:raging`.
- A definition can be an effect, composition or complete named definition. Catalogue registration does not decide a runtime holder, source, price or lifetime.
- Built-in entries are projected from the actual typed definition values. Data-defined entries use the same value model; no second JSON implementation of native Rage is maintained.
- Catalogue construction validates the complete supplied set, including unused entries. Duplicate refs and invalid/unresolved content refuse the whole catalogue. No global mutable registration or network fetch occurs while binding an instance.
- Enumeration returns owned metadata for each definition/group/member: ref, relative path, node form, name/description, declared order and kind information for leaves. A studio can present actual choices without guessing key names.

## 2. Selection is a ref plus a path

Working contract:

```go
type DefinitionReference struct {
    Definition core.Ref
    Path       []string
}
```

`Path` follows R9 member keys. Empty path selects the root value; `['benefits']` selects a group; `['benefits', 'physical-resistance']` selects a member. The selected value is copied, not removed.

The reference's definition identity and relative member path remain separate. Core ref parsing currently has its own segment-count bound and identifier grammar; do not concatenate arbitrary member paths into a ref or change core's grammar for this feature.

The proposed configuration spelling uses one ref string plus an array:

```json
{
  "use": {
    "definition": "dnd5e:definitions:raging",
    "path": ["benefits", "physical-resistance"]
  }
}
```

This is an authoring envelope, not a third canonical R9 node. Resolve it to the actual configured value before compilation. The ref string is explicitly parsed/validated with core; the new wire contract does not silently inherit the existing core decoder's historical object fallback.

Traversal uses member keys, never display names or kind names. Missing refs/members, traversal through an effect leaf, ambiguous/duplicate keys and unsupported envelope fields are explicit errors. Selection does not infer parameters from the receiving source or actor.

## 3. Whole-member replacement on an owned value

A selected composition can be derived with explicit replacements:

```go
type MemberReplacement struct {
    Path       []string
    Definition AuthoredDefinition
}
```

`AuthoredDefinition` here denotes the authoring form that can still contain references. The final public type spelling belongs with the concrete resolver API; its result must be the already agreed canonical Definition, with no references left inside it.

Rules:

- Paths are relative to the selected value, not the original catalogue root or destination application.
- Every replacement target must already exist. Empty path is refused: using another whole value is ordinary selection/assembly, not a disguised root replacement.
- Duplicate paths and ancestor/descendant overlaps in one request are errors. Results cannot depend on mutation order.
- Replace the target member's complete Definition. Retain its local key, position and descriptive member metadata; new metadata or membership can be authored by ordinary composition rather than an implicit merge.
- Do not merge kind params, Amount fields, guards, arrays or runtime state. The complete replacement is validated through the ordinary kind/compiler contract.
- Validate all targets against the selected source shape before applying anything. A bad later replacement returns no partial derived value.
- The source catalogue and other selections remain unchanged. Replacement creates no runtime state, chooses no lifetime and carries no price/refund semantics.
- Replacement values may use other valid catalogue references; they undergo the same resolution/cycle checks.

Keeping a member key does not claim its old mechanical meaning still applies. Keys/metadata remain authored descriptions; the replacement's kind/configuration is the only mechanical authority.

## 4. The fixed-bonus ring, without implicit overrides

The ring can select native Rage's benefit group and explicitly replace its damage member with a complete fixed-bonus effect. This sketch assumes the named definition's member key is `damage-bonus`; that key must be fixed once in the typed projection/catalogue contract.

```json
{
  "use": {
    "definition": "dnd5e:definitions:raging",
    "path": ["benefits"],
    "replace": [
      {
        "path": ["damage-bonus"],
        "definition": {
          "effect": {
            "kind": "damage_bonus",
            "amount": {"fixed": 2},
            "when": {
              "all": [
                {"fact": {"name": "action.melee", "op": "eq", "value": true}},
                {"fact": {"name": "action.weapon_pool", "op": "eq", "value": true}},
                {"fact": {"name": "action.ability", "op": "eq", "value": "str"}}
              ]
            }
          }
        }
      }
    ]
  }
}
```

The guard is explicit because this replaces a complete effect, not one amount field. It is configuration, not copied execution logic. A frequently reused fixed-bonus composition can itself be named once in the catalogue.

Native Rage continues to use its live-scaling member. The derived ring shares the resistance/advantage definitions and selects its own concentration maintenance at the host layer. A ward selects physical resistance and the Rage maintenance group unchanged. All three have equivalent typed-code and configuration construction paths.

## 5. Resolution, cycles and ownership

- Forward references within the supplied catalogue are allowed; registration order is not execution or precedence.
- Resolve references and replacements before intrinsic mechanical compilation and before binding/payment. Inline content needs no catalogue unless it contains a use reference.
- Cycle detection follows the active definition/ref/member expansion path, not a blanket ban on mentioning the same catalogue entry twice. Reusing a resolved acyclic subtree twice is valid and later gets independent instance paths/state.
- Missing/ambiguous references, reference cycles, malformed replacement paths and invalid resolved definitions refuse without partial catalogue publication or partial activation. Error context names the entry/member/reference path that failed.
- Resolver caches, if used internally, expose no borrowed mutable tree. A result owns its values; no running instance retains the catalogue or reruns replacement on reload.
- Definition derivation identity is not activation provenance. The new source still supplies its own origin and application/attachment identities when bound.
- No arbitrary reflection paths, runtime factory callbacks, latest-version lookup, hidden defaults or automatic old-format normalization are introduced.

## 6. Provider/consumer boundaries

| Operation | Input | Output / refusal |
|---|---|---|
| Build catalogue | Explicit entries from typed projections and authored documents | Validated immutable catalogue, or complete refusal including unused invalid entries |
| Resolve selection | Definition ref + member path, optional complete-member replacements, supplied catalogue | Owned canonical R9 Definition with all uses expanded; no runtime identity |
| Resolve inline content | Authored effect/composition and optional catalogue only if references occur | Same canonical Definition, not a fabricated catalogue entry |
| Compile | Resolved canonical value or the typed projection facade | One intrinsic validator and owned compiled values; no catalogue lookup |
| Bind | Compiled values + explicit host placement/identity | Existing runtime/custody with independent state; no definition derivation |

The compiler facade should expose one fallible operation to Go callers for a typed leaf, group or whole definition; the projection delegates into the same canonical compiler. Exact facade/resolver function signatures are still to be fixed in the replacement plan, not inferred as multiple evaluators from this table.

## 7. Conformance cases

- Selecting the root, a group or a leaf returns the exact configured value, not a variant chosen by the caller's class or source type.
- A fixed+2 damage replacement changes only that member; the original live-scaling definition and other derived consumers remain unchanged.
- A bad or overlapping replacement returns no partial value; an unknown target does not create a new member.
- Two references to the same group under different local keys are both legal; compilation/binding gives independent qualified paths/state.
- Self/cross-reference cycles refuse; acyclic reuse, including valid references to another member of the same definition, does not fail merely because the ref repeats.
- Editing a resolved/derived output cannot change the catalogue or a second resolution result.
- Reloading an active instance after a catalogue edit preserves its saved resolved content and state.
- Native Rage, the fixed-bonus concentration ring and the activity ward resolve to equivalent typed/configured mechanical values and intended host placement.

## Decision boundary

The proposed new capability is **explicit whole-member replacement during authoring**. R9 remains settled. No replacement or parameter-override implementation starts merely because this document exists. Native Rage's timeline/state decomposition is still a separate required contract; this reference operation cannot make a turn counter equivalent to round anchoring.
