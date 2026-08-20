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

## Review-fix round

- Migrated the encounter adapter and its callers/tests to the canonical
  `StrikeResolver`, `StrikeInput`, `StrikeOutcome`, and phased Strike names.
- Removed encounter's deleted `WeaponForActionRef`/`MeleeWeaponProvider`
  callers; encounter range validation now stays at the default boundary while
  weapon semantics are compiled by the rulebook Strike input.
- Restored the eight condition behavior/persistence suites and migrated their
  fixtures to marked damage components. `go test ./conditions` and
  `go test ./monster` pass.
- Strike now publishes `PostAttackRollChain` for hit and miss outcomes, with a
  focused subscriber regression covering the published roll snapshot.
- The architecture guard now scans production declarations, calls, and type
  references (including deleted weapon-selection/provider names).

Verification for this round:

```text
bash scripts/check-no-legacy-attack.sh       # PASS
git diff --check                             # PASS
go test ./conditions ./monster               # PASS
```

The requested encounter compile check remains blocked before package
compilation by the published module graph: `seed_monsters.go` resolves the
published `damage` package, which lacks local `damage.Damage`, `damage.Validate`,
and `damage.AddsAttackAbilityModifier`. The exact command was:

```text
cd encounter && go test ./... -run '^$' -count=1
```

It fails at `seed_monsters.go:447`, `:477`, and `:483`; no additional
encounter compiler diagnostics are reachable until the local module graph is
used.
