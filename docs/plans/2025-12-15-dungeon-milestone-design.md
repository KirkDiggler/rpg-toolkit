# Dungeon Milestone Design

**Date**: 2025-12-15
**Status**: Approved
**Milestone**: Playable Dungeon with Fighter, Barbarian, Rogue, Monk (Levels 1-3)

## Overview

Create a playable dungeon experience where players can:
- Create characters from 4 classes (Fighter, Barbarian, Rogue, Monk)
- Join a lobby, select character, equip gear
- Enter dungeon (auto long rest fires)
- Navigate connected rooms fighting goblins, skeletons, and a boss
- Use all class features appropriate to their level (1-3)
- Experience death saves when at 0 HP (3 failures = dungeon failure)
- Victory = all monsters dead

## Scope

### In Scope
- All level 1-3 features for Fighter, Barbarian, Rogue, Monk
- Equipment UI rework (compact, accessible in lobby)
- Long rest before dungeon (automatic)
- Death saving throws
- Template-based room generation with CR scaling
- Connectable rooms for multi-room dungeons
- Monster variety (goblins, skeletons, boss)
- Multiplayer fixes (second player recognition in combat)

### Stretch Goals
- Short rest between rooms (hit dice recovery, short-rest feature reset)
- Traps (event bus listeners for movement)

### Out of Scope
- Spellcasting classes (Eldritch Knight spells skipped)
- Skill checks for discovery/exploration
- Shopping/item acquisition
- Permadeath or save/load
- XP and leveling (features working is the goal, leveling is secondary)
- Random room generation (templates for now, tracked for future)

## Class Features Audit

### Fighter (Levels 1-3)

| Level | Feature | Implementation Status | Needs |
|-------|---------|----------------------|-------|
| 1 | Fighting Style - Archery | Done | - |
| 1 | Fighting Style - Defense | Done | - |
| 1 | Fighting Style - Dueling | Done | Uses GameCtx |
| 1 | Fighting Style - Great Weapon | Done | - |
| 1 | Fighting Style - Two-Weapon | Partial | Hardcodes ability mod to +3 |
| 1 | Fighting Style - Protection | Not Done | Needs reaction system + ally check |
| 1 | Second Wind | Exists | Verify 1/short rest tracking |
| 2 | Action Surge | Exists | Verify 1/short rest tracking |
| 3 | Champion - Improved Critical | Exists | Crit on 19-20 |

**Subclass Note**: Battle Master (maneuvers) and Eldritch Knight (spells) deferred.

### Barbarian (Levels 1-3)

| Level | Feature | Implementation Status | Needs |
|-------|---------|----------------------|-------|
| 1 | Rage | Done | Verify uses/long rest |
| 1 | Unarmored Defense | Done | DEX+CON to AC |
| 2 | Reckless Attack | Needs verification | Advantage grant + incoming tracking |
| 2 | Danger Sense | Needs verification | DEX save advantage |
| 3 | Berserker - Frenzy | Needs verification | Bonus action attack while raging |

### Rogue (Levels 1-3)

| Level | Feature | Implementation Status | Needs |
|-------|---------|----------------------|-------|
| 1 | Sneak Attack | Partial | Currently triggers on any DEX attack, needs ally position check |
| 1 | Expertise | Track only | Skill checks out of scope |
| 2 | Cunning Action | Needs implementation | Bonus action dash/disengage/hide |
| 3 | Thief - Fast Hands | Needs verification | Use object as bonus action |

**Note**: Assassin's Assassinate needs surprise round - complex, may defer.

### Monk (Levels 1-3)

| Level | Feature | Implementation Status | Needs |
|-------|---------|----------------------|-------|
| 1 | Unarmored Defense | In PR #394 | DEX+WIS to AC |
| 1 | Martial Arts | In PR #394 | DEX for monk weapons, scaling damage, bonus unarmed |
| 2 | Ki | In PR #394 | Ki points = monk level, recover on short rest |
| 2 | Flurry of Blows | In PR #394 | 1 ki, 2 unarmed strikes as bonus action |
| 2 | Patient Defense | In PR #394 | 1 ki, Dodge as bonus action |
| 2 | Step of the Wind | In PR #394 | 1 ki, Disengage/Dash as bonus action |
| 3 | Deflect Missiles | In PR #394 | Reaction, reduce ranged damage |
| 3 | Unarmored Movement | In PR #394 | +10 ft speed |

**Note**: PR #394 has merge conflicts with main due to GameCtx changes. Needs conflict resolution.

## Discovered Gaps

### Critical
1. **Sneak Attack** - Doesn't check advantage OR ally within 5ft (just DEX weapon)
2. **Protection style** - Not implemented (needs reaction system)
3. **Two-Weapon Fighting** - Hardcodes ability modifier to +3
4. **Monk features** - PR #394 closed, needs reopening and conflict resolution
5. **Action economy** - Exists in toolkit but API doesn't enforce it
6. **Multiplayer** - Combat room doesn't recognize second player

### Moderate
1. **GameCtx** - Needs Room entities for positional queries
2. **Rest system** - Long rest needs implementing for dungeon start
3. **Death saves** - Not implemented

## Technical Details

### Death Saves Flow

```
Character HP hits 0
    |
    +-> Character marked unconscious
    |   (skip turn actions, only death saves)
    |
    +-> On character's turn: roll d20
    |   +- 1 = 2 failures
    |   +- 2-9 = 1 failure
    |   +- 10-19 = 1 success
    |   +- 20 = regain 1 HP, conscious
    |
    +-> If takes damage while down:
    |   +- +1 failure (or +2 if crit)
    |
    +-> 3 failures = character "dead" = dungeon failed for party
    |
    +-> 3 successes = stabilized (still unconscious, no more saves)
```

