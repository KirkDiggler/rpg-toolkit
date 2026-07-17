package encounter_test

// permanent_conditions_test.go is the golden-list regression test for
// rpg-toolkit#778's exclusion set: the union of
// character.StructurallyPermanentConditionRefs() and
// monstertraits.AllTraitRefs() that active_conditions.go filters
// ActiveConditions against.
//
// This is a deliberate tripwire, not a redundant assertion: the set is
// derived from classes.GetGrants()/fightingstyles.All() (genuinely
// data-driven) and monstertraits' own trait list (hand-mirrored against
// LoadJSON's dispatch — see AllTraitRefs' doc comment for why that one
// isn't fully derivation-proof). A future class migration adding a new
// permanent Grant.Conditions entry, a new fighting style, or a new monster
// trait changes this test's expected list. That's the point: it forces a
// human to look at the diff and confirm the new ref really is build-time
// -only before the test goes green again, rather than the ref silently
// joining (or silently failing to join) the exclusion set.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
)

// TestStructurallyPermanentConditionRefs_GoldenList pins the exact character
// -side derived set: 3 Grant.Conditions entries (UnarmoredDefense,
// MartialArts, SneakAttack — Barbarian/Monk/Rogue level-1 grants) + 6
// fighting-style refs (every entry in fightingstyles.All()). Order-
// independent (ElementsMatch) since the underlying derivation walks a map
// (classes.All) with no ordering guarantee.
func TestStructurallyPermanentConditionRefs_GoldenList(t *testing.T) {
	got := dnd5eCharacter.StructurallyPermanentConditionRefs()

	want := []string{
		// Grant.Conditions entries (classes/grant.go).
		"dnd5e:conditions:unarmored_defense",
		"dnd5e:conditions:martial_arts",
		"dnd5e:conditions:sneak_attack",
		// Fighting styles (fightingstyles.All()).
		"dnd5e:conditions:fighting_style_archery",
		"dnd5e:conditions:fighting_style_defense",
		"dnd5e:conditions:fighting_style_dueling",
		"dnd5e:conditions:fighting_style_great_weapon_fighting",
		"dnd5e:conditions:fighting_style_protection",
		"dnd5e:conditions:fighting_style_two_weapon_fighting",
	}

	require.ElementsMatch(t, want, got,
		"if this fails because a NEW ref appeared: confirm it's genuinely build-time-only "+
			"(never reachable via Encounter.ActivateFeature) before updating this golden list — "+
			"see StructurallyPermanentConditionRefs' doc comment for the invariant this depends on. "+
			"if a ref is MISSING: a class/fighting-style was removed, or the derivation broke.")

	// Genuinely runtime-activated refs must NEVER appear here — this is the
	// other half of the tripwire: it's not enough for the golden list to be
	// exhaustive, it must also stay exclusive.
	for _, runtimeRef := range []string{
		"dnd5e:conditions:raging",
		"dnd5e:conditions:dodging",
		"dnd5e:conditions:disengaging",
		"dnd5e:conditions:hidden",
	} {
		assert.NotContains(t, got, runtimeRef,
			"%s is genuinely live-activated and must never be classified as structurally permanent", runtimeRef)
	}
}

// TestMonstertraitsAllTraitRefs_GoldenList pins the 4 known monster trait
// refs monstertraits.LoadJSON's dispatch switch recognizes.
func TestMonstertraitsAllTraitRefs_GoldenList(t *testing.T) {
	got := monstertraits.AllTraitRefs()

	want := []string{
		"dnd5e:monster_traits:immunity",
		"dnd5e:monster_traits:vulnerability",
		"dnd5e:monster_traits:pack_tactics",
		"dnd5e:monster_traits:undead_fortitude",
	}

	require.ElementsMatch(t, want, got,
		"AllTraitRefs must mirror LoadJSON's dispatch switch exactly (loader.go) — "+
			"a ref added as a new LoadJSON case must be added here too")
}
