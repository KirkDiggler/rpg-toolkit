# Cleric contribution plan — Cure Wounds

Design baseline: toolkit main `fd4cdade`. Session handoff updated 2026-09-12
against main `ead003f0` after #1693, #1694, and #1695 merged.
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
`rulebooks/dnd5e/encounter/v0.76.0`; #1695 is
`rulebooks/dnd5e/resolution/v0.44.0`. Session #1696 consumes all three real tags.

The user decided that missing creature type must not block selection or healing.
Only a known excluded type receives a paid no-effect result. Include the user
before settling eligibility, missing-data defaults, compatibility, or new scope.
Broad creature classification remains outside this slice. Session #1696 removes
the now-unneeded `HealingTargetsInput.Excludes` argument. Its acceptance tests
verify selection, healing, payment, saved HP and exact story replay after JSON
reload for monsters with missing or unknown refs and no explicit type. The full
session suite and vet pass with released dependencies and no workspace override.
The remaining handoff is review/merge of #1696 and its CI-issued session tag;
API/protos/web adoption remains outside this slice.

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

### Healing Word inspection (2026-09-12)

The user authorized continuing with Healing Word, Bless, remaining cantrips,
and domain support as separate slices, one PR at a time. Healing Word starts
from main `f39a7b26`, after acquisition #1698 released as rulebook v0.160.0.

Existing healing arithmetic, ability/Life contributions, exclusions, paid
delivery, HP persistence, and story results are reusable. Missing creature
types remain eligible under the settled user policy. The current profile
validator permits healing only on touch casts; resolution and session also
use touch-specific target handling. Healing Word needs a visible creature
within 60 feet, including eligible self-targeting, and a bonus-action cost.
Its first-level healing is 1d4 plus casting modifier. Preparation, automatic
domain grants, upcasting and world-clock casting remain separate.

The 2014 bonus-action spell restriction is not currently enforced: session's
cast documentation explicitly relies on spending the standard action to stop
a second spell. The user explicitly chose to implement the shared same-turn
restriction with Healing Word. Enforcement must handle both cast orders,
one-action cantrips, same-turn reaction spells, persistence, and turn boundaries;
it must not be replaced with a blanket one-spell-per-turn restriction.

