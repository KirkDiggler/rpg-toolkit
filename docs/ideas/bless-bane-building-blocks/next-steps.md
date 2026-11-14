# Path forward — effect-first definitions

Direction: [design R6–R8](design.md#rulings). Agreement and the change from the first cut: [PR #2425 record](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6869803445). This is the design/replanning sequence, **not a checked implementation plan**. The [first-cut plan](plan.md) and [implementation receipt](implementation.md) remain evidence; do not continue their consumer-adoption step as if the authoring model were unchanged.

## Where we stand

- The architectural direction and acceptance bar are clear: configurable effects/compositions, typed discoverable fields, one code/config representation, complete native Rage and symmetric maintenance reuse.
- The existing unmerged source is useful: neutral selection, typed leaf meanings, returned construction errors, custody and ordering/lifetime tests are retained.
- The concrete canonical type/reference/native-maintenance contracts still need to be fixed against the code. We do not yet have a handoff-ready replacement execution plan.
- Root API adoption, independent review of the final shape, and merging remain on hold. There is no new operator approval gate for a faithful plan; return only a genuine unresolved architectural/rules decision.

## 1. Reconcile the complete definition and its actual behavior

[The native baseline](native-maintenance-evidence.md) records fresh passing Raging
suites, exact clock/grace cases and the inspected native reader dependency.
Simultaneous-ending characterization and the replacement state/lowering contract
still need completion; the baseline is not a new runtime design.

**Owner:** root rulebook design, with mechanics/effects read for its existing capabilities.

Inspect the native Raging recipe/codec and the activity/duration/end parts together. Produce a parity matrix for activation grace, attacks including misses, positive applied damage, idle ending, ordinary round progress, missing/backwards/reset clock observations and competing terminal events. Identify which native state fields and condition-identity checks downstream code consumes.

**Output:** measured behavior/state matrix and source map, with named existing tests and any needed characterization cases. This decides whether existing parts suffice, need a precise extension, or need another reusable rulebook policy. It does not decide to replace round anchoring with turn counting because the latter is convenient.

## 2. Specify one canonical effect/composition model

The [accepted R9 composition contract](composition-contracts.md) supplies resolved
data shapes and conformance cases: ordered named tree, flat runtime lowering and
explicit placement rather than group-owned lifetimes. [Acceptance](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6873569590)
is recorded. The compiler facade, reference/variant and native-maintenance contracts
remain distinct work, not a reason to reopen R9.

**Owner:** reusable rulebook effect/composition layer, below condition/application placement.

Fix the concrete types for a configured leaf, ordered named composition members and a complete named definition. Specify how a typed Raging value exposes real fields and projects those same values into the generic model, with no duplicate content list. Separate intrinsic configuration from runtime holder/scope/source assignment. Make cloning/compilation ownership, invalid input and unbound compilation explicit.

**Output:** concrete API sketches for whole Rage, its benefit group, one physical-resistance field and its maintenance group. Name the package dependency direction, including the named-definition/catalogue home, before writing tasks. The compiler must accept a direct value without creating a one-entry catalogue merely to operate.

## 3. Specify configuration discovery and reference resolution

The [R10 contract proposal](reference-contracts.md) specifies selection/ownership
and makes the fixed-bonus variant question concrete. [Its decision checklist](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6873671557)
asks only about explicit whole-member replacement versus reassembly/another
explicit parameter contract; no generic merge is assumed.

**Owner:** catalogue/resolver adapter over the same values.

Fix definition/group/member identity, enumeration metadata, direct effect/composition declarations and references, supported depth, and refusal of missing/ambiguous/invalid references. If composition references can be recursive, define cycle detection rather than leaving evaluation order to runtime. Resolve and freeze selected content before binding/payment; do not implement latest-catalogue reload or dynamic inheritance.

**Output:** configuration examples for the exact same three consumers as the typed examples, and a comparison showing their resolved mechanical values/placement agree. A studio-facing metadata contract is not a studio UI implementation.

## 4. Map complete native Rage into the existing runtime

**Owner:** named rulebook definition plus condition/application adapters; neutral provider changes only if a measured capability is missing.

Make native Rage consume the complete definition that exposes the fields. Preserve its measured semantics, not a private recipe shape merely for convenience. Specify local versus linked binding, the relevant condition identity, terminal-scope mapping and the saved-representation decision. Keep payment/activation eligibility and application custody with their current owners.

**Output:** precise provider/consumer/state contracts and three integration journeys: native Rage; ring reusing benefits with concentration; ward reusing physical resistance with Rage upkeep. Both code and configuration paths must reach those journeys. No facade over an unrelated private maintenance recipe counts as completion.

## 5. Replace the execution contracts, then build the revised cut

Update S20 #956 and the current draft rather than invent a competing initiative. Re-cut the tasks in module-sized PR boundaries, with exact files/interfaces, RED→GREEN tests, requirement coverage and provider/consumer tables. Retain the finished neutral/leaf work; adapt or remove the condition-first adapters that no longer belong. Create additional slice issues only if the measured plan introduces a distinct deliverable/release unit.

The final gates include:

- normal Rage behavior and native identity/state handling;
- complete, group and member reuse through typed fields and configuration;
- the ring and reverse-direction ward proof;
- preserved Bless/Bane selection, independent lifetimes, exact custody and source provenance;
- malformed references/inputs and owned compiled/saved values;
- actual API consumer graph and private/live/reloaded behavior after the shape settles;
- one joined independent review, exact-head local/remote gates, and separately authorized inside-out release.

**Output:** a checked replacement plan linked from #2425/#703. That plan, not the retained first-cut task list, becomes the next executor's authority.
