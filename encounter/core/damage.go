package core

// DamageComponent is the encounter-SDK-level descriptor of a single damage
// source within a hit. Rulebook resolvers populate []DamageComponent on
// AttackOutcome so the encounter SDK can forward the breakdown to
// DamageDealtEvent without importing rulebook-specific types.
//
// Source is an opaque identifier the wire layer uses to label each line.
// Convention: use the toolkit's SourceRef.String() for named sources (e.g.
// "dnd5e:conditions:sneak_attack") or a category string ("weapon", "ability")
// for generic sources. Amount is the component's total contribution (dice +
// flat bonus after chain modifiers). DamageType mirrors the top-level
// DamageDealtEvent field for components that carry a distinct damage type.
// IsCritical flags components that were doubled for a critical hit.
type DamageComponent struct {
	Source     string
	Amount     int
	DamageType string
	IsCritical bool
}
