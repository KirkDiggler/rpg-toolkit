---
name: how to add a rulebook entry
description: Adding static D&D 5e data — background grants, race grants, spell data, monster stats
updated: 2026-05-02
---

# How to add a rulebook entry

Static data (background proficiencies, race languages, spell damage tables, monster stat blocks) lives in `rulebooks/dnd5e/` data-only packages. These packages have constants and data functions, no game logic.

## Adding a background grant

Edit `rulebooks/dnd5e/backgrounds/grants.go`:

```go
func GetGrants(bg Background) *Grant {
    switch bg {
    case MyNewBackground:
        return &Grant{
            SkillProficiencies: []skills.Skill{
                skills.Perception,
                skills.Survival,
            },
            ToolProficiencies: []proficiencies.Tool{
                proficiencies.ToolNavigator,
            },
        }
    // ... existing cases
    }
}
```

Also add the background constant in `backgrounds/backgrounds.go`.

**Write a test.** The `backgrounds/` package has no test files (issue #615). Any new background grant must include a test:

```go
// backgrounds/grants_test.go
func TestGetGrants(t *testing.T) {
    grant := GetGrants(MyNewBackground)
    require.NotNil(t, grant)
    assert.Contains(t, grant.SkillProficiencies, skills.Perception)
    assert.Contains(t, grant.ToolProficiencies, proficiencies.ToolNavigator)
}
```

Without this test, a wrong skill assignment in the switch goes undetected until rpg-api creates a character with wrong proficiencies.

## Adding a race grant

Edit `rulebooks/dnd5e/races/grants.go` — same pattern as backgrounds. Add a test in `races/grants_test.go`.

## Adding a monster stat block

Monster data lives in `rulebooks/dnd5e/monster/`. Each monster type is a function that returns a `*monster.Data`:

```go
// monster/goblin.go
func NewGoblin() *Data {
    return &Data{
        Name:       "Goblin",
        CR:         0.25,
        HitPoints:  7,
        ArmorClass: 15,
        Speed:      30,
        // ... abilities, attacks, traits
    }
}
```

Add the monster to the monster registry (wherever `GetMonster(type)` is implemented).

Write a test that asserts the stat block values match the Monster Manual entry. This is the source of truth — once merged, rpg-api will use these values in all encounters.

## Adding a spell

Spell data lives in `rulebooks/dnd5e/spells/`. Add a `SpellData` entry:

```go
// spells/spells.go
var Fireball = SpellData{
    Name:       "Fireball",
    Level:      3,
    School:     SchoolEvocation,
    CastingTime: ActionType,
    Range:       150,
    Components:  []Component{Verbal, Somatic, Material},
    Concentration: false,
    // DamageExpression: "8d6" — handled by the damage resolution chain, not here
}
```

The spell data struct is static. Damage resolution and saving throws happen in `combat/` using the chain pattern.

## Naming conventions

- All identifiers use constants, not magic strings.
- Background, Race, Class, Skill, Tool constants are typed aliases of `core.ID`.
- Monster types are Go type constants: `const GoblinType EntityType = "goblin"`.
- Ref values use lowercase hyphenated strings: `"my-new-background"`.

## Before committing

```bash
cd /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e
go test -race ./...         # all rulebook tests must pass
golangci-lint run ./...     # no new lint violations
```
