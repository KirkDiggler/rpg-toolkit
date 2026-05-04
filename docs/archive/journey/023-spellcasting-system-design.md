# Journey 023: Spellcasting System Design

## Date: 2025-08-12

## The Problem

While working on the features refactor, we added `WithSpellLevel()` as a pragmatic solution for spell casting. But this exposed deeper questions:

1. **Who verifies spell slots?** The spell itself shouldn't care about slots
2. **How do characters get spellbooks?** Not all characters are spellcasters
3. **How do we compose spellcasting?** Multiclass characters, magic items granting spells, racial spells
4. **Where do spell lists live?** Class-specific lists, domain spells, learned vs prepared

## The Core Insight

Spellcasting is a **capability**, not a core character trait. Some characters have it, some don't. Some get it from their class, some from race, some from items.

## Current Approach (Too Simple)

```go
// We're treating spells like features
type Character struct {
    Features []Feature
    Spells   []Spell  // But not everyone has spells!
}

// And the spell has to check everything
func (f *FireballSpell) Activate(owner core.Entity, opts ...ActivateOption) error {
    // Does owner have spell slots? Who knows!
    if !owner.HasSpellSlot(3) {  // Spell shouldn't know this
        return ErrNoSpellSlots
    }
}
```

## Design Questions

### 1. How Do Characters Get Spellcasting?

**Option A: Spellcasting as a Feature**
```go
type SpellcastingFeature struct {
    *features.SimpleFeature
    spellbook     *Spellbook
    spellSlots    *SpellSlots
    ability       AbilityScore  // INT for wizard, WIS for cleric
    spellListRef  *core.Ref     // "dnd5e:spell_list:wizard"
}

// Character gets it like any feature
wizard.AddFeature(NewWizardSpellcasting())
```

**Option B: Spellcasting as a Capability Interface**
```go
type Spellcaster interface {
    GetSpellbook() *Spellbook
    GetSpellSlots() *SpellSlots
    CanCastSpell(spell Spell, level int) bool
    CastSpell(spellRef string, level int, target core.Entity) error
}

type Character struct {
    core.Entity
    features []Feature
    // Optional capabilities
    spellcasting Spellcaster  // nil for non-casters
}
```

**Option C: Composable Spellcasting Sources**
```go
type Character struct {
    spellSources []SpellSource  // Multiple sources of spells
}

type SpellSource interface {
    GetKnownSpells() []Spell
    GetSpellSlots(level int) int
    GetSaveDC() int
}

// Examples
classSpells := &ClassSpellcasting{class: "wizard", level: 5}
racialSpells := &RacialSpellcasting{race: "tiefling"}  // Darkness 1/day
itemSpells := &ItemSpellcasting{item: "wand_of_fireballs"}
```

### 2. Spell Preparation vs Known Spells

Different classes handle spells differently:

**Wizards** - Large spellbook, prepare subset each day
```go
type WizardSpellcasting struct {
    spellbook      []Spell  // All spells in book
    preparedSpells []Spell  // Today's selection
    spellSlots     *SpellSlots
}
```

**Sorcerers** - Limited known spells, cast any known
```go
type SorcererSpellcasting struct {
    knownSpells []Spell  // Small fixed list
    spellSlots  *SpellSlots
    sorceryPoints int    // Metamagic resource
}
```

**Clerics** - Know all spells, prepare subset
```go
type ClericSpellcasting struct {
    domainSpells   []Spell  // Always prepared
    preparedSpells []Spell  // Daily selection from full list
    spellSlots     *SpellSlots
}
```

### 3. Multiclassing Complexity

How do we handle:
```go
// Character is Wizard 3 / Cleric 2
// - Wizard spells use INT
// - Cleric spells use WIS  
// - Spell slots combine (special table)
// - Prepared spells tracked separately
```

### 4. Spell-Like Abilities

