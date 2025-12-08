# Monster Behavior - Brainstorm

> All ideas welcome here - practical, dreamy, or a little crazy.

## The Trigger

We have a goblin in a demo room that does nothing but get beat up. Can we build something simple and extensible that makes it fight back?

## Goals

**Primary (B):** Goblin picks closest enemy, moves toward it, attacks, and uses Nimble Escape (Disengage or Hide as bonus action).

**Stretch (C):** Goblin chooses between Disengage and Hide based on conditions — HP threshold, allies nearby, terrain, etc.

---

## Architecture Reminder

From our existing design:

```
rpg-api (orchestrator) → dnd5e rulebook (game logic) → toolkit (infrastructure)
                              ↑
                        Goblin behavior lives here
```

- **Toolkit** = infrastructure only (behavior interfaces, state machines, behavior trees, memory, perception)
- **Rulebook** = game implementation (what does a D&D goblin actually do?)
- **API** = data orchestrator (stores state, coordinates execution)

---

## Ideas Dump

### The Simple Loop
What's the absolute minimum behavior loop?

```
1. Perceive: Who can I see?
2. Decide: Pick a target (closest enemy)
3. Act: Move toward target, attack if in range
4. React: Use Nimble Escape (bonus action)
```

### Targeting Strategies
- **Closest** — Simple distance calculation
- **Weakest** — Lowest current HP
- **Squishiest** — Lowest AC or caster-looking
- **Threat** — Who hurt me most recently?
- **Pack tactics** — Who are my allies already attacking?

For v1, "closest" is probably right. But the system should support swapping strategies.

### Nimble Escape Decision
Goblin's Nimble Escape: "The goblin can take the Disengage or Hide action as a bonus action on each of its turns."

When to Disengage vs Hide?
- **Disengage** — When surrounded, when low HP and need to retreat safely
- **Hide** — When there's cover nearby, when at range and want advantage on next attack

This is where it gets interesting. The goblin needs to "know" things:
- Am I threatened by multiple enemies?
- Is there cover nearby?
- Am I trying to retreat or set up for next turn?

### Memory Ideas
What should a goblin remember?
- Who attacked me last (for grudge/threat targeting)
- Where is cover (for Hide decisions)
- Where are my allies (for pack tactics)
- Have I been hurt this fight (for flee threshold)

### Behavior Patterns
From the existing ADR, we support multiple paradigms:

**State Machine (simple)**
```
Idle → Spotted Enemy → Approach → Attack → Use Bonus Action → End Turn
```

**Behavior Tree (flexible)**
```
Selector
├── Sequence [Combat]
│   ├── HasTarget?
│   ├── MoveToward
│   ├── Attack
│   └── NimbleEscape
└── Sequence [Search]
    ├── NoTarget?
    └── Patrol
```

**Utility AI (nuanced)**
- Score each action based on situation
- Highest score wins
- Good for "should I Disengage or Hide?" decisions

### The Dreamy Stuff

**Personality traits** — Cowardly goblins flee earlier. Brave ones hold ground.

**Pack communication** — Goblins call out targets to each other.

**Learning** — Goblin remembers "that guy in armor hits hard" from previous rounds.

**Tactical retreats** — Goblin falls back to chokepoint when outnumbered.

**Ambush behavior** — Goblin hides until optimal moment.

### The Crazy Stuff

**Morale system** — Goblins flee when allies die. Worg arrival boosts morale.

**Fear of fire** — Goblins specifically avoid characters who've used fire.

**Loot awareness** — Goblin grabs shiny object and runs.

**Surrender** — At low HP, goblin drops weapon and begs for mercy.

---

## Key Design Decision: Monsters Are Entities

**Monsters use the same systems as characters.** They're not special cases.

| System | Characters | Monsters |
|--------|------------|----------|
| Action Economy | 1/1/1 | Same |
| Features | Rage, Second Wind | Nimble Escape, Pack Tactics |
| Conditions | Poisoned, Blessed | Same — potions, poison, spells work identically |
| Event Subscriptions | Modify/cancel via events | Same |

This means:
- A healing potion works on a monster the same as a character
- Poison applies conditions the same way
- Features subscribe to events and modify behavior

## Actions as Rich Objects

Actions aren't just "attack with weapon" — they're **things the monster can do** with behavioral metadata:

```
Action: Scimitar
  - Cost: Action
  - Type: MeleeAttack
  - Range: 5 ft (reach)
  - Damage: 1d6+2 slashing
  - Trigger: Target in melee range

Action: Shortbow
  - Cost: Action
  - Type: RangedAttack
  - Range: 80/320 ft
  - Damage: 1d6+2 piercing
  - Trigger: Target at range, clear line of sight

Action: Drink Healing Potion
  - Cost: Action
  - Type: Consumable
  - Effect: Heal 2d4+2
  - Trigger: HP < 30%
  - Uses: 1 (limited)

Action: Nimble Escape (Disengage)
  - Cost: Bonus Action
  - Type: Movement
  - Effect: No opportunity attacks this turn
  - Trigger: Threatened by multiple enemies OR retreating

Action: Nimble Escape (Hide)
  - Cost: Bonus Action
  - Type: Stealth
  - Effect: Hidden condition, advantage on next attack
  - Trigger: Cover available, not in melee
```

**Behavior becomes:** "Look at available actions, check which triggers are satisfied, pick the best one based on priority/utility."

## Answered Questions

1. **Action Economy** — Same as characters. Monsters get 1/1/1, features can grant extras.

2. **Features** — Special abilities ARE features. Nimble Escape is a Feature that the goblin has.

3. **Conditions** — Work identically. Healing potions, poison, spell effects all apply.

4. **Action Selection** — Use utility scoring. Each action gets a score based on situation, highest valid score wins. State machines can be integrated later as a proper toolkit tool.

## Action Selection: Utility Scoring

**The Behavior Loop:**
```
1. Perceive: What's around me? (targets, cover, threats)
2. Evaluate: Score each available action based on current situation
3. Select: Pick highest-scoring valid action
4. Execute: Do the action, consume resources
5. Repeat: If resources remain (bonus action), go back to step 2
```

**Example Scoring for Goblin:**

| Action | Base Score | Modifiers |
|--------|------------|-----------|
| Scimitar | 50 | +20 if target adjacent |
| Shortbow | 50 | +20 if target at range, -100 if adjacent (disadvantage) |
| Nimble Escape (Disengage) | 30 | +40 if HP < 50%, +20 per adjacent enemy |
| Nimble Escape (Hide) | 30 | +30 if cover available, +20 if has ranged attack |
| Drink Potion | 20 | +60 if HP < 30%, -100 if HP > 70% |

