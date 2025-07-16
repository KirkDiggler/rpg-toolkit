package proficiency

import (
	"context"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

// WeaponProficiency implements weapon proficiency for D&D 5e
type WeaponProficiency struct {
	*proficiency.SimpleProficiency
	level    int
	category WeaponCategory  // If this is a category proficiency
	weapons  map[string]bool // Specific weapons this proficiency covers
}

// NewWeaponProficiency creates a weapon proficiency that adds attack bonuses
func NewWeaponProficiency(owner core.Entity, subject string, source string, level int) *WeaponProficiency {
	wp := &WeaponProficiency{
		level:   level,
		weapons: make(map[string]bool),
	}

	// Check if this is a category proficiency
	switch subject {
	case "simple-weapons":
		wp.category = WeaponCategorySimple
		wp.loadSimpleWeapons()
	case "martial-weapons":
		wp.category = WeaponCategoryMartial
		wp.loadMartialWeapons()
	default:
		// Specific weapon proficiency
		wp.weapons[subject] = true
	}

	// Create the underlying simple proficiency with custom handlers
	wp.SimpleProficiency = proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		ID:      fmt.Sprintf("%s-weapon-prof-%s", owner.GetID(), subject),
		Type:    "proficiency.weapon",
		Owner:   owner,
		Subject: subject,
		Source:  source,
		ApplyFunc: func(p *proficiency.SimpleProficiency, bus events.EventBus) error {
			// Subscribe to attack roll events to add proficiency bonus
			p.Subscribe(bus, events.EventOnAttackRoll, 100, wp.handleAttackRoll)
			return nil
		},
	})

	return wp
}

// handleAttackRoll adds proficiency bonus if using a proficient weapon
func (wp *WeaponProficiency) handleAttackRoll(ctx context.Context, e events.Event) error {
	// Only apply to our owner
	if e.Source() == nil || e.Source().GetID() != wp.Owner().GetID() {
		return nil
	}

	// Get the weapon being used
	weapon, ok := e.Context().GetString("weapon")
	if !ok || weapon == "" {
		return nil
	}

	// Check if proficient with this weapon
	if !wp.isProficientWith(weapon) {
		return nil
	}

	// Add proficiency bonus modifier
	profBonus := GetProficiencyBonus(wp.level)
	e.Context().AddModifier(events.NewModifier(
		"weapon-proficiency",
		events.ModifierAttackBonus,
		events.NewRawValue(profBonus, fmt.Sprintf("proficiency with %s", weapon)),
		50, // Apply after base modifiers
	))

	return nil
}

// isProficientWith checks if this proficiency applies to a weapon
func (wp *WeaponProficiency) isProficientWith(weapon string) bool {
	// Direct weapon proficiency
	if wp.weapons[weapon] {
		return true
	}

	// Category proficiency
	switch wp.category {
	case WeaponCategorySimple:
		return isSimpleWeapon(weapon)
	case WeaponCategoryMartial:
		return isMartialWeapon(weapon)
	}

	return false
}

// loadSimpleWeapons populates the list of simple weapons
func (wp *WeaponProficiency) loadSimpleWeapons() {
	simpleWeapons := []string{
		"club", "dagger", "greatclub", "handaxe", "javelin",
		"light-hammer", "mace", "quarterstaff", "sickle", "spear",
		"light-crossbow", "dart", "shortbow", "sling",
	}
	for _, w := range simpleWeapons {
		wp.weapons[w] = true
	}
}

// loadMartialWeapons populates the list of martial weapons
func (wp *WeaponProficiency) loadMartialWeapons() {
	martialWeapons := []string{
		"battleaxe", "flail", "glaive", "greataxe", "greatsword",
		"halberd", "lance", "longsword", "maul", "morningstar",
		"pike", "rapier", "scimitar", "shortsword", "trident",
		"war-pick", "warhammer", "whip", "blowgun", "hand-crossbow",
		"heavy-crossbow", "longbow", "net",
	}
	for _, w := range martialWeapons {
		wp.weapons[w] = true
	}
}

// Helper functions for checking weapon categories
func isSimpleWeapon(weapon string) bool {
	simpleWeapons := map[string]bool{
		"club": true, "dagger": true, "greatclub": true,
		"handaxe": true, "javelin": true, "light-hammer": true,
		"mace": true, "quarterstaff": true, "sickle": true,
		"spear": true, "light-crossbow": true, "dart": true,
		"shortbow": true, "sling": true,
	}
	return simpleWeapons[strings.ToLower(weapon)]
}

func isMartialWeapon(weapon string) bool {
	martialWeapons := map[string]bool{
		"battleaxe": true, "flail": true, "glaive": true,
		"greataxe": true, "greatsword": true, "halberd": true,
		"lance": true, "longsword": true, "maul": true,
		"morningstar": true, "pike": true, "rapier": true,
		"scimitar": true, "shortsword": true, "trident": true,
		"war-pick": true, "warhammer": true, "whip": true,
		"blowgun": true, "hand-crossbow": true, "heavy-crossbow": true,
		"longbow": true, "net": true,
	}
	return martialWeapons[strings.ToLower(weapon)]
}