Not everything uses spell slots:
```go
// Racial abilities
tiefling.CastDarkness()  // 1/day, no slot

// Magic items
wand.CastFireball()  // Uses charges, not slots

// Warlock invocations
warlock.CastDetectMagic()  // At will, no slot

// Monster abilities
dragon.CastSpell("suggestion")  // 3/day
```

## Proposed Design

### Core Concepts

1. **Spellcasting is a Feature** that provides spell capabilities
2. **Spellbook** tracks known/prepared spells
3. **SpellSlots** manages slot consumption
4. **SpellSources** allow composition

### Implementation

```go
// The base spellcasting feature
type SpellcastingFeature struct {
    *features.SimpleFeature
    source      *core.Source     // Where spellcasting comes from
    ability     AbilityScore     // Casting stat
    spellList   *core.Ref        // Available spell list
    
    // Spell management
    knownSpells    []*Spell
    preparedSpells []*Spell  // nil if casts from known
    
    // Resources
    slots *SpellSlotTable
}

// Character-level spell management
func (c *Character) CastSpell(spellRef string, level int, target core.Entity) error {
    // Find spellcasting source that knows this spell
    var caster SpellcastingFeature
    var spell *Spell
    
    for _, feat := range c.features {
        if sc, ok := feat.(SpellcastingFeature); ok {
            if spell = sc.GetSpell(spellRef); spell != nil {
                caster = sc
                break
            }
        }
    }
    
    if spell == nil {
        return ErrSpellNotKnown
    }
    
    // Verify slot available
    if !caster.HasSlot(level) {
        return ErrNoSpellSlot
    }
    
    // Consume slot
    caster.UseSlot(level)
    
    // Cast the spell - it just does spell things
    return spell.Activate(c, 
        WithTarget(target),
        WithSpellLevel(level),
        WithSaveDC(caster.GetSaveDC()),
    )
}
```

### Spell-Like Abilities

For things that don't use slots:

```go
type SpellLikeAbility struct {
    *features.SimpleFeature
    spell     *Spell
    uses      *resources.CountResource  // 3/day, 1/day, etc
    recharge  RechargeType  // Short rest, long rest, dawn
}

func (s *SpellLikeAbility) Activate(owner core.Entity, opts ...ActivateOption) error {
    if !s.uses.CanConsume(1) {
        return ErrNoUsesRemaining
    }
    
    s.uses.Consume(1)
    
    // Cast without using spell slots
    return s.spell.Activate(owner, opts...)
}
```

## Benefits of This Approach

1. **Separation of Concerns**
   - Spells just describe effects
   - Spellcasting features manage slots/preparation
   - Character coordinates between them

2. **Composable**
   - Multiple spellcasting sources
   - Each tracks its own spells/slots
   - Character aggregates them

3. **Flexible**
   - Handles class spellcasting
   - Handles racial/item abilities
   - Handles spell-like abilities

4. **Type-Safe**
   - No checking if character "has" spellcasting
   - Features declare their capabilities

## Open Questions

1. **Spell slot combining for multiclass** - Use lookup table? Calculate?
2. **Ritual casting** - Feature of the spell or the caster?
3. **Concentration** - Track at character level or spellcasting level?
4. **Spell components** - Do we track material components?
5. **Upcasting** - All spells can upcast or only some?

## Success Criteria

A multiclass Wizard 3/Cleric 2 should be able to:
```go
// Cast wizard spell using INT
character.CastSpell("fireball", 3, target)  // Uses wizard spellcasting

// Cast cleric spell using WIS  
character.CastSpell("cure_wounds", 1, target)  // Uses cleric spellcasting

// Both pull from combined spell slots
// Character manages the complexity, spells stay simple
```

## Next Steps

1. Implement SpellcastingFeature
2. Create SpellSlotTable for slot management
3. Design spell preparation mechanics
4. Handle multiclass slot combination
5. Create examples for each casting class

## The Measure

Success is when implementing a new spellcasting class is just:
1. Define their spell list
2. Define their slot progression
3. Define their preparation rules
4. Everything else just works

The complexity should be in the game rules, not the infrastructure.