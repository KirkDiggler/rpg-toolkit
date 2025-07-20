// Package effects provides composable effect infrastructure for modifying
// game values and triggering behaviors without implementing specific effects.
//
// Purpose:
// This package enables the creation of flexible, composable effects that
// can modify values, trigger on events, stack with other effects, and
// expire over time, all while remaining agnostic to what is being affected.
//
// Scope:
//   - Effect composition and chaining
//   - Conditional effect application
//   - Stackable effects with different stacking rules
//   - Temporary effects with duration tracking
//   - Triggered effects responding to events
//   - Effect targeting and area resolution
//   - Resource consumption for effect costs
//   - Effect tracker for managing active effects
//
// Non-Goals:
//   - Specific effect implementations: Damage, healing, etc. are game-specific
//   - Combat calculations: How effects modify combat is game logic
//   - Spell effects: Magical effects are game-specific
//   - Buff/debuff rules: What stacks and how is game-specific
//   - Effect visuals: Particle effects belong in presentation
//   - Balance values: Effect power/duration is game design
//
// Integration:
// This package integrates with:
//   - events: For triggered effects and notifications
//   - conditions: Effects may apply or require conditions
//   - resources: Effects may consume resources
//   - dice: For random effect values
//
// Games use this infrastructure to build their specific effect systems
// while the toolkit provides the architectural patterns.
//
// Example:
//
//	// Game defines a damage effect
//	type DamageEffect struct {
//	    amount     int
//	    damageType string
//	}
//
//	func (d *DamageEffect) Apply(target any) error {
//	    if damageable, ok := target.(Damageable); ok {
//	        return damageable.TakeDamage(d.amount, d.damageType)
//	    }
//	    return errors.New("target cannot take damage")
//	}
//
//	// Compose effects
//	fireballEffect := effects.Compose(
//	    &DamageEffect{amount: 8, damageType: "fire"},
//	    &ConditionEffect{condition: "burning", duration: time.Second * 6},
//	)
//
//	// Conditional effects
//	conditionalDamage := effects.Conditional(
//	    &DamageEffect{amount: 10, damageType: "cold"},
//	    func(target any) bool {
//	        return !hasResistance(target, "cold")
//	    },
//	)
//
//	// Track active effects
//	tracker := effects.NewTracker()
//	tracker.Add("regeneration", &RegenerationEffect{}, time.Minute)
package effects
