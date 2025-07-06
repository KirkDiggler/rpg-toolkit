# Proficiency System Migration Guide

This guide shows how to migrate the DND bot's proficiency system to use rpg-toolkit.

## Current DND Bot Structure

```go
// In character/character_proficiencies.go
type Character struct {
    Proficiencies map[ProficiencyType][]*Proficiency
}

func (c *Character) HasWeaponProficiency(weapon string) bool
func (c *Character) HasSkillProficiency(skill string) bool
func (c *Character) GetProficiencyBonus() int
```

## Migration Steps

### Step 1: Add Entity Wrappers

Create wrappers in `internal/adapters/toolkit/entity.go`:

```go
package toolkit

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/dnd-bot-discord/internal/domain/character"
)

type CharacterEntity struct {
    *character.Character
}

func (c *CharacterEntity) GetID() string   { return c.Character.ID }
func (c *CharacterEntity) GetType() string { return "character" }

func WrapCharacter(c *character.Character) core.Entity {
    return &CharacterEntity{Character: c}
}
```

### Step 2: Create Proficiency Service

Create `internal/adapters/toolkit/proficiency_service.go`:

```go
package toolkit

import (
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

type ProficiencyService struct {
    bus      *events.Bus
    managers map[string]*proficiency.SimpleManager
}

func NewProficiencyService(bus *events.Bus) *ProficiencyService {
    return &ProficiencyService{
        bus:      bus,
        managers: make(map[string]*proficiency.SimpleManager),
    }
}

// Migrate existing character proficiencies
func (s *ProficiencyService) MigrateCharacter(char *character.Character) error {
    entity := WrapCharacter(char)
    manager := proficiency.NewSimpleManager()
    s.managers[char.ID] = manager
    
    // Migrate each proficiency type
    for profType, profs := range char.Proficiencies {
        for _, prof := range profs {
            toolkitProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
                ID:      fmt.Sprintf("%s-%s", char.ID, prof.Key),
                Owner:   entity,
                Subject: prof.Key,
                Source:  string(profType),
            })
            
            if err := manager.AddProficiency(toolkitProf, s.bus); err != nil {
                return err
            }
        }
    }
    
    return nil
}
```

### Step 3: Update Character Methods

Replace proficiency methods to use toolkit:

```go
// Before:
func (c *Character) HasWeaponProficiency(weaponKey string) bool {
    // Old implementation
}

// After:
func (c *Character) HasWeaponProficiency(weaponKey string) bool {
    return proficiencyService.CheckProficiency(c.ID, weaponKey)
}

// Add to character service
func (s *CharacterService) GetProficiencyBonus(level int) int {
    return 2 + ((level - 1) / 4)
}
```

### Step 4: Integration Points

#### Attack Calculations
```go
// In weapon.go Attack() method
func (w *Weapon) Attack(attacker *Character) (int, int, error) {
    // Get ability modifier
    attackBonus := attacker.GetAbilityModifier(w.AbilityScore)
    
    // Add proficiency bonus if proficient
    if proficiencyService.CheckProficiency(attacker.ID, w.Key) {
        attackBonus += proficiencyService.GetProficiencyBonus(attacker.Level)
    }
    
    // Roll attack...
}
```

#### Skill Checks
```go
func (c *Character) RollSkillCheck(skill string) (int, error) {
    // Get ability modifier for skill
    modifier := c.GetAbilityModifier(skillAbility[skill])
    
    // Add proficiency bonus if proficient
    if proficiencyService.CheckProficiency(c.ID, skill) {
        modifier += proficiencyService.GetProficiencyBonus(c.Level)
    }
    
    // Roll dice...
}
```

#### Saving Throws
```go
func (c *Character) RollSavingThrow(ability string) (int, error) {
    modifier := c.GetAbilityModifier(ability)
    
    // Check for save proficiency
    saveKey := fmt.Sprintf("%s-save", strings.ToLower(ability))
    if proficiencyService.CheckProficiency(c.ID, saveKey) {
        modifier += proficiencyService.GetProficiencyBonus(c.Level)
    }
    
    // Roll dice...
}
```

### Step 5: Benefits of Migration

1. **Event Integration**: Proficiency changes trigger events
   ```go
   bus.Subscribe("proficiency.added", func(ctx context.Context, e events.Event) error {
       // Update character sheet UI
       // Log proficiency gain
       return nil
   })
   ```

2. **Consistent API**: All proficiencies use same interface
   ```go
   // Works for weapons, skills, tools, languages, etc.
   proficiencyService.CheckProficiency(charID, subject)
   ```

3. **Category Support**: Built-in support for proficiency categories
   ```go
   // "simple-weapons" proficiency covers all simple weapons
   // "martial-weapons" proficiency covers all martial weapons
   ```

4. **Future Features**: Easy to add expertise, half-proficiency, etc.

## Testing the Migration

Run the existing tests to ensure compatibility:
```bash
cd examples/dndbot_integration
go test -run TestProficiencyIntegration
```

## Rollback Plan

If issues arise, the migration can be rolled back by:
1. Keeping the original proficiency methods as fallbacks
2. Using a feature flag to toggle between systems
3. Running both systems in parallel during transition

## Next Steps

After proficiencies are migrated:
1. **Features/Feats**: Use similar pattern for character features
2. **Conditions**: Migrate status effects
3. **Resources**: Spell slots, rage uses, etc.