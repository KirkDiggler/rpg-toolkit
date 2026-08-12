---
name: how to add a rulebook entry
description: Entry points for D&D 5e grants, spells, and monster content
updated: 2026-08-10
---

# How to add a rulebook entry

D&D 5e content lives in owning packages under `rulebooks/dnd5e/`. Some entries are simple data mappings; others, especially monsters and spells, compose runtime rule behavior and persistence loaders. Verify the owning package and its tests instead of assuming an entry is data-only.

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

## Adding a monster

Use the dedicated [Add a D&D 5e monster](add-a-dnd5e-monster.md) contract. The current path is a runtime factory in `rulebooks/dnd5e/monster/monsters/`, plus a canonical ref, registry entry, focused tests, and the applicable multi-step load/round-trip proof. Factories return `*monster.Monster`, not `*monster.Data`.

The guide also enforces provenance: do not copy closed Monster Manual content. A clause that the current action/trait/resolution paths cannot express requires separately scoped mechanic work; do not silently omit it.

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
