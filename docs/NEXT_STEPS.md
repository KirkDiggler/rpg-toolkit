# Next Steps - Combat Vertical Slice

**Last updated**: 2025-11-26
**Status**: Phase 1 Complete ✅, Feature Implementation Workflow Established ✅

## What We Just Finished

✅ **PR #357 - Feature Implementation Workflow**
- Established skill for implementing D&D 5e features/spells/conditions
- Created `.claude/skills/dnd5e-feature-implementation/SKILL.md`
- Defines standard structure, definition of done, combat log output pattern
- Integration tests print readable combat log for debugging/verification

✅ **PR #334 - Combat Integration Tests** (Issue #324)
- Created comprehensive integration test suite
- Validates event-driven combat system works end-to-end
- Tests: Rage + Attack combo with hit/miss/crit scenarios
- Descriptive narrative output (perfect for future combat log - Issue #335)
- Character loading from Data (simulates DB pattern)

✅ **PR #333 - Character Condition Tracking** (Issue #332)
- Characters can receive and track conditions
- Conditions subscribe to events and modify combat
- Full lifecycle: Apply → Subscribe → Modify → Remove
- Architecture doc: `docs/journey/045-circular-dependencies-events-subpackage.md`

## Where We Are: Combat Vertical Slice

**Parent Issue**: #322 - Combat Vertical Slice: Attack with Event-Driven Modifiers

### Phase 1: Toolkit Combat Foundation ✅ COMPLETE
- [x] Monster entity (simple Goblin)
- [x] Attack resolution with event chains
- [x] Rage wired to attack events
- [x] Integration tests proving it works

### Phase 2: API Orchestration ⏭️ NEXT
**Issue**: #325 - Phase 2: API Attack endpoint orchestration

**What to build**:
1. **Attack RPC endpoint** in `rpg-api/internal/handlers/dnd5e/v1alpha1/`
   - Load character from DB (includes active features)
   - Load monster from encounter state
   - Call `combat.ResolveAttack()` from toolkit
   - Update monster HP in encounter
   - Save state and return result

2. **Monster storage** in `rpg-api/internal/repositories/encounters/`
   - Track monster HP in encounter data
   - Remove dead monsters

**Key Pattern**: API orchestrates, toolkit does rules

### Phase 3: UI Display ⏭️ FUTURE
**Issue**: #326 - Phase 3: UI display attack results with Rage bonus

**What to build**:
- Call Attack RPC from Discord activity
- Display damage breakdown showing Rage bonus
- Update monster HP display

## Important Files for Phase 2

### Toolkit (reference implementation)
- `rulebooks/dnd5e/combat/attack.go` - Attack resolution API
- `rulebooks/dnd5e/combat/integration_test.go` - How it all works
- `rulebooks/dnd5e/combat/integration.md` - Test documentation

### API (where you'll work next)
- `rpg-api/internal/handlers/dnd5e/v1alpha1/` - gRPC handlers
- `rpg-api/internal/orchestrators/dnd5e/` - Business logic layer
- `rpg-api/internal/repositories/encounters/` - Encounter persistence

### Proto (define the API contract)
- `rpg-api-protos/dnd5e/api/v1alpha1/encounter.proto` - Add Attack RPC

## Key Architectural Decisions

### Events Subpackage (Journey 045)
- `dnd5e/events/` is foundational layer
- `dnd5e/conditions/`, `dnd5e/features/`, `dnd5e/character/` import events
- No circular dependencies
- Features create conditions and publish them in events

### Condition Lifecycle
1. Feature creates condition with all data
2. Feature publishes `ConditionAppliedEvent`
3. Character receives event and calls `condition.Apply(ctx, bus)`
4. Condition subscribes to combat events (e.g., DamageChain)
5. Condition modifies events as they flow
6. Eventually: `condition.Remove(ctx, bus)` unsubscribes

### LoadFromData Pattern
Characters are loaded from `character.Data` struct (simulates DB):
```go
data := &character.Data{
    ID: "char-1",
    Features: []json.RawMessage{...},
    // ...
}
char, err := character.LoadFromData(ctx, data, bus)
```

This is the pattern rpg-api will use when loading from Postgres.

## Open Issues to Consider

- **#336** - Update golangci-lint: increase line length to 140 (quick win)
- **#335** - Combat Log system (future enhancement using integration test narrative)
- **#331** - Pattern: Streamline entity targeting checks (code quality)
- **#329** - Regenerate core/mock - GetType signature mismatch (bug fix)

## Game Server Next Steps

Once Phase 2 & 3 are done, you'll have:
- ✅ Toolkit with combat rules
- ✅ API orchestrating combat
- ✅ UI displaying results

Then the game server becomes:
- **Real-time combat orchestration** via Discord Activity
- **Encounter state management** (who's in combat, turn order)
- **WebSocket updates** for live combat feed
- **Combat log streaming** (Issue #335's narrative output)

## Quick Reference Commands

```bash
# Run integration tests with narrative output
cd rpg-toolkit/rulebooks/dnd5e
go test -v ./combat -run TestCombatIntegrationSuite

# Run specific test
go test -v ./combat -run TestCombatIntegrationSuite/TestBarbarianRageAddsDamageOnHit

# Check what's next
gh issue view 325  # Phase 2 API work
gh issue view 322  # Parent issue with full plan
```

## Breadcrumbs for Future You

1. **Integration tests are the spec** - Look at `combat/integration_test.go` to see exactly how combat works
2. **Events are the glue** - Everything communicates via EventBus
3. **Conditions are self-contained** - They manage their own subscriptions
4. **API pattern**: Load → Call toolkit → Save → Return
5. **Combat log ready** - The test output format is exactly what Issue #335 needs

## What's Working Right Now

You can literally load a barbarian, activate rage, and attack a goblin in a test. The event system properly adds the +2 rage damage. The architecture is proven.

**Next**: Wire it through the API so Discord can trigger it.

---

## Architecture Overview

Understanding who uses what is critical for implementing features correctly.

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

---

## GitHub Organization

### Projects (cross-repo tracking)
- **Phase 1: Level 1 Combat Encounters** - Current focus
- **RPG Ecosystem Roadmap** - Big picture planning
- **RPG Toolkit Development** - Toolkit-specific work

### Issue Tracking Pattern
Each feature typically spans multiple repos:
1. **rpg-toolkit** - Feature implementation (rules logic)
2. **rpg-api** - RPC endpoint + orchestration
3. **rpg-dnd5e-web** - UI to trigger and display

Link issues across repos and track in Projects.

---

## Feature Implementation Workflow

We've established a pattern for implementing D&D 5e features efficiently using focused agents.

**Skill:** `.claude/skills/dnd5e-feature-implementation/SKILL.md`

### The Workflow

1. **Main context** stays high-level - tracks what needs to be done, reviews results
2. **Agents** do the implementation - use the `dnd5e-feature-implementation` skill
3. **Integration tests** prove it works - print combat log output with `-v`

### Definition of Done

A feature is COMPLETE when:
- [ ] Unit tests pass
- [ ] Lint passes
- [ ] Integration test prints combat log (run with `-v`)
- [ ] Example test documents event subscription pattern
- [ ] All data is trackable (dice rolls, rerolls, modifiers)
- [ ] JSON round-trips correctly (game server can persist/reload)

### Dispatching an Agent

```
Implement [Feature Name] for D&D 5e following the dnd5e-feature-implementation skill.

Requirements:
- [Specific behavior from D&D 5e rules]
- [Level scaling if any]

Use rage.go and raging.go as templates.
Integration test should print combat log showing all tracked data.
Ensure ToJSON/loadJSON round-trip works for game server persistence.
```

### Open Feature Issues (rpg-toolkit)

| Issue | Feature | Class | Priority |
|-------|---------|-------|----------|
| #352 | Fighting Style | Fighter | High |
| #353 | Second Wind | Fighter | High |
| #354 | Unarmored Defense | Monk | High |
| #355 | Martial Arts | Monk | High |

### Running Integration Tests

```bash
# See all combat log output
cd rpg-toolkit/rulebooks/dnd5e
go test -v ./combat -run TestCombatIntegrationSuite

# Run specific feature test
go test -v ./combat -run TestCombatIntegrationSuite/TestBarbarianRageAddsDamageOnHit
```

---

🤖 Last updated by Claude Code session ending 2025-11-26