Party has until 3rd failure to heal/stabilize the downed player.

### CR-Based Room Generation

```
Party enters dungeon
    |
    +-> Calculate party strength (sum of character levels)
    |
    +-> For each room:
    |   +- Select template (predefined room shape)
    |   +- If combat: select monsters where total CR ~ target difficulty
    |   +- Place monsters with some randomness in positions
    |
    +-> Boss room: CR = harder than normal rooms
```

Templates for this milestone; random generation tracked for future.

### Rest System

**Long Rest (MVP)**:
- Triggered automatically before dungeon start
- Resets: HP to max, all hit dice, all feature uses (Rage, Action Surge, etc.)

**Short Rest (Stretch)**:
- Available between rooms
- Player can spend hit dice to recover HP
- Resets: short-rest features (Second Wind, Action Surge, Ki)

## Tracks by Repository

Work is organized by repository. One track per repo at a time; repos can run in parallel.

### rpg-toolkit (sequential)

| Order | Track | Description |
|-------|-------|-------------|
| 1 | GameCtx Audit | Ensure Room, ability scores accessible |
| 2 | Monk Integration | Reopen PR #394, resolve conflicts |
| 3 | Feature Gaps | Sneak Attack ally check, Protection style, Two-Weapon ability mod |
| 4 | Monster Additions | Skeletons, boss (troll/orc) |

### rpg-api (sequential)

| Order | Track | Description |
|-------|-------|-------------|
| 1 | Multiplayer Fix | Combat room recognizes all players |
| 2 | API Enforcement | Action/bonus/reaction limits |
| 3 | Rest System | Long rest before dungeon, short rest stretch |
| 4 | Dungeon Generation | Room templates with CR scaling |
| 5 | Death Saves | 0 HP -> saves -> fail condition |

### rpg-dnd5e-web (parallel with above)

| Order | Track | Description |
|-------|-------|-------------|
| 1 | Equipment UI | Compact layout with icons |
| 2 | Lobby Flow | Equip step before ready |
| 3 | Multiplayer Polish | All players visible, smooth experience |

### Cross-repo (after above)

- **Verification** - End-to-end testing of all classes/features

## Definition of Done

### rpg-toolkit

**GameCtx Audit**
- [ ] Room entities accessible via `gamectx.Room(ctx)`
- [ ] Character ability scores accessible for modifier calculations
- [ ] Integration test showing feature can query ally positions

**Monk Integration**
- [ ] PR #394 merged (conflicts resolved)
- [ ] Ki resource consumed and recovered correctly
- [ ] All monk features have passing tests
- [ ] Monk equipment choices work end-to-end

**Feature Gaps**
- [ ] Sneak Attack triggers only with advantage OR ally within 5ft (test)
- [ ] Protection style imposes disadvantage via reaction (test)
- [ ] Two-Weapon Fighting uses actual ability modifier (test)

**Monster Additions**
- [ ] Skeleton stat block exists with actions
- [ ] Boss monster (troll or orc) stat block exists
- [ ] Monster behavior works for new types

### rpg-api

**Multiplayer Fix**
- [ ] Combat room recognizes all joined players
- [ ] All player characters appear in combat state
- [ ] Turn order includes all player characters

**API Enforcement**
- [ ] Attack rejected if no action available
- [ ] Bonus action features rejected if bonus action used
- [ ] Turn tracks action/bonus/reaction state

**Rest System**
- [ ] Long rest resets HP, hit dice, all feature uses
- [ ] Long rest fires automatically before dungeon start
- [ ] (Stretch) Short rest spends hit dice, resets short-rest features

**Dungeon Generation**
- [ ] Room created with random monster placement
- [ ] Monster selection uses CR to scale difficulty
- [ ] Rooms can be connected for multi-room dungeon

**Death Saves**
- [ ] 0 HP triggers unconscious state
- [ ] Death save rolled each turn while unconscious
- [ ] 3 failures = dungeon failed
- [ ] 3 successes = stabilized
- [ ] Damage while down = automatic failure

### rpg-dnd5e-web

**Equipment UI**
- [ ] Compact layout fits on one screen (no scrolling between slots and inventory)
- [ ] Icons identify item types
- [ ] Manual verification: equip/unequip works

**Lobby Flow**
- [ ] Equipment accessible before ready (in lobby)
- [ ] Manual verification: can change gear then ready up

**Multiplayer Polish**
- [ ] All party members visible with their ready state
- [ ] All players' characters render on combat map
- [ ] Manual verification: 2+ players can take turns

## Future Work

These items are out of scope but should be tracked as issues for future milestones. Pay attention to these during implementation - take notes on insights gained.

### Dungeon System
- Random room shape generation
- Procedural room connections
- Room themes/biomes affecting content
- Trap system (event bus listeners for movement)

### Combat Enhancements
- Surprise rounds (for Assassin's Assassinate)
- Battle Master maneuvers
- Reactions beyond Protection style
- Spellcasting integration

### Classes
- Remaining subclasses (Eldritch Knight, Arcane Trickster - need spells)
- Multi-classing support

### Progression
- XP from kills
- Level up flow
- Feat selection

### Persistence
- Save/load dungeon progress
- Character persistence across sessions

## References

- PR #394: Monk Level 1-3 Features (closed, needs reopening)
- Fighting styles: `rulebooks/dnd5e/fightingstyles/fightingstyles.go`
- Sneak attack: `rulebooks/dnd5e/conditions/sneak_attack.go`
- Fighting style conditions: `rulebooks/dnd5e/conditions/fighting_style.go`
- GameCtx: `rulebooks/dnd5e/gamectx/`
