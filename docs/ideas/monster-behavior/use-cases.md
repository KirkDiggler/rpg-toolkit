# Monster Behavior - Use Cases

Concrete scenarios showing the behavior system working end-to-end.

---

## Use Case 0: Monster Joins Initiative

**Scenario:** Combat starts, goblin is added to turn order.

**Setup:**
- Player character (DEX +2) in room
- Goblin (DEX +2) spawned in room
- Combat begins

**Initiative Roll:**
```go
// Orchestrator creates entities map
entities := map[core.Entity]int{
    initiative.NewParticipant(playerID, "character"): 2,  // DEX +2
    initiative.NewParticipant(goblinID, "monster"):   2,  // DEX +2
}

// Roll initiative
rolls := initiative.RollForOrder(entities, roller)
// Result: [{goblin, 18}, {player, 12}]  (goblin rolled higher)

// Create tracker
tracker := initiative.New([]core.Entity{goblin, player})
```

**Turn Order Stored:**
```json
{
  "initiative_data": {
    "order": [
      {"id": "goblin-1", "type": "monster"},
      {"id": "player-1", "type": "character"}
    ],
    "current": 0,
    "round": 1
  },
  "movement_remaining": 30
}
```

**Result:** Goblin goes first. When combat starts, orchestrator detects monster turn and calls `TakeTurn`.

---

## Use Case 1: Basic Melee Attack (Complete Turn Flow)

**Scenario:** Goblin's turn - moves toward player and attacks.

**Initial State:**
- Goblin at position (0, 5)
- Player at position (6, 5) — 30 feet away
- Goblin has full HP (7/7)
- Fresh action economy: Action=1, BonusAction=1, Movement=30ft

**Turn Execution:**

```go
// Orchestrator detects monster turn
if output.EntityType == "monster" {
    // Load monster
    monster, _ := monster.LoadFromData(ctx, goblinData, bus)

    // Fresh action economy
    economy := combat.NewActionEconomy()  // 1 action, 1 bonus, 30ft movement

    // Execute turn
    result, _ := monster.TakeTurn(ctx, &TurnInput{
        Bus:           bus,
        ActionEconomy: economy,
        GameCtx:       &GameContext{Room: room},
        Roller:        roller,
    })
}
```

**Inside TakeTurn:**

```
1. BUILD PERCEPTION from room:
   - MyPosition: (0, 5)
   - Enemies: [{player, pos:(6,5), dist:30ft, adjacent:false}]

2. FIRST LOOP - Score actions:
   - Scimitar: base=50, not adjacent → Score=50 (but CanActivate fails - out of range)
   - Shortbow: base=50, +20 (target at range) → Score=70

3. SELECT: Shortbow wins with 70

4. EXECUTE Shortbow:
   - CanActivate: target exists, in range (30ft < 80ft) ✓
   - But wait - goblin prefers melee. Let's say behavior moves first.

   Actually, let's reconsider:
   - Movement is part of action execution, not scored separately
   - Scimitar action can move toward target then attack

5. REVISED - Scimitar with movement:
   - CanActivate checks: target not adjacent
   - Action uses GameCtx.Room to pathfind and move
   - Move 25ft → now at (5, 5), adjacent
   - Attack: roll d20+4 vs AC
   - economy.UseAction() → Action=0

6. SECOND LOOP - Score remaining:
   - ActionEconomy: Action=0, BonusAction=1, Movement=5ft
   - Scimitar: costs Action, can't afford
   - Shortbow: costs Action, can't afford
   - Nimble Escape (Disengage): base=30, HP fine, 1 enemy → Score=50
   - Nimble Escape (Hide): base=30, no cover → Score=30

7. SELECT: Disengage wins with 50 (marginal - goblin might skip)
   - Actually at full HP, neither scores high
   - No action taken

8. THIRD LOOP - No affordable actions, exit loop
```

**Action Economy Tracking:**
```
Start:     Action=1, Bonus=1, Movement=30ft
After move: Action=1, Bonus=1, Movement=5ft
After attack: Action=0, Bonus=1, Movement=5ft
End:       Action=0, Bonus=1, Movement=5ft
```

**TurnResult:**
```go
&TurnResult{
    MonsterID: "goblin-1",
    Actions: []ExecutedAction{
        {
            ActionID:   "scimitar",
            ActionType: TypeMeleeAttack,
            TargetID:   "player-1",
            Success:    true,
            Details:    &AttackResult{Hit: true, Damage: 5},
        },
    },
    Movement: []Position{{0,5}, {5,5}},  // Path taken
}
```

