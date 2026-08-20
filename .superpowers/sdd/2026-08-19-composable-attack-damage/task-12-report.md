# Task 12 report — delete the replaced combat resolution stack

Status: DONE (scoped implementation; encounter-wide integration remains for
the release cutover)

## Changes

- Deleted the combat package's monolithic and phased attack resolvers and
  their obsolete attack, breakdown, two-weapon, integration, and turn-manager
  callers/tests.
- Deleted encounter's legacy `CombatResolver`/`PhasedCombatResolver` adapter
  surface and tests, and removed the old opportunity-attack resolver call from
  movement.
- Kept generic `DealDamage`, `ApplyDamage`, and `FinalDamage` contracts while
  making chain resolution private to generic damage flow.
- Removed event-wide `DamageChainEvent` primary damage metadata. Conditions now
  read the canonical marked `DamageComponent` for primary dice/type.
- Updated the living combat architecture overview and added executable
  `scripts/check-no-legacy-attack.sh`.

## TDD / verification evidence

RED:

```text
bash scripts/check-no-legacy-attack.sh
```

Failed before deletion and listed `AttackInput`, `ResolveAttackHitInput`,
`ApplyAttackOutcomeInput`, and the legacy resolver functions.

GREEN / focused checks:

```text
bash scripts/check-no-legacy-attack.sh                         # PASS
git diff --check                                               # PASS
env GOCACHE=/tmp/rpg-toolkit-task12-go-cache go test ./combat ./conditions ./monster/... -count=1  # PASS
```

The resolution suite requires the temporary local multi-module workspace
used by Tasks 8–10; the published dependency graph still predates the local
canonical damage types. The encounter suite could not be started in this
sandbox because its published dependencies required unavailable module-cache
downloads. Those release-order dependency checks remain Task 13 scope.

## Concerns

Top-level encounter callers that previously depended on the deleted resolver
adapter are intentionally removed ahead of the Task 13 dependency publication
and replacement integration. No compatibility attack resolver or event-wide
damage mirror remains in production.
