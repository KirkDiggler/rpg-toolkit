---
name: dnd5e-feature-implementation
description: Use when implementing D&D 5e features, spells, or conditions - provides structure, definition of done, and combat log output pattern
---

# D&D 5e Feature/Spell Implementation Skill

## Working Together

**You have context about what's been built and why. I should ask before assuming, especially when building something new. Together we catch the "oh we already solved this" moments before they become bugs.**

Before implementing:
- Look at how similar things were done (check existing code first)
- Ask clarifying questions ("I see features uses X pattern - should this follow the same?")
- Don't assume - a quick back and forth catches mistakes early

## Discovering New Infrastructure

Sometimes a feature needs infrastructure that doesn't exist yet (new event types, new character methods, etc.).

**This is expected.** When you discover this:
1. Identify what's missing ("We need a HealingReceivedEvent but it doesn't exist")
2. Ask if this should be built as part of the feature or separately
3. If building it, add to your checklist

Common infrastructure that might need adding:
- New event types in `events/events.go`
- New Character subscriptions/handlers
- New chain types for modifier pipelines

## For Agents

When dispatched to implement a feature:

1. **Branch management** - Ask if you should create a new branch or if you're working on an existing one. Don't assume.

2. **Missing infrastructure** - If the feature needs something that doesn't exist (events, handlers, etc.), implement it as part of your work. Document what you added.

3. **When to ask vs proceed:**
   - Clear instructions with templates to follow → proceed
   - Ambiguous requirements or multiple valid approaches → ask
   - Infrastructure changes that might affect other code → ask

4. **Reporting done** - Include:
   - What files you created/modified
   - Test output (actual output, not just "tests pass")
   - Lint output
   - Any infrastructure you added beyond the feature itself

## Architecture: Who Uses What

```
┌─────────────────────────────────────────────────────────────────┐
│  rpg-dnd5e-web (Discord Activity)                               │
│  - Renders UI, calls API RPCs                                   │
│  - Never knows game rules, just displays what API returns       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  rpg-api (Game Server / Orchestrator)                           │
│  - Persists data (characters, encounters, features as JSON)     │
│  - Orchestrates: loads data → calls toolkit → saves results     │
│  - NEVER knows what "Rage" is - just loads/saves JSON blobs     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  rpg-toolkit/rulebooks/dnd5e (D&D 5e Rules)                     │
│  - Implements game rules (Rage adds +2 damage)                  │
│  - Data interfaces: ToJSON() / LoadJSON() for persistence       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  rpg-toolkit/core, events, dice, mechanics                      │
│  - Generic infrastructure (not D&D specific)                    │
└─────────────────────────────────────────────────────────────────┘
```

## Data Contract

The game server needs a **distinct data contract** for persistence:
- Every feature/condition needs a `Data` struct with `json` tags
- `ToJSON()` serializes runtime state to the data contract
- `loadJSON()` hydrates from the data contract
- `loader.go` routes by `core.Ref` to the correct type

**Look at `rage.go` and `features/loader.go` as the template.**

## Implementation Checklist

### Core Implementation
- [ ] Feature struct + `FeatureBehavior` interface
- [ ] Condition struct + `ConditionBehavior` interface (if applicable)
- [ ] Data struct + `ToJSON()` / `loadJSON()`
- [ ] Register in `loader.go`

### Event Integration
- [ ] Condition subscribes to appropriate chain (AttackChain, DamageChain)
- [ ] Modifier added at correct stage (StageFeatures, StageConditions)
- [ ] Feature publishes ConditionAppliedEvent on activation
- [ ] Condition cleans up subscriptions on Remove()

### Tests
- [ ] Unit tests (CanActivate, Activate, chain modification, JSON round-trip)
- [ ] Integration test in `combat/integration_test.go` with combat log output
- [ ] Example test showing event subscription pattern

## Definition of Done

A feature is COMPLETE when:
1. `go test ./...` passes
2. `golangci-lint run ./...` passes
3. Integration test prints combat log (run with `-v`)
4. JSON round-trips correctly (game server can persist/reload)
5. All data is trackable (dice rolls, rerolls, modifiers)

**You MUST run tests and lint before reporting done.** Include actual output in your report. If lint fails, use the `lint-refactoring` skill to fix issues properly - don't just suppress warnings.

## Combat Log Output Pattern

Integration tests should print readable output showing what the game server would log:

```
=== [Feature Name] Integration Test ===
Attacker: Grog (Level 5 Barbarian, STR +3)
Defender: Goblin Scout (AC 13)

→ Grog activates [Feature]!
  ✓ [Condition] applied ([effect description])

→ Grog attacks Goblin Scout
  Attack roll: 1d20(15) + STR(3) + Prof(2) = 20
  vs AC 13 → HIT!

  Damage breakdown:
    1d12 weapon damage: [8]
    + STR modifier: 3
    + [Feature] bonus: 2
  = Total damage: 13
```

For rerolls (Great Weapon Fighting style):
```
  Damage breakdown:
    2d6 weapon damage: [1, 4]
      → Reroll die 0: was 1, now 3
    Final rolls: [3, 4] = 7
```

## Running Tests

```bash
cd rpg-toolkit/rulebooks/dnd5e
go test -v ./combat -run TestCombatIntegrationSuite
```
