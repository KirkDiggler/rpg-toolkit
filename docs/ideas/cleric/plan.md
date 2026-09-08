# Cleric contribution plan

Status: refreshed 2026-09-08 against toolkit main `b2f3b88d` after the overnight
casting/concentration work and merged Cleric PR #1585. Companion: [design.md](design.md).
Player milestone: [rpg-project#406](https://github.com/KirkDiggler/rpg-project/issues/406).

## What changed

| Previous assumption | Verified now | Plan change |
| --- | --- | --- |
| Cleric creation is the first implementation task | [#1585](https://github.com/KirkDiggler/rpg-toolkit/pull/1585) merged; released as `rulebooks/dnd5e/v0.150.0` | Mark the creation slice complete. Preserve its regression suite. |
| Shared casting still needs its first executable cantrip | [#1573–#1576](https://github.com/KirkDiggler/rpg-toolkit/pull/1576) delivered True Strike and Vicious Mockery, cast profiles, nested saves, session Cast, and records | Sacred Flame extends the existing cast content and acceptance path; do not build a second spell machine. |
| Concentration needs a new lifecycle | [#1583](https://github.com/KirkDiggler/rpg-toolkit/pull/1583), [#1584](https://github.com/KirkDiggler/rpg-toolkit/pull/1584), [#1586](https://github.com/KirkDiggler/rpg-toolkit/pull/1586), and [#1587](https://github.com/KirkDiggler/rpg-toolkit/pull/1587) merged ownership, damage checks, removal, records, and session projection | Reuse the merged lifecycle. Bless needs specific extensions and acceptance evidence, not a concentration rewrite. |
| Basic cast wire/UI support remains to be designed | Protos #310/#311/#313 merged; API #949/#951 and web #994/#1000 merged into `dev` | Reuse ref-based cantrip choices, Cast declarations/RPC, save/cast/break beats, and concentration presentation. Verify Cleric through that path. |
| A merged class fix immediately reaches the game | API `dev` still pins root `v0.149.0`; session `v0.69.0` and resolution `v0.37.0` also pin root `v0.149.0` | Record this for later integration. Updating pins in other repositories is deferred and does not block the next toolkit contribution. |

The older #243 and #406 issue bodies still describe pre-implementation gaps.
Use merged code and release pins for planning; their open state is not evidence
that casting is absent. No issue bodies or consumer repositories were changed
in this reassessment. Consumer merges into `dev` do not establish production deployment.

## Completed: creation and persistence

Cleric base armor/weapon proficiencies, fixed shield, Life heavy armor,
subclass-aware choice validation, duplicate-choice rejection, and Life's
conditional starting warhammer check are merged. Regression coverage includes
invalid choices, equipment alternatives/quantities, class/domain replacement,
background coexistence, and draft/sheet JSON round trips.

The Windows appearance test fix also merged. Before merge the full D&D module
suite passed natively with no skips; the timestamp test passed 100 consecutive
runs. See [implementation.md](implementation.md) for scope and verification.
Other domains' automatic grants, dwarf weapon-training grants, and the wider
spell catalog are not newly implemented by this creation contribution.

## Sacred Flame on the existing cast path

The level-one Sacred Flame content merged in #1592 as D&D `v0.151.0`.
It adds Sacred Flame beside True Strike and
Vicious Mockery in `spells/cast.go`. `session/casts.go` offers known cantrips with a supported profile and
uses `Character.SpellSaveDC()`, which already reads the class's casting ability.
This supplies the path for a Wisdom-based Cleric without a Cleric-specific
session verb. The existing cast profile supports a single-target save with
negated-on-success damage, matching the basic shape of this candidate.

Content checkpoint: the profile declares 60-foot range, a Dexterity save,
1d8 radiant damage on failure, no damage on success, and no condition or
concentration. Tests validate the source-backed profile and prove that a
created/reloaded Cleric with Wisdom 16 supplies DC 13 while Guidance and Light
remain known but unsupported. Session now adopts D&D `v0.151.0` and verifies
the offer, both save outcomes, action payment, exhausted slots, range/refusals,
and JSON-persisted story/HP after manager recreation. No other-repository pins
changed. The full D&D content suite and session suite pass natively on Windows.

Acceptance work:

- [x] Add the source-backed level-one Sacred Flame content using the current
  profile (range, Dexterity save, radiant damage, no damage on success).
- [x] Prove the session offer's known-cantrip ownership, valid candidates,
  forged/stale declaration rejection, one action payment, and no spell-slot cost.
- [x] Verify the saved result and damaged target survive the session save/load path;
  unsupported known cantrips must not acquire executable offers by accident.
- [x] Explicitly assess the cover exception against the current save/targeting
  implementation; absence of all cover handling is not proof of that exception.
  Review the remaining general casting rules before claiming full spell fidelity.
  Result: current intel/range preflight has no geometric cover input to the save;
  there is no cast-profile exception flag. Cover fidelity and visibility/total-cover
  acceptance remain unverified; see [implementation.md](implementation.md).
- [ ] Publish toolkit modules inside-out where more than one module changes.
  API/web adoption and pin updates are deferred to the later integration pass;
  inventory concrete missing fields before proposing any proto changes.

### Separate shared correctness dependency

[#1582](https://github.com/KirkDiggler/rpg-toolkit/issues/1582) is still open and
confirmed in `resolution/contest.go:applyPreparedDamage`: spell damage is not
folded through the damage chain, so resistance, immunity and vulnerability are
not applied by their subscribers. Sacred Flame content can be developed while
that fix is owned separately, but its damage-correctness acceptance must cover
those cases before the feature is described as fully supported. Do not hide
this shared fix inside an unrelated Cleric content change or redo the strike
machine. Concentration checks must use the resulting applied amount.

## Remaining level-one Life work

### Bless recipient behavior is a separate part of the spell

Concentration supplies ownership and cleanup; it does not supply the blessed
recipient's bonuses. Implement a persisted Bless condition on each selected
recipient and register it with the caster's existing concentration owner.

Recipient acceptance checks:

- The recipient's attack rolls and saving throws receive the spell's d4 bonus;
  ability checks do not. Roll breakdowns identify the contribution.
- An ally's rolls use that ally's condition, not a bonus attached only to the
  caster. Include the caster among the valid recipients when selected.
- Save/load retains the effect and its granting caster/spell identity without
  duplicating subscriptions or bonuses.
- Ending or replacing the owning concentration removes those effects and their
  future contributions. Unrelated effects and another caster's ownership survive.
- Overlapping Bless casts follow the reviewed same-spell stacking rules; prove
  both the effective bonus and removal behavior before claiming support.

Multiple-target selection and the concentration-duration extensions remain
separate acceptance work alongside that recipient condition.

| Slice | Reuse | Remaining work |
| --- | --- | --- |
| Preparation, domain grants and slots | Known refs, persisted sheet data, existing spend/recovery substrate | Prepared class choices versus always-prepared domain grants; deduplication without losing grant origin; live slot spending/recovery and stored-data migration under [#799](https://github.com/KirkDiggler/rpg-toolkit/issues/799). Current Cast offers read cantrips and price one action only. |
| Cure Wounds and Disciple of Life | Cast declaration, target preflight, payment and record flow | A healing consequence; touch/ally/self and no-effect targeting semantics; spell-healing context; Wisdom and domain contribution; maximum HP/downed interactions; atomic persisted HP/slot results. Do not duplicate the cast door. |
| Bless | Merged concentrating owner, follow-up saves, removal events, record and UI projection | Multiple selected recipients (current CastInput has one Target); attack/save bonus condition; one-minute duration beyond the current caster-turn-end counter and combat-end cleanup; voluntary ending/incapacitation coverage; precise ownership when casts overlap; slot/preparation support. |

Bless acceptance must demonstrate that ending one caster's concentration removes
only that spell's effects, including after reload and with overlapping casts.
The current child address is `{member_id, condition_ref}`; the multi-caster
case needs an explicit design/proof before treating that address as sufficient
for Bless. This is a remaining acceptance boundary, not a new bug claim from
the earlier mixed-branch review.

Agreed sequence: completed creation -> Sacred Flame session acceptance ->
Cure Wounds with Life healing -> Bless. Use explicit spell-access fixtures while
preparation is deferred; add live slot payment when leveled spells require it,
reusing existing slot storage and long-rest recovery. Shared damage correctness can proceed
alongside the content work. Concentration is now an adopted dependency rather
than a reason to hold all Cleric development.

## Consumer handoff and completion

Existing consumer evidence:

- [protos #310](https://github.com/KirkDiggler/rpg-api-protos/pull/310),
  [#311](https://github.com/KirkDiggler/rpg-api-protos/pull/311), and
  [#313](https://github.com/KirkDiggler/rpg-api-protos/pull/313).
- [API #949](https://github.com/KirkDiggler/rpg-api/pull/949) and
  [#951](https://github.com/KirkDiggler/rpg-api/pull/951), merged into `dev`.
- [web #994](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/994) and
  [#1000](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1000), merged into `dev`.

These demonstrate shared integration, not a Cleric acceptance run. At the later
integration pass, update consumer pins as needed and verify
class availability, all three selected Cleric cantrips surviving creation, the
supported Sacred Flame offer, its visible result, and reload through the actual
consumer path. No new Cleric service or complete spell-management RPC is implied.
No other-repository pin updates are part of the current toolkit work.

The playable milestone remains an explicitly supported subset: creation,
preparation, an at-will save cantrip, domain healing, Bless, slot exhaustion and
recovery. Complete class support additionally requires the agreed 2014 spell
catalog, rituals, components/focus and remaining behaviors. Other domains,
Channel Divinity, leveling, multiclassing and 2024 rules remain outside scope.

The refresh used merged code, dependency pins, release tags and PR/issue records.
The subsequent slices ran profile, Cleric creation/persistence and session
acceptance tests. They did not conduct an API/web or live Cleric playtest.