Sources: [Healing Word](https://www.dndbeyond.com/spells/2140-healing-word),
[2014 casting rules](https://www.dndbeyond.com/sources/dnd/basic-rules-2014/spellcasting).
Acceptance should cover visibility/range, self/ally/other known recipients,
dying recovery, missing-type healing, known excluded types, ordinary/Life
calculations, bonus action plus slot payment, replay, and the agreed same-turn
policy. Stage provider changes first; adopt each real release before advancing
the next module. Do not enable acquisition before the delivery path is usable.

The first provider PR supplies the shared rule and character payment operation:
`combat.SpellTurnState.AfterCast` checks the declared spell level and casting
time in both orders; `Character.CanPaySpell` projects legality and affordability;
`Character.PaySpell` pays through the existing gate and records history only
on success. History persists with the character economy but has its own explicit
turn identity, preserved across economy refresh. It is cleared on combat exit.
Ordinary `combat.Pay` continues to own prices only.

This provider does not enable Healing Word or wire the live casting door yet.
After it releases, the remaining work is spell content/classification, ranged
healing target validation and delivery, then session offers and execution. The
composition must supply a turn identity that distinguishes active creatures,
rounds, and encounters; the payer's existing refresh number is insufficient.
Consumer tests must prove that identity using actual clock transitions, as well
as offer/execution agreement, saved continuation, and failed-payment behavior.

### Healing Word session integration (after #1703)

#1703 released resolution `v0.45.0`. The session slice updates that pin and
rulebook `v0.162.0` together with the calling code; encounter stays `v0.76.0`.
Offers ask `Character.CanPaySpell`, and Cast supplies the same explicit turn
identity to resolution payment. The token includes session, encounter, round,
and active member. Existing combat-exit cleanup clears spell history before
a new fight can reuse round one. The seam reports the provider's restriction
as an unavailable offer; it does not recreate the spellcasting rule.

Healing offers route to touch or ranged healing eligibility in resolution.
Ranged healing includes self and dying/stabilized recipients while requiring
current sight and unblocked range for other creatures. Missing creature type
continues to allow healing; known exclusions remain paid no-effect.

Validation covers both spell orders, action-cantrip combinations, actual round
and active-member changes, combat exit/re-entry, JSON reload, saved cast-push
resume without repayment, healing calculation/story projection, and stale
sight/wall/range/target/slot refusal before payment. Existing Bard and Cure
Wounds tests remain part of the full session suite.

After this session release, the next root-only slice can expose Healing Word
in acquisition. This PR does not change preparation, acquisition, or external
repository pins.

### Healing Word acquisition (after #1705)

#1705 merged as `9fbfba45` and released session `v0.79.0`. The root-module
acquisition slice adds Healing Word to the existing Cleric and Bard options.
Cleric continues to select all supported first-level spells while preparation
is deferred (now four). Bard keeps the existing four-known-spell limit and
chooses four of five supported options. No automatic grant is added to loading
an existing sheet, and existing Bard selections remain valid. An unfinished
Cleric draft must refresh its spell selection to include the new supported entry.

Validate accepted selections through finalization and JSON reload, preserve
spent slots on load, and retain rejection of duplicates, unsupported entries,
and incorrect counts. Slot initialization, casting mechanics, preparation,
domain grants, and adjacent repository pins do not change in this slice.

### Bless: provider, resolution, session, acquisition

Healing Word acquisition #1707 merged as `7d79fad7`, releasing rulebook
`v0.163.0`. Bless is the next authorized spell. Its reference is the
[2014 Basic Rules Bless entry](https://www.dndbeyond.com/spells/2016-bless):
one action, 30 feet, up to three creatures, concentration up to one minute,
and an additive d4 on attack rolls and saving throws.

The user explicitly chose known targets within range and a clear path,
including the caster and dying/stabilized recipients. Current sight is not
required. This must be a declared targeting contract, not spell-name dispatch
or reuse of attack eligibility. Dead/defeated targets are not eligible; Bless
adds no undead/construct exclusion. Keep the existing one-to-three selected
target convention and ten-subsequent-turn-end concentration timing.

1. **Root provider:** add `BlessedCondition`, canonical ref/loader/display and
   cleanup registration, plus the level-one Bless profile and a declared
   `CastTargetKnownCreature` contract. Reuse contributed dice, source-qualified
   addresses, non-stacking groups, and the existing concentration owner.
   No acquisition or consumer pin changes in this PR.
2. **Resolution, after the root release:** pin the actual tag; bind Blessed
   in the existing prepared condition delivery; support known-creature
   eligibility and range/clear-path validation. Prove selected self, multiple
   recipients, dying/death saves, attacks and saves with sourced calculations,
   Bless plus Bane, overlapping casters, reload, and all concentration endings.
   Validate before payment or RNG and preserve the spell-turn gate.
3. **Session, after resolution release:** update the real pin together with
   offer/execution plumbing. Project known targets including self rather than
   current-sight attack candidates, revalidate on execution, and prove the
   resulting casts and bonuses through persisted story and actual turn flow.
   Check whether this consumer requires any additional projection fields.
4. **Root acquisition, after session release:** add Bless to the existing
   supported Cleric selection without silently implementing domain preparation.

Continue one Go module per PR, merge/tag before the next consumer pin, and no
temporary versions or adjacent repository work. Preparation, domain automatic
grants, upcasting, and world-clock casting remain deferred.

### Healing Word content provider (after #1700)

#1700 merged as `412f3dc7` and released rulebook `v0.161.0`. The next provider
adds Healing Word's level-one content: one bonus action and one first-level
slot, one creature at 60 feet, 1d4 plus existing sourced healing modifiers,
and the settled undead/construct exclusions. The profile accepts single-target
ranged healing; resolution must still implement sight/range and healing-life-state
validation, including self and dying recipients, before the session enables it.

Every executable spell now declares `combat.SpellCasting` alongside its content.
Do not derive classification from payment: free spells still obey casting rules.
The older `SpellData` table is incomplete for working Bard cantrips, so it is
not a prerequisite for compiling cast content. An absent profile classification
remains explicitly unclassified; it must never be interpreted as a free action
cantrip by the upcoming consumer. Existing profile validation accepts older
unclassified declarations; the live enforcement contract is still pending.

Cleric/Bard acquisition lists remain unchanged. After this provider releases,
resolution must adopt its real tag and wire classification plus explicit turn
identity into spell payment and ranged healing. Session follows with offers,
execution, and real clock transition tests. No adjacent repository pins change.

### Healing Word resolution integration (after #1701)

#1701 merged as `45d5ab49`, releasing rulebook `v0.162.0`. Resolution adopts
that actual tag, keeping encounter at its existing `v0.76.0` pin.

The user approved rejecting incomplete costed casting inputs, with the explicit
requirement that session's calling code and dependency pin be updated together.
The runner reads classification from the cloned cast profile and requires
`Cost.SpellTurn` plus a matching caster/payer. It uses Character.PaySpell after
target preflight. A free combat spell supplies a Cost with nil Profile; the
existing nil-Cost API remains explicitly ungated mechanical resolution.

Ranged healing uses the same prepared healing delivery and a dedicated target
projection. It permits healable dying recipients, self, and unknown creature
types; exclusions remain paid no-effect outcomes. Other recipients require
current encounter sight holdings as well as range and an unblocked ray. The
encounter's pinned View implementation was audited: it only reads held intel,
without consulting capabilities or refreshing perception.

Before session adopts the resolution release, update every costed cast to
supply the explicit turn identity and project the same spell-payment rule in
offers. Prove identity across actual turn transitions and encounter lifetimes,
including combat exit/re-entry, and preserve already-paid history on resume.
Update the session pin and plumbing in one PR; a pin-only upgrade would refuse
existing casts. Then enable acquisition and verify the complete player flow.

### Supported spell acquisition (2026-09-12)

The user authorized reusing Bard's creation/known-spell pipeline while full
preparation is designed separately. Offer all currently supported first-level
Cleric spells: Bane, Command, Cure Wounds. `ClericSpells1` requires each once;
the shared validator must reject duplicates (#1662). Existing cantrip choices
remain unchanged. This is temporary spell access, not a Wizard spellbook,
prepared-spell limit, or automatic domain grant.

Finalization seeds the existing first-level slot resource from Cleric's class
table (two slots), sharing Bard's initialization and long-rest recovery.
Creation tests cover draft/character persistence, executable definitions,
slot spending/reload/rest, invalid picks, and class changes. Existing finalized
sheets are not backfilled during load. Unfinished drafts must supply the new
spell choice before finalizing; API/UI adoption must expose that requirement.
No consumer pins or preparation/rest selection flows change in this rulebook PR.

Next candidate is Bless using Bane's contributed dice and source-qualified ownership,
including selected-self targeting and overlapping-caster acceptance. Preparation/domain
grants and out-of-combat casting remain separately scheduled work. The eventual API/web
integration must be observed through the real game before declaring the Cleric journey
complete. Historical creation/Sacred Flame evidence remains in `implementation.md`.
