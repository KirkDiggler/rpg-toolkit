// Package spells provides infrastructure for spell management and casting
// without implementing specific spells or magical systems.
//
// Purpose:
// This package establishes the framework for spell systems including
// spell slots, spell lists, concentration, and casting mechanics while
// remaining agnostic to specific spells or magic rules.
//
// Scope:
//   - Spell interface and metadata
//   - Spell slot management and consumption
//   - Spell list organization and filtering
//   - Concentration tracking
//   - Spell targeting and area templates
//   - Casting time and component tracking
//   - Spell preparation and known spells
//
// Non-Goals:
//   - Spell effects: What spells do is game-specific
//   - Spell schools: Magic categorization is game-specific
//   - Spell components: Material/verbal/somatic rules are game-specific
//   - Spell balance: Spell power and slots are game design
//   - Magic systems: Vancian, mana, etc. are game-specific
//   - Spell acquisition: How spells are learned is game logic
//   - Metamagic: Spell modifications are game-specific
//
// Integration:
// This package integrates with:
//   - effects: Spells create effects when cast
//   - resources: May use spell slots or mana
//   - events: Publishes spell cast/failed/interrupted events
//   - spatial: For spell targeting and areas
//
// Games implement their magic systems using this infrastructure
// while defining their own spells and casting rules.
//
// Example:
//
//	// Game defines a spell
//	type FireballSpell struct {
//	    level    int
//	    damage   string // "8d6"
//	    radius   float64
//	    saveType string
//	}
//
//	func (f *FireballSpell) ID() string { return "fireball" }
//	func (f *FireballSpell) Level() int { return f.level }
//	func (f *FireballSpell) CastingTime() time.Duration { return time.Second }
//	func (f *FireballSpell) Range() float64 { return 150.0 }
//
//	// Spell slot management
//	slots := spells.NewSlotTable()
//	slots.SetSlots(3, 4) // 4 third-level slots
//	slots.SetSlots(4, 3) // 3 fourth-level slots
//
//	// Cast spell
//	if slots.HasSlot(3) {
//	    slots.UseSlot(3)
//	    // Execute fireball effect
//	}
//
//	// Concentration
//	concentration := spells.NewConcentration()
//	err := concentration.Begin("fly", entity)
//	// Later...
//	if damaged {
//	    if !concentration.Check(damageTaken) {
//	        concentration.Break() // Lost concentration
//	    }
//	}
//
//	// Spell lists
//	wizardSpells := spells.NewSpellList()
//	wizardSpells.Add(&FireballSpell{level: 3})
//	wizardSpells.Add(&FlySpell{level: 3})
//
//	// Filter available spells
//	available := wizardSpells.FilterByLevel(3)
package spells
