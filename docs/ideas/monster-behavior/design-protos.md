# Design: Proto Changes (rpg-api-protos)

## Overview

Proto changes needed to communicate monster turn results from server to client.

---

## 1. New Messages

### MonsterTurnResult

Captures everything that happened during one monster's turn:

```protobuf
// dnd5e/api/v1alpha1/combat.proto

// MonsterTurnResult captures what happened during a monster's turn
message MonsterTurnResult {
    // ID of the monster that took this turn
    string monster_id = 1;

    // Display name of the monster
    string monster_name = 2;

    // Actions the monster executed during its turn
    repeated MonsterExecutedAction actions = 3;

    // Path the monster moved during its turn (may be empty)
    repeated api.v1alpha1.Position movement_path = 4;
}
```

### MonsterExecutedAction

Details of a single action taken:

```protobuf
// MonsterExecutedAction represents one action taken during a monster turn
message MonsterExecutedAction {
    // ID of the action (e.g., "scimitar", "shortbow", "nimble-escape-hide")
    string action_id = 1;

    // Type of action for UI categorization
    string action_type = 2;  // "melee_attack", "ranged_attack", "stealth", "heal", "movement"

    // Target of the action (may be empty for self-targeting actions)
    string target_id = 3;

    // Whether the action succeeded
    bool success = 4;

    // Action-specific details
    oneof details {
        AttackResult attack_result = 5;
        HealResult heal_result = 6;
        StealthResult stealth_result = 7;
    }
}
```

### Supporting Messages

```protobuf
// HealResult for healing actions
message HealResult {
    int32 amount_healed = 1;
    int32 new_hp = 2;
    int32 max_hp = 3;
}

// StealthResult for hide actions
message StealthResult {
    int32 stealth_roll = 1;
    bool is_hidden = 2;
}
```

---

## 2. Updated Responses

### EndTurnResponse

Add monster turns to the response:

```protobuf
// dnd5e/api/v1alpha1/encounter.proto

message EndTurnResponse {
    // Current combat state (always returned)
    CombatState combat_state = 1;

    // Monster turns that executed before next player's turn
    // Empty if no monsters acted, or if next entity was also a monster
    // that is now the active turn
    repeated MonsterTurnResult monster_turns = 2;

    // Set if combat ended during monster turns (e.g., TPK)
    optional EncounterResult encounter_result = 3;
}

// EncounterResult indicates how combat ended
message EncounterResult {
    // Reason combat ended
    string reason = 1;  // "all_players_down", "all_monsters_dead", "fled"

    // Which side won (if applicable)
    string winner = 2;  // "players", "monsters", "none"
}
```

### DungeonStartResponse

Also include monster turns if monsters go first:

```protobuf
message DungeonStartResponse {
    // ... existing fields ...

    CombatState combat_state = X;

    // Monster turns that executed before first player's turn
    // Empty if a player won initiative
    repeated MonsterTurnResult monster_turns = Y;
}
```

### AttackResponse / MoveResponse / etc.

Ensure all combat responses include CombatState:

```protobuf
message AttackResponse {
    CombatState combat_state = 1;
    AttackResult result = 2;
}

message MoveResponse {
    CombatState combat_state = 1;
    api.v1alpha1.Position new_position = 2;
    int32 movement_remaining = 3;
}
```

---

## 3. Optional: Monster Display Data

If client needs to display monster info (HP bar, name, position):

```protobuf
// Monster as seen by the client
message MonsterView {
    string id = 1;
    string name = 2;
    string monster_type = 3;

    // Health (may be exact or approximate based on game rules)
    int32 current_hp = 4;
    int32 max_hp = 5;

    // Position in the room
    api.v1alpha1.Position position = 6;

    // Visual conditions (poisoned, hidden, etc.)
    repeated string visible_conditions = 7;
}
```

Then CombatState could include:

```protobuf
message CombatState {
    // ... existing fields ...

    // Monsters visible to players
    repeated MonsterView monsters = X;
}
```

---

## 4. Existing AttackResult

The existing AttackResult should work for monster attacks:

```protobuf
// Already exists in combat.proto
message AttackResult {
    bool hit = 1;
    int32 attack_roll = 2;
    int32 attack_total = 3;
    int32 target_ac = 4;
    int32 damage = 5;
    string damage_type = 6;
    bool critical = 7;
    // ... other fields
}
```

No changes needed - monsters reuse the same attack resolution as characters.

---

## File Summary

| File | Changes |
|------|---------|
| dnd5e/api/v1alpha1/combat.proto | Add MonsterTurnResult, MonsterExecutedAction, HealResult, StealthResult |
| dnd5e/api/v1alpha1/encounter.proto | Update EndTurnResponse, DungeonStartResponse with monster_turns |
| dnd5e/api/v1alpha1/encounter.proto | Add EncounterResult for combat end |
| dnd5e/api/v1alpha1/combat.proto | Optional: Add MonsterView |

---

## Client Usage Example

TypeScript client processing monster turns:

```typescript
async function endTurn(encounterId: string, entityId: string) {
    const response = await client.endTurn({ encounterId, entityId });

    // Update combat state
    setCombatState(response.combatState);

    // Animate monster turns sequentially
    for (const monsterTurn of response.monsterTurns) {
        await animateMonsterTurn(monsterTurn);
    }

    // Check for combat end
    if (response.encounterResult) {
        handleCombatEnd(response.encounterResult);
        return;
    }

    // Show "Your turn" if it's now a player's turn
    if (isPlayerTurn(response.combatState)) {
        showYourTurnPrompt();
    }
}

async function animateMonsterTurn(turn: MonsterTurnResult) {
    // Show monster name
    showMonsterAction(turn.monsterName);

    // Animate movement if any
    if (turn.movementPath.length > 1) {
        await animateMovement(turn.monsterId, turn.movementPath);
    }

    // Animate each action
    for (const action of turn.actions) {
        await animateAction(turn.monsterId, action);
    }

    // Brief pause between monsters
    await delay(500);
}
```

---

## Proto Location

These protos should live in:

```
rpg-api-protos/
└── proto/
    └── dnd5e/
        └── api/
            └── v1alpha1/
                ├── combat.proto      # MonsterTurnResult, MonsterExecutedAction
                └── encounter.proto   # Updated responses
```

---

## Migration Notes

- EndTurnResponse gains `monster_turns` field - backward compatible (empty array for old clients)
- DungeonStartResponse gains `monster_turns` field - backward compatible
- New message types don't affect existing clients until they opt-in