**End State:**
- Goblin at (5, 5), adjacent to player
- Player took 5 slashing damage
- Goblin's turn ends, orchestrator calls EndTurn → advances to player

---

## Use Case 2: Ranged Attack Preference

**Scenario:** Goblin starts at range and prefers to stay at range.

**Initial State:**
- Goblin at position (0, 0)
- Player at position (8, 0) — 40 feet away
- Goblin has full HP (7/7)
- Cover (pillar) at position (2, 2)
- Fresh action economy: Action=1, BonusAction=1, Movement=30ft

**Inside TakeTurn:**

```
1. BUILD PERCEPTION:
   - Enemies: [{player, dist:40ft, adjacent:false}]
   - Cover: [{pillar, pos:(2,2), dist:~15ft}]

2. FIRST LOOP - Score actions:
   - Scimitar: base=50, not adjacent → CanActivate would need to move 35ft (out of range)
   - Shortbow: base=50, +20 (target at range, not adjacent) → Score=70

3. SELECT: Shortbow wins

4. EXECUTE Shortbow:
   - No movement needed (already at range)
   - Attack: roll d20+4 vs AC
   - economy.UseAction() → Action=0

5. SECOND LOOP - Score remaining:
   - ActionEconomy: Action=0, BonusAction=1, Movement=30ft
   - Nimble Escape (Disengage): base=30, no adjacent enemies → Score=30
   - Nimble Escape (Hide): base=30, +30 (cover nearby) +20 (has ranged) → Score=80

6. SELECT: Hide wins with 80

7. EXECUTE Hide:
   - Move toward pillar (15ft) → Movement=15ft remaining
   - Apply "hidden" condition
   - economy.UseBonusAction() → BonusAction=0
```

**Action Economy Tracking:**
```
Start:        Action=1, Bonus=1, Movement=30ft
After attack: Action=0, Bonus=1, Movement=30ft
After move:   Action=0, Bonus=1, Movement=15ft
After hide:   Action=0, Bonus=0, Movement=15ft
End:          Action=0, Bonus=0, Movement=15ft
```

**TurnResult:**
```go
&TurnResult{
    MonsterID: "goblin-1",
    Actions: []ExecutedAction{
        {ActionID: "shortbow", ActionType: TypeRangedAttack, TargetID: "player-1", Success: true},
        {ActionID: "nimble-escape-hide", ActionType: TypeStealth, TargetID: "goblin-1", Success: true},
    },
    Movement: []Position{{0,0}, {2,2}},
}
```

**End State:**
- Goblin hidden behind pillar at (2, 2)
- Player possibly took 1d6+2 piercing damage
- Next turn: Goblin attacks with advantage (if still hidden)

---

## Use Case 3: Nimble Escape (Disengage) — Tactical Retreat

**Scenario:** Goblin is wounded and surrounded, needs to escape.

**Initial State:**
- Goblin at position (5, 5) with 2 HP remaining (2/7 = 28%)
- Player A at (4, 5) — adjacent
- Player B at (6, 5) — adjacent
- Exit/safety at (0, 5)
- Fresh action economy: Action=1, BonusAction=1, Movement=30ft

**Inside TakeTurn:**

```
1. BUILD PERCEPTION:
   - MyPosition: (5, 5)
   - Enemies: [{playerA, dist:5ft, adjacent:true}, {playerB, dist:5ft, adjacent:true}]
   - HP: 2/7 (28%)

2. FIRST LOOP - Score actions:
   - Scimitar: base=50, +20 (adjacent) → Score=70
   - Shortbow: base=50, -100 (adjacent enemies) → Score=-50
   - Nimble Escape (Disengage): base=30, +40 (HP<50%), +40 (2 adjacent × 20) → Score=110 ★
   - Nimble Escape (Hide): base=30, -100 (adjacent) → Score=-70

3. SELECT: Disengage wins with 110 — survival instinct!

4. EXECUTE Disengage:
   - Apply "disengaged" condition (no opportunity attacks this turn)
   - economy.UseBonusAction() → BonusAction=0
   - Note: Goblin can now move without triggering OAs

5. SECOND LOOP - Score remaining:
   - ActionEconomy: Action=1, BonusAction=0, Movement=30ft
   - Scimitar: base=50, still adjacent → Score=70
   - Shortbow: still adjacent → Score=-50
   - Drink Potion: base=20, +60 (HP<30%) → Score=80 ★

6. SELECT: Drink Potion wins with 80

7. EXECUTE Drink Potion:
   - Move away first: 30ft toward exit → now at (0, 5)
   - Movement=0ft remaining
   - Consume potion, heal 2d4+2 → regain 6 HP
   - economy.UseAction() → Action=0

8. THIRD LOOP - No actions remaining, exit
```

