# Actions Are Data — Implementation Record

**Implemented:** 2026-08-24

**Issue:** [#1198](https://github.com/KirkDiggler/rpg-toolkit/issues/1198)

**Design:** [design.md](design.md)

**Plan:** [plan.md](plan.md)

**Decision:** [ADR-0045](../../adr/0045-actions-are-data.md)

## What shipped

The implementation landed through four independently versioned Go modules, each
using the repository's normal squash workflow:

| Layer | Pull request | Squash commit | Released tag |
|---|---|---|---|
| root D&D | [#1234](https://github.com/KirkDiggler/rpg-toolkit/pull/1234) | `6eac5d2` | `rulebooks/dnd5e/v0.99.0` |
| resolution | [#1235](https://github.com/KirkDiggler/rpg-toolkit/pull/1235) | `aa2e87c` | `rulebooks/dnd5e/resolution/v0.12.0` |
| encounter | [#1236](https://github.com/KirkDiggler/rpg-toolkit/pull/1236) | `3954a4a` | `rulebooks/dnd5e/encounter/v0.33.0` |
| session | [#1237](https://github.com/KirkDiggler/rpg-toolkit/pull/1237) | `588c56c` | `rulebooks/dnd5e/session/v0.27.0` |

The original integrated draft [#1232](https://github.com/KirkDiggler/rpg-toolkit/pull/1232)
was never merged. It proved the whole path but violated the repository's
module-isolation and squash workflow. It was quarantined, used as a
file-reconciliation source, and closed after all four replacement PRs and tags
were verified.

## Final architecture

- `rulebooks/dnd5e/combat/actions` owns inert, serializable `Definition`,
  `AttackProfile`, delivery, ability/weapon evidence, and ordered
  `ConditionApplication` contracts. It remains a package inside the root D&D
  module and has an import-boundary regression test.
- `character.AssembleAttack` derives the shared definition from sheet,
  equipment, proficiency, grip, and ability facts. `character.CostOfSwing`
  owns Attack-plus-Strike price composition.
- Monster factories author complete shared definitions directly.
  `monster.Data.Actions`, `Monster.Actions`, loading, and cloning all use that
  contract; no executable `monster/actions` loader remains.
- Flurry of Blows spends Ki and banks `CapacityFlurryStrike`; it does not grant
  a self-removing action object.
- `resolution.NewAction` validates and dispatches by the populated profile arm.
  Strike consumes the shared attack profile without producer or content-ref
  switches.
- `Machine.Start` is pure preflight. Definition, participant, range, condition,
  recurrence, and save-ability checks happen before payment; effective-AC and
  attack chains begin only after the door accepts the cost.
- Strike interprets melee/ranged delivery, long-range disadvantage, attack
  cancellation, spell-versus-weapon damage attribution, one ordered damage
  fold/application, and ordered condition outcomes.
- Monster and character condition receivers both persist applied conditions.
- Encounter carries opaque `ActionView.RangeFeet`; session projects
  `AttackDelivery.MaxRangeFeet()` into it.
- Session assembles character definitions, selects cloned monster definitions,
  presents their one declared cost to resolution, and maps
  `resolution.ErrOutOfRange` to its existing `ErrOutOfReach` contract.

## Delivery and implementation discoveries

### Module delivery had to be corrected

The first plan tried to preserve provider pseudo-version commits with a merge
commit. That contradicted `CLAUDE.md`'s module-isolation rule and the repository's
squash workflow—the exact squash-SHA failure ADR-0034 already documents.

The corrected flow was provider-first and tag-driven. Root D&D merged and
published `v0.99.0`; resolution pinned that stable version, merged, and
published `v0.12.0`; encounter independently published `v0.33.0`; session then
pinned all three stable providers before merging. Resolution could precede the
new encounter release because it uses the already-published `CellsFromFeet`
contract, not the new `RangeFeet` projection.

### Review strengthened the boundaries

Review found and fixed several cases not exposed by the original integrated
suite:

- empty non-hand inventory slots no longer assemble as unarmed strikes;
- malformed condition parameter JSON is rejected before storage;
- character assembly reads equipment slots directly rather than allocating a
  persistence snapshot;
- monster action labels follow one authored-name convention;
- effective AC cannot publish before payment;
- unsupported recurring gates and noncanonical save abilities fail during
  outer preflight;
- attack-chain cancellation stops before dice and damage;
- spell attacks retain `DamageSourceSpell` rather than looking like weapons;
- scripted resolution rollers fail on exhaustion and therefore pin exact roll
  order;
- monster-target condition applications are proven to return dirty persisted
  condition data through the released root keeper;
- non-nil monster definition costs reach resolution's door and are refused
  rather than silently executing free;
- ranged monster turns and affordability use delivery-neutral maximum-range
  vocabulary consistently.

## Verification evidence

After all four squash merges and tags, the idea worktree was merged with current
`origin/main` and the following evidence passed:

```bash
make test-all
```

All repository modules passed.

```bash
cd rulebooks/dnd5e && golangci-lint run ./...
cd rulebooks/dnd5e/encounter && golangci-lint run ./...
cd rulebooks/dnd5e/resolution && golangci-lint run ./...
cd rulebooks/dnd5e/session && golangci-lint run ./...
```

All four changed modules reported `0 issues`.

The repository-wide hard-cut census found none of the removed compilers,
runtime monster-action types/configs, executable action lifecycle topics, or
legacy action package files. It also confirmed that `combat/actions` has no
`go.mod`, no committed `replace` directive exists, `git diff --check` is clean,
and `scripts/check-decisions.sh` reports all 47 ADRs summarized.

Each implementation PR's GitHub changed-module tests, API compatibility checks,
and optimized CI checks passed before squash. The main-branch auto-tag workflow
completed successfully for every released module.

## Deliberate deferrals

- Save-area, healing, and sequence profiles remain absent until their machines
  exist.
- Multiattack definitions remain removed; component attacks are available.
- The pre-existing goblin one-foot scimitar reach defect remains separate in
  [#1233](https://github.com/KirkDiggler/rpg-toolkit/issues/1233).
- Enriching replayable `StruckBody` with ordered damage components and
  advantage/disadvantage attribution requires an encounter carrier followed by
  a session consumer, tracked in
  [#1238](https://github.com/KirkDiggler/rpg-toolkit/issues/1238).

## Hard-cut result

No compatibility action representation survived. The repository contains no
`rulebooks/dnd5e/actions`, no `rulebooks/dnd5e/monster/actions`, no
`resolution.AttackProfile`, no `AttackFromCharacter`, no
`AttackFromMonsterAction`, no legacy action JSON translator, and no permanent
adapter. Producers author or assemble one shared data contract; resolution
interprets it.
