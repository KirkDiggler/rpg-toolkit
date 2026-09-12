# Cleric contribution plan — Cure Wounds

Updated 2026-09-12 against toolkit main `fd4cdade`.
Implementation is authorized by the user in this conversation after their maintainer discussion.
The external proposal still says approval requested; this records the user's authorization,
not an invented GitHub approval or board status.

Proposal: [toolkit #1643](https://github.com/KirkDiggler/rpg-toolkit/issues/1643).
Journey: [rpg-project #406](https://github.com/KirkDiggler/rpg-project/issues/406).
Companions: [design.md](design.md), [implementation.md](implementation.md),
[source-index.json](source-index.json).

## North star

### Release handoff correction (2026-09-12)

The user requires one PR at a time with actual released dependency tags.
Rulebook PR #1693 goes first, followed by encounter #1694, resolution #1695,
and session #1696. Leave the already opened downstream PRs in place; do not
advance or repin them together. After each provider merges, verify its CI-issued
module tag before updating and validating the next consumer. Existing temporary
pins in downstream PRs are outstanding release work, not an approved pattern
to repeat. The user controls merging. No other-repository dependency bumps are
authorized. Verify live PR and release state when resuming; this sequence is
not evidence that any PR has merged.

Provider releases verified: #1693 is `rulebooks/dnd5e/v0.159.0`; #1694 is
`rulebooks/dnd5e/encounter/v0.76.0`. Resolution #1695 consumes those real tags.

The user decided that missing creature type must not block selection or healing.
Only a known excluded type receives a paid no-effect result. Include the user
before settling eligibility, missing-data defaults, compatibility, or new scope.
Broad creature classification remains outside this slice. When advancing #1696,
remove its now-unneeded `HealingTargetsInput.Excludes` argument and verify an
untyped custom monster can be selected, healed, and saved using the resolution
release. That session work remains pending until #1695 merges and is tagged.

### Intended behavior

A level-one 2014 Cleric with explicit access to Cure Wounds can, during initiative,
choose themself or another creature they can touch, spend one action and one
first-level slot, and restore HP through the existing Cast path. The story explains
what was rolled, which modifiers contributed, and how much healing actually landed.
Reload preserves that story, the healed creature and the spent slot.

Example: Wisdom 16 and a d8 result of 5 request 8 HP. Disciple of Life adds a
separately attributed 3 at first level, requesting 11. If the target lacks only
4 HP, the result reports 11 requested and 4 restored. No saving throw or new
concentration is involved, and existing concentration is preserved.

This is a bounded combat healing contribution, not complete Cleric or Cure Wounds
support. Preparation remains deferred; explicit spell-access fixtures are intentional.

## What is already done

- Cleric creation/proficiencies and persistence: merged #1585.
- Sacred Flame content and session acceptance: merged #1592 and #1594.
  Cover fidelity, higher-level scaling and the shared cast-damage correctness
  work remain outside those proofs; do not absorb them into this healing slice.
- Bane supplies leveled-spell offers, first-level resource-backed slot payment,
  ordered target results, contributed dice and source-qualified concentration.
- Blade Ward supplies self-only casting. It does not supply choosing yourself
  among other creature targets.
- Area spells, Dissonant Whispers and Command now add area/cell targets, movement,
  half damage, options and driven turns. Preserve these existing contracts.
- Animated Armor supplies a real construct fixture; Skeleton supplies an undead fixture.

At this baseline session pins D&D `v0.158.0`, encounter `v0.75.0`, resolution
`v0.43.0`, spatial `v0.13.0` and intel `v0.2.0`. Those are evidence of the starting
graph, not versions to enforce forever. No other-repository pins are in scope.

## Design boundaries

### Healing is an immediate consequence, not a stored condition

The existing no-save cast path only delivers conditions. Generalize it to deliver
healing as a distinct consequence. Rename the condition-only result conversion
when its responsibility broadens; never create a transient healing condition just
to change HP. Preserve existing condition delivery and replacement behavior.

Separate initiating/paying, calculating, and applying healing:

- The cast/feature/item/rest entry owns its action and resource cost.
- Shared healing rules produce a sourced calculation from declared dice and
  applicable contributions. A spell delivery invokes this; reusable arithmetic
  must not be buried in the Cast verb or its delivery loop.
- Character/monster handlers remain the owners of HP mutation, clamping and
  applied facts. Character healing above zero already resets death-save progress.
- Persistent features/conditions can affect healing where their rules apply.
  Disciple of Life owns its spell-healing predicate and contribution; it is not
  a Cure Wounds-specific switch in session or a universal healing bonus.

Introduce only the small shared healing operation and context this customer needs.
Do not add a new independently versioned mechanics module, generic effect language,
or speculative potion/rest framework. Other healing sources can reuse the operation
later without inheriting spell rules. Medicine-related actions are not automatically HP healing.

### Content and targeting

Add a healing declaration to `combat/actions.CastProfile`, with validation and deep
cloning, and compile Cure Wounds through the existing spell table. Extend the exact
caster inputs beyond save DC to support the casting modifier and sourced healing
contributions. Reuse the current action-plus-first-level-resource price.

Add a content-controlled way to select one creature including the caster.
`CastTargetSelf` is a no-picker self-only mode; it cannot stand for this choice.
Cure Wounds is not restricted to allies. Do not disturb area, cell or option inputs.

Encounter owns positional facts. Resolution/rulebook owns what those facts mean for
touch and recipient eligibility; session projects those answers. Existing sightings,
attack-target participation and range alone do not establish the complete healing
contract. In particular, do not add a sight requirement merely because attack offers
use sightings. Inspect existing encounter/spatial obstruction capabilities first.
`CellAt` answers movement permission, which is not itself permission to touch.

Resolve and test touch, dead-target and no-effect rules against the reviewed 2014
sources before implementing those decisions. Undead/construct immunity to this spell
must not be confused with an invalid request. Full HP does not imply a malformed target.
Do not fabricate HP support for world-kind NPCs that have no healable sheet.

## Implementation route

These describe implementation responsibilities; the release handoff above governs
the one-PR-at-a-time publication sequence.

1. **Define the acceptance scene and narrow contracts.** Start with a Cleric,
   another injured character, an undead and a construct. Record the expected
   request, payment and sourced result. Resolve the targeting/no-effect questions
   above and the Life feature's load/attachment path. Keep ordinary-caster controls.
2. **Root rulebook support.** Add the healing declaration, shared healing calculation
   and Life contribution, plus Cure Wounds content. Use typed source context so the
   feature can distinguish leveled-spell healing from unrelated healing. Feature
   availability must survive load without duplicate bonuses; it must not require
   implementing the separate automatic domain-spell grant system.
3. **Resolution delivery.** Extend `newGatelessCast`, `preparedCast`, `startCast`
   and `deliverCast`. Preflight declaration and recipient before payment/RNG;
   execute healing inside the existing Gather after the door pays. Publish
   `HealingReceivedEvent` with matching amount/calculation and spell provenance.
   Reuse `EffectHealingApplied` capture. Generalize `deliveredConditions` to
   translate condition and healing results, adding an `ImposedEffect` healing arm
   that retains requested/applied HP, before/after HP and calculation. Preserve
   Command option binding, condition replacement, movement and other result arms.
4. **Encounter and session integration.** Reuse `RecordCast` and
   `ResultHealingApplied`; add record acceptance rather than a new record format.
   Extend encounter production code only where touch or recovery proves a missing
   capability. Update session offers and `imposedResult` to carry the provider's
   target policy and healing facts. Keep `Manager.Cast`, `Targets`, selectors,
   payment, dirty-sheet saving and the existing healing presentation body.
5. **Prove the combined path.** Run the scene through actual module consumers,
   including repository JSON round trips and manager recreation. Fix demonstrated
   gaps before presenting the wave for review. Record observed results and remaining
   limits in `implementation.md`; do not turn this checklist green from code reading.

## Acceptance checklist

Passed in the combined local walk; full root tests also pass on independent pins.
Pushed consumer pins and CI are checked during publication.

- [x] Ordinary caster: correct d8 and casting modifier, with sourced calculation.
- [x] Life Cleric: separate first-level Disciple contribution, applied exactly once;
  survives load and does not apply to an unrelated nonspell healing control.
- [x] Self and another reachable creature are selectable; no accidental ally-only rule.
- [x] Touch boundary and obstruction agree between offers and execution, including
  the resolved visibility rule. No healing through an impassable barrier by distance alone.
- [x] Malformed, duplicate, unknown, stale and unreachable selections refuse before
  RNG/payment; current spell access and target state are revalidated.
- [x] Exactly one action and one first-level slot are spent. Either exhausted currency
  refuses without mutation. Existing cantrips remain action-only.
- [x] Requested versus applied healing is correct at and near maximum HP; full-HP and
  zero-effect casts have explicit, source-backed payment and result assertions.
- [x] Dying/stabilized character healed above zero resets death-save progress and can
  participate correctly afterward and after reload. Dead targets are not revived.
- [x] Skeleton and Animated Armor receive no healing; no-effect handling and cost
  are explicit and distinct from invalid-request refusal.
- [x] Negative healing cannot lower HP. Coordinate the domain guard with #1467;
  keep calculation totals and requested amounts consistent.
- [x] Cure Wounds causes no save or concentration owner and preserves existing concentration.
- [x] JSON round trips and manager recreation retain exact healing story, HP, Life
  availability and spent resources without duplicate application or reroll.
- [x] Existing long rest restores the slot resource; no new slot ledger is created.
- [x] Roller, publication and repository failures have explicit assertions describing
  actual state at failure; no unsupported cross-repository atomicity/refund promise.
- [x] Sacred Flame, Bane, Blade Ward, Second Wind and the newer cast paths retain their
  behavior. Full suites in changed modules and the combined consumer path pass.

## Failure semantics and scope control

[#1467](https://github.com/KirkDiggler/rpg-toolkit/issues/1467) tracks negative healing.
[#1466](https://github.com/KirkDiggler/rpg-toolkit/issues/1466) tracks Second Wind
resource behavior on infrastructure failure. Both remained open at reassessment.
Coordinate shared fixes without silently widening this issue into a resource rewrite.

Known invalid inputs and inability to pay must refuse without spending or rolling.
That is different from an unforeseen failure after payment. Session currently saves
sheets before recording/committing the encounter; do not promise transaction-wide
rollback across those writes. Test and document the actual failure behavior.

Deferred: preparation management, automatic domain-spell grants (#299), upcasting,
level progression, components/focus enforcement, out-of-combat/world-clock casting,
API/protos/web adoption, other-repository pins, Bless content and broad damage fixes.
`session.Cast` still refuses world-clock casts at this baseline. Between-fights healing
is a later explicit slice, not a hidden assumption in combat acceptance.

## Development, review and publication

Advance one owning-module PR at a time using actual released provider tags.
Do not introduce temporary dependency pins or committed local overrides. Verify
the current PR before handoff; the user merges it, CI issues the module tag, and
only then update and verify the next consumer. Keep pending consumer acceptance
distinct from evidence already established for the current module.

Run formatting, tidy and meaningful tests in owning modules. On Windows, keep
unrelated line-ending changes out of the diff. Report local tooling limits honestly;
CI supplies additional validation. Check current adjacent PRs before changing shared
cast/targeting surfaces, especially ongoing perception and declaration presentation work.

Keep this plan and the implementation evidence current as findings change the shape.
User authorization is already given; ordinary implementation choices do not require
asking for the same approval again. Material scope changes should be surfaced clearly.

## After this slice

Next candidate is Bless using Bane's contributed dice and source-qualified ownership,
including selected-self targeting and overlapping-caster acceptance. Preparation/domain
grants and out-of-combat casting remain separately scheduled work. The eventual API/web
integration must be observed through the real game before declaring the Cleric journey
complete. Historical creation/Sacred Flame evidence remains in `implementation.md`.