**Example Calculation:**
Goblin at 20% HP, adjacent to one enemy, cover nearby:
- Scimitar: 50 + 20 = **70**
- Disengage: 30 + 40 + 20 = **90** ← Winner (run away!)
- Hide: 30 + 30 + 20 = 80 (but can't hide while adjacent)
- Potion: 20 + 60 = 80

The numbers are tunable. The *system* is what matters.

## Open Questions

1. **Pathfinding** — Where does "move toward target" actually happen? Toolkit infrastructure or rulebook?

2. **Turn Structure** — Does behavior run once per turn? Can it react mid-turn to events?

3. **Perception Range** — How far can the goblin "see"? Comes from senses (darkvision 60 ft, passive perception 9).

4. **Cover Detection** — For Hide decisions, how do we know if cover is available? Spatial queries?

5. **Action Selection** — When multiple actions are valid, how do we pick? Utility scoring? Priority order?

6. **Action Template Format** — How do we define actions? Struct? Proto? JSON like dnd5eapi?

---

## What's Next

1. Write use cases (concrete scenarios with the goblin)
2. Answer the open questions
3. Design the interfaces needed
4. Create implementation issues

---

## rpg-api Integration

> Brainstormed after toolkit implementation was complete. Focus: how does the game server orchestrate monster turns?

### The Core Flow

When a player ends their turn:
1. Server advances to next entity in initiative
2. If monster → auto-execute `TakeTurn`, record result, advance again
3. Repeat until reaching a player's turn
4. Return `CombatState` (pointing to player) + all `MonsterTurnResults`

**Key insight:** Streaming vs batch doesn't change architecture. UI can animate through batch results sequentially ("goblin moves... goblin attacks..."). Streaming is a future optimization, not a design change.

### Monster Storage

**Decision: Monsters live in the encounter.**

Monsters are ephemeral - they spawn, fight, die. Characters persist across encounters. Different lifecycles = different storage.

```go
Encounter {
    ID: "enc-1"
    Monsters: []MonsterData  // stored inline, loaded with encounter
    Characters: []string     // just IDs, loaded from character repo
}
```

No separate monster repository needed.

### Perception Building

**Decision: Toolkit builds perception from Room/GameContext.**

The game server stays thin - just passes the room. Toolkit does the work:

```go
// rpg-api orchestrator (thin)
monster.TakeTurn(ctx, &TurnInput{
    Bus:           bus,
    ActionEconomy: economy,
    GameCtx:       &GameContext{Room: room},
    Roller:        roller,
})

// Toolkit builds perception internally
func (m *Monster) TakeTurn(ctx, input) {
    perception := m.buildPerception(input.GameCtx.Room)
    // ... behavior loop
}
```

**Why:** Less game server responsibility. GameContext is already the pattern (sneak attack will need spatial too). Room should be mockable in tests - if not, fix it.

### Enemy Detection

**Decision: Entity type for MVP.**

Simple rule: `character` type = enemy, `monster` type = ally.

```go
if entity.GetType() == "character" {
    perception.Enemies = append(...)
}
```

Faction system is future work (charmed monsters, PvP, monster infighting).

### CombatState Always Returned

**Decision: Every combat call returns CombatState.**

No separate `next_entity_id` fields. CombatState is the source of truth:

```go
EndTurnResponse {
    CombatState    // Always returned - includes whose turn it is
    MonsterTurns   // What happened before next player turn
}

AttackResponse {
    CombatState    // Always returned
    Result         // Attack outcome
}
```

Client always knows current state from CombatState.

### Orchestrator Structure

**Decision: Extract monster turn execution to helper method.**

Keeps `EndTurn` focused:

```go
func (o *Orchestrator) EndTurn(ctx, input) (*EndTurnOutput, error) {
    o.advanceTurn(...)
    monsterResults := o.executeMonsterTurns(ctx, encounter)
    return &EndTurnOutput{
        CombatState:  encounter.CombatState,
        MonsterTurns: monsterResults,
    }
}

func (o *Orchestrator) executeMonsterTurns(ctx, encounter) []MonsterTurnResult {
    var results []MonsterTurnResult
    for isMonsterTurn(encounter) {
        monster := loadMonster(encounter, currentEntityID)
        result := monster.TakeTurn(ctx, buildTurnInput(...))
        results = append(results, result)
        advanceToNextEntity(encounter)
    }
    return results
}
```

### Edge Cases

Captured for use cases doc:

1. **Dead monster's turn** → Skip, remove from initiative order
2. **Player drops to 0 HP** → Complete current monster turn, then player's turn for death saves
3. **All players down** → Encounter stopped event, run is over
4. **Monster kills monster** (future) → Dead monsters removed from initiative

### Proto Changes Needed

New messages:
```protobuf
message MonsterTurnResult {
    string monster_id = 1;
    repeated MonsterExecutedAction actions = 2;
    repeated Position movement_path = 3;
}

message MonsterExecutedAction {
    string action_id = 1;
    string action_type = 2;  // "melee_attack", "ranged_attack", etc.
    string target_id = 3;
    bool success = 4;
    oneof details {
        AttackResult attack_result = 5;
        HealResult heal_result = 6;
    }
}
```

Update EndTurnResponse:
```protobuf
message EndTurnResponse {
    CombatState combat_state = 1;
    repeated MonsterTurnResult monster_turns = 2;
}
```

### Open Questions (API-side)

1. **Room mock pattern** — How do we stub Room in orchestrator tests? Need to verify this is clean.
2. **Event bus per encounter** — Is there one bus per encounter, or shared? Monster needs to wire to it.
3. **Monster data in proto** — Do we need `Monster` proto message for client display (HP, name, position)?

---

## References

- [ADR-0016: Behavior System Architecture](../adr/0016-behavior-system-architecture.md)
- [Journey 017: Encounter System Design](../journey/017-encounter-system-design.md)
- [Encounter System Implementation Plan](../archive/planning/encounter-system-implementation.md)
- [Rulebook-Behavior Integration](../archive/planning/rulebook-behavior-integration.md)