**Action Economy Tracking:**
```
Start:           Action=1, Bonus=1, Movement=30ft
After disengage: Action=1, Bonus=0, Movement=30ft
After move:      Action=1, Bonus=0, Movement=0ft
After potion:    Action=0, Bonus=0, Movement=0ft
End:             Action=0, Bonus=0, Movement=0ft
```

**TurnResult:**
```go
&TurnResult{
    MonsterID: "goblin-1",
    Actions: []ExecutedAction{
        {ActionID: "nimble-escape-disengage", ActionType: TypeMovement, Success: true},
        {ActionID: "drink-potion", ActionType: TypeHeal, Success: true, Details: &HealResult{Amount: 6}},
    },
    Movement: []Position{{5,5}, {0,5}},
}
```

**End State:**
- Goblin retreated 30 ft to (0, 5), now safe
- Goblin healed from 2 HP to 8 HP (back to full!)
- Players must chase or use ranged attacks
- No opportunity attacks triggered (Disengage condition active)

---

## Use Case 4: Nimble Escape (Hide) — Ambush Setup

**Scenario:** Goblin sets up for an ambush attack with advantage.

**Initial State:**
- Goblin at position (5, 5) with full HP
- Player at position (10, 5) — 25 feet away
- Large rock (cover) at position (4, 3)
- Goblin has not been detected yet

**Behavior Execution:**
1. **Perceive:** Player 25 ft away, cover available
2. **Evaluate actions:**
   - Attack: Could shoot, but no advantage
   - Hide: 30 + 30 (cover) + 20 (has ranged) = 80 → set up ambush
3. **Execute bonus:** Move behind rock, Hide (Stealth +6)
4. **Hold action:** Wait for player to approach, or...
5. **Execute action:** Shortbow attack with advantage (hidden)

**End State:**
- Goblin attacked with advantage
- If hit, goblin revealed
- If missed, goblin may remain hidden (DM discretion)

---

## Use Case 5: Healing Potion (Stretch Goal)

**Scenario:** Goblin critically wounded, prioritizes survival.

**Initial State:**
- Goblin at position (5, 5) with 1 HP remaining (1/7)
- Player at position (6, 5) — adjacent
- Goblin has 1 healing potion

**Behavior Execution:**
1. **Perceive:** One enemy adjacent, HP critical (14%)
2. **Evaluate actions:**
   - Scimitar: 50 + 20 = 70
   - Drink Potion: 20 + 60 (HP < 30%) = 80 → survival priority!
   - Disengage: 30 + 40 + 20 = 90 → also high
3. **Decision tension:** Disengage vs Potion?
   - If goblin drinks potion, still adjacent and might die before next turn
   - If goblin disengages, can drink potion safely but uses bonus action
4. **Smart play:** Disengage (bonus), move away, then drink potion (action)
5. **Execute:** Nimble Escape (Disengage) → Move 30 ft → Drink Potion

**End State:**
- Goblin escaped and healed
- Potion consumed (uses: 0)
- Goblin lives to fight another round

---

## Integration Points

These use cases exercise:

| Component | Used In |
|-----------|---------|
| Action Economy | All — tracking action/bonus action/movement |
| Perception | UC1, UC2 — detecting targets and cover |
| Utility Scoring | All — selecting best action |
| Movement/Pathfinding | UC1, UC3 — moving toward/away from targets |
| Attack Resolution | UC1, UC2, UC4 — existing combat system |
| Features (Nimble Escape) | UC2, UC3, UC4 — bonus action Disengage/Hide |
| Conditions | UC5 — healing effects |
| Consumables | UC5 — potion usage tracking |

---

## What We Learn

1. **Movement is part of the decision** — Goblin may need to move before attacking or after disengaging
2. **Bonus action timing matters** — Nimble Escape happens before or after main action depending on situation
3. **Scoring needs tuning** — The numbers will need playtesting
4. **Personality could vary** — Some goblins might be braver (lower weight on flee behaviors)
5. **Resource tracking** — Potions, uses of abilities need to be tracked
