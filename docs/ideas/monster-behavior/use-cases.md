# Monster Behavior - Use Cases

Concrete scenarios showing the behavior system working end-to-end.

---

## Use Case 1: Basic Melee Attack

**Scenario:** Goblin sees a player character, moves toward them, and attacks.

**Initial State:**
- Goblin at position (0, 5)
- Player at position (6, 5)
- Goblin has full HP (7/7)
- Goblin's turn begins

**Behavior Execution:**
1. **Perceive:** Goblin detects player 30 feet away (within darkvision 60 ft)
2. **Evaluate actions:**
   - Scimitar: 50 base, but target not adjacent → invalid
   - Shortbow: 50 + 20 (target at range) = 70 → valid
   - Move toward target: Always valid as fallback
3. **Select:** Shortbow scores highest, but goblin prefers melee (personality?) OR moves first
4. **Execute movement:** Move 30 ft toward player → now at (5, 5), adjacent
5. **Re-evaluate:** Now adjacent
   - Scimitar: 50 + 20 = 70 → valid
   - Shortbow: 50 - 100 (disadvantage when adjacent) = -50 → poor choice
6. **Execute attack:** Scimitar attack, roll d20+4 vs AC
7. **Bonus action:** Evaluate Nimble Escape
   - Disengage: 30 base (HP fine, only 1 enemy) = 30
   - Hide: 30 base, no cover nearby = 30
   - Neither scores high, goblin holds bonus action or takes no action

**End State:**
- Goblin at (5, 5), adjacent to player
- Player possibly took 1d6+2 slashing damage
- Goblin used: Action (attack), Movement (30 ft), Bonus Action (none)

---

## Use Case 2: Ranged Attack Preference

**Scenario:** Goblin starts at range and prefers to stay at range.

**Initial State:**
- Goblin at position (0, 0)
- Player at position (8, 0) — 40 feet away
- Goblin has full HP (7/7)
- Cover (pillar) at position (2, 2)

**Behavior Execution:**
1. **Perceive:** Player 40 ft away, within shortbow range (80 ft)
2. **Evaluate actions:**
   - Scimitar: Invalid (not adjacent)
   - Shortbow: 50 + 20 (target at range) = 70 → valid
3. **Execute attack:** Shortbow attack, roll d20+4 vs AC
4. **Bonus action:** Evaluate Nimble Escape
   - Disengage: 30 base, no adjacent enemies = 30
   - Hide: 30 + 30 (cover nearby at pillar) + 20 (has ranged) = 80 → high!
5. **Execute bonus:** Move toward pillar, Hide action
   - Goblin rolls Stealth (+6) vs player's passive perception

**End State:**
- Goblin hidden behind pillar
- Player possibly took 1d6+2 piercing damage
- Next turn: Goblin attacks with advantage (if still hidden)

---

## Use Case 3: Nimble Escape (Disengage) — Tactical Retreat

**Scenario:** Goblin is wounded and surrounded, needs to escape.

**Initial State:**
- Goblin at position (5, 5) with 2 HP remaining (2/7)
- Player A at (4, 5) — adjacent
- Player B at (6, 5) — adjacent
- Exit/safety at (0, 5)

**Behavior Execution:**
1. **Perceive:** Two enemies adjacent, HP critical (28%)
2. **Evaluate actions:**
   - Scimitar: 50 + 20 = 70
   - Disengage: 30 + 40 (HP < 50%) + 40 (2 adjacent enemies × 20) = 110 → highest!
   - Hide: Can't hide while adjacent
   - Drink Potion: 20 + 60 (HP < 30%) = 80
3. **Decision:** Disengage scores highest — survival instinct kicks in
4. **Execute bonus:** Nimble Escape (Disengage)
5. **Execute movement:** Move 30 ft toward exit without provoking opportunity attacks
6. **Action remaining:** Could attack at range or drink potion
   - Shortbow: Now at range, 50 + 20 = 70
   - Drink Potion: 80 (still wounded)
7. **Execute action:** Drink healing potion, regain 2d4+2 HP

**End State:**
- Goblin retreated 30 ft, now safer
- Goblin healed, possibly back to ~7 HP
- Players must chase or use ranged attacks

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
