//nolint:dupl // Test functions follow same table-driven pattern
package refs_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/assert"
)

func TestFeaturesNamespace(t *testing.T) {
	t.Run("Rage returns correct ref", func(t *testing.T) {
		ref := refs.Features.Rage()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("features"), ref.Type)
		assert.Equal(t, core.ID("rage"), ref.ID)
	})

	t.Run("SecondWind returns correct ref", func(t *testing.T) {
		ref := refs.Features.SecondWind()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("features"), ref.Type)
		assert.Equal(t, core.ID("second_wind"), ref.ID)
	})
}

func TestConditionsNamespace(t *testing.T) {
	t.Run("Raging returns correct ref", func(t *testing.T) {
		ref := refs.Conditions.Raging()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("conditions"), ref.Type)
		assert.Equal(t, core.ID("raging"), ref.ID)
	})

	t.Run("BrutalCritical returns correct ref", func(t *testing.T) {
		ref := refs.Conditions.BrutalCritical()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("conditions"), ref.Type)
		assert.Equal(t, core.ID("brutal_critical"), ref.ID)
	})

	t.Run("UnarmoredDefense returns correct ref", func(t *testing.T) {
		ref := refs.Conditions.UnarmoredDefense()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("conditions"), ref.Type)
		assert.Equal(t, core.ID("unarmored_defense"), ref.ID)
	})
}

func TestClassesNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Barbarian", refs.Classes.Barbarian, "barbarian"},
		{"Bard", refs.Classes.Bard, "bard"},
		{"Cleric", refs.Classes.Cleric, "cleric"},
		{"Druid", refs.Classes.Druid, "druid"},
		{"Fighter", refs.Classes.Fighter, "fighter"},
		{"Monk", refs.Classes.Monk, "monk"},
		{"Paladin", refs.Classes.Paladin, "paladin"},
		{"Ranger", refs.Classes.Ranger, "ranger"},
		{"Rogue", refs.Classes.Rogue, "rogue"},
		{"Sorcerer", refs.Classes.Sorcerer, "sorcerer"},
		{"Warlock", refs.Classes.Warlock, "warlock"},
		{"Wizard", refs.Classes.Wizard, "wizard"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("classes"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestModuleConstants(t *testing.T) {
	assert.Equal(t, core.Module("dnd5e"), refs.Module)
	assert.Equal(t, core.Type("features"), refs.TypeFeatures)
	assert.Equal(t, core.Type("conditions"), refs.TypeConditions)
	assert.Equal(t, core.Type("classes"), refs.TypeClasses)
}

func TestAbilitiesNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Strength", refs.Abilities.Strength, "str"},
		{"Dexterity", refs.Abilities.Dexterity, "dex"},
		{"Constitution", refs.Abilities.Constitution, "con"},
		{"Intelligence", refs.Abilities.Intelligence, "int"},
		{"Wisdom", refs.Abilities.Wisdom, "wis"},
		{"Charisma", refs.Abilities.Charisma, "cha"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("abilities"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestWeaponsNamespace(t *testing.T) {
	// Test a representative sample
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Club", refs.Weapons.Club, "club"},
		{"Dagger", refs.Weapons.Dagger, "dagger"},
		{"Longsword", refs.Weapons.Longsword, "longsword"},
		{"Greataxe", refs.Weapons.Greataxe, "greataxe"},
		{"Shortbow", refs.Weapons.Shortbow, "shortbow"},
		{"Longbow", refs.Weapons.Longbow, "longbow"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("weapons"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestArmorNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Padded", refs.Armor.Padded, "padded"},
		{"Leather", refs.Armor.Leather, "leather"},
		{"ChainMail", refs.Armor.ChainMail, "chain-mail"},
		{"Plate", refs.Armor.Plate, "plate"},
		{"Shield", refs.Armor.Shield, "shield"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("armor"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestDamageTypesNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Slashing", refs.DamageTypes.Slashing, "slashing"},
		{"Piercing", refs.DamageTypes.Piercing, "piercing"},
		{"Bludgeoning", refs.DamageTypes.Bludgeoning, "bludgeoning"},
		{"Fire", refs.DamageTypes.Fire, "fire"},
		{"Cold", refs.DamageTypes.Cold, "cold"},
		{"Lightning", refs.DamageTypes.Lightning, "lightning"},
		{"Poison", refs.DamageTypes.Poison, "poison"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("damage_types"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestRacesNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Human", refs.Races.Human, "human"},
		{"Elf", refs.Races.Elf, "elf"},
		{"HighElf", refs.Races.HighElf, "high-elf"},
		{"Dwarf", refs.Races.Dwarf, "dwarf"},
		{"MountainDwarf", refs.Races.MountainDwarf, "mountain-dwarf"},
		{"Halfling", refs.Races.Halfling, "halfling"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("races"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestSkillsNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Acrobatics", refs.Skills.Acrobatics, "acrobatics"},
		{"Athletics", refs.Skills.Athletics, "athletics"},
		{"Stealth", refs.Skills.Stealth, "stealth"},
		{"Perception", refs.Skills.Perception, "perception"},
		{"Investigation", refs.Skills.Investigation, "investigation"},
		{"Persuasion", refs.Skills.Persuasion, "persuasion"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("skills"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestBackgroundsNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Acolyte", refs.Backgrounds.Acolyte, "acolyte"},
		{"Criminal", refs.Backgrounds.Criminal, "criminal"},
		{"Folk Hero", refs.Backgrounds.FolkHero, "folk-hero"},
		{"Noble", refs.Backgrounds.Noble, "noble"},
		{"Sage", refs.Backgrounds.Sage, "sage"},
		{"Soldier", refs.Backgrounds.Soldier, "soldier"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("backgrounds"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestLanguagesNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Common", refs.Languages.Common, "common"},
		{"Elvish", refs.Languages.Elvish, "elvish"},
		{"Dwarvish", refs.Languages.Dwarvish, "dwarvish"},
		{"Draconic", refs.Languages.Draconic, "draconic"},
		{"Infernal", refs.Languages.Infernal, "infernal"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("languages"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestFightingStyleConditions(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"FightingStyleArchery", refs.Conditions.FightingStyleArchery,
			"fighting_style_archery"},
		{"FightingStyleDefense", refs.Conditions.FightingStyleDefense,
			"fighting_style_defense"},
		{"FightingStyleDueling", refs.Conditions.FightingStyleDueling,
			"fighting_style_dueling"},
		{"FightingStyleGreatWeaponFighting", refs.Conditions.FightingStyleGreatWeaponFighting,
			"fighting_style_great_weapon_fighting"},
		{"FightingStyleProtection", refs.Conditions.FightingStyleProtection,
			"fighting_style_protection"},
		{"FightingStyleTwoWeaponFighting", refs.Conditions.FightingStyleTwoWeaponFighting,
			"fighting_style_two_weapon_fighting"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("conditions"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestToolsNamespace(t *testing.T) {
	// Test a representative sample
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"AlchemistSupplies", refs.Tools.AlchemistSupplies, "alchemist-supplies"},
		{"SmithTools", refs.Tools.SmithTools, "smith-tools"},
		{"ThievesTools", refs.Tools.ThievesTools, "thieves-tools"},
		{"Flute", refs.Tools.Flute, "flute"},
		{"DiceSet", refs.Tools.DiceSet, "dice-set"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("tools"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestSpellsNamespace(t *testing.T) {
	// Test a representative sample of cantrips and leveled spells
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Fire Bolt", refs.Spells.FireBolt, "fire-bolt"},
		{"Mage Hand", refs.Spells.MageHand, "mage-hand"},
		{"Magic Missile", refs.Spells.MagicMissile, "magic-missile"},
		{"Shield", refs.Spells.Shield, "shield"},
		{"Fireball", refs.Spells.Fireball, "fireball"},
		{"Lightning Bolt", refs.Spells.LightningBolt, "lightning-bolt"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("spells"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestExpandedConditionsNamespace(t *testing.T) {
	// Test new standard D&D 5e conditions
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Blinded", refs.Conditions.Blinded, "blinded"},
		{"Charmed", refs.Conditions.Charmed, "charmed"},
		{"Deafened", refs.Conditions.Deafened, "deafened"},
		{"Frightened", refs.Conditions.Frightened, "frightened"},
		{"Grappled", refs.Conditions.Grappled, "grappled"},
		{"Incapacitated", refs.Conditions.Incapacitated, "incapacitated"},
		{"Invisible", refs.Conditions.Invisible, "invisible"},
		{"Paralyzed", refs.Conditions.Paralyzed, "paralyzed"},
		{"Petrified", refs.Conditions.Petrified, "petrified"},
		{"Poisoned", refs.Conditions.Poisoned, "poisoned"},
		{"Prone", refs.Conditions.Prone, "prone"},
		{"Restrained", refs.Conditions.Restrained, "restrained"},
		{"Stunned", refs.Conditions.Stunned, "stunned"},
		{"Unconscious", refs.Conditions.Unconscious, "unconscious"},
		{"Exhaustion", refs.Conditions.Exhaustion, "exhaustion"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("conditions"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

// TestSingletonIdentity verifies that refs return the same pointer instance
// enabling pointer comparison (ref == refs.Abilities.Strength())
func TestSingletonIdentity(t *testing.T) {
	t.Run("Abilities return same pointer", func(t *testing.T) {
		ref1 := refs.Abilities.Strength()
		ref2 := refs.Abilities.Strength()
		assert.Same(t, ref1, ref2, "Strength() should return the same pointer")

		ref3 := refs.Abilities.Dexterity()
		ref4 := refs.Abilities.Dexterity()
		assert.Same(t, ref3, ref4, "Dexterity() should return the same pointer")

		// Different refs should be different pointers
		assert.NotSame(t, ref1, ref3, "Strength and Dexterity should be different pointers")
	})

	t.Run("Weapons return same pointer", func(t *testing.T) {
		ref1 := refs.Weapons.Longsword()
		ref2 := refs.Weapons.Longsword()
		assert.Same(t, ref1, ref2, "Longsword() should return the same pointer")

		ref3 := refs.Weapons.Greataxe()
		ref4 := refs.Weapons.Greataxe()
		assert.Same(t, ref3, ref4, "Greataxe() should return the same pointer")
	})

	t.Run("Conditions return same pointer", func(t *testing.T) {
		ref1 := refs.Conditions.Raging()
		ref2 := refs.Conditions.Raging()
		assert.Same(t, ref1, ref2, "Raging() should return the same pointer")
	})

	t.Run("Features return same pointer", func(t *testing.T) {
		ref1 := refs.Features.Rage()
		ref2 := refs.Features.Rage()
		assert.Same(t, ref1, ref2, "Rage() should return the same pointer")
	})

	t.Run("Classes return same pointer", func(t *testing.T) {
		ref1 := refs.Classes.Fighter()
		ref2 := refs.Classes.Fighter()
		assert.Same(t, ref1, ref2, "Fighter() should return the same pointer")
	})

	t.Run("Spells return same pointer", func(t *testing.T) {
		ref1 := refs.Spells.Fireball()
		ref2 := refs.Spells.Fireball()
		assert.Same(t, ref1, ref2, "Fireball() should return the same pointer")
	})
}

// TestSingletonSwitchPattern demonstrates the intended usage pattern
func TestSingletonSwitchPattern(t *testing.T) {
	// This test demonstrates how singletons enable switch on ref directly
	ref := refs.Abilities.Strength()

	var matched bool
	switch ref {
	case refs.Abilities.Strength():
		matched = true
	case refs.Abilities.Dexterity():
		matched = false
	default:
		matched = false
	}

	assert.True(t, matched, "Switch on singleton ref should work via pointer comparison")
}

// TestWeaponsByID verifies the ByID lookup returns singleton refs
func TestWeaponsByID(t *testing.T) {
	t.Run("ByID returns same pointer as named method", func(t *testing.T) {
		// This is the critical test: ByID must return the SAME singleton
		// that the named method returns, enabling pointer comparison
		longswordByID := refs.Weapons.ByID("longsword")
		longswordNamed := refs.Weapons.Longsword()
		assert.Same(t, longswordByID, longswordNamed, "ByID should return same singleton as named method")

		greataxeByID := refs.Weapons.ByID("greataxe")
		greataxeNamed := refs.Weapons.Greataxe()
		assert.Same(t, greataxeByID, greataxeNamed, "ByID should return same singleton as named method")
	})

	t.Run("ByID returns nil for unknown weapon", func(t *testing.T) {
		unknown := refs.Weapons.ByID("unknown-weapon-id")
		assert.Nil(t, unknown, "ByID should return nil for unknown weapon ID")
	})

	t.Run("ByID enables weapon pointer comparison in converters", func(t *testing.T) {
		// Simulates the weaponToRef helper usage
		weaponID := "longsword"
		ref := refs.Weapons.ByID(weaponID)

		// This should work via pointer comparison
		var matched bool
		switch ref {
		case refs.Weapons.Longsword():
			matched = true
		default:
			matched = false
		}

		assert.True(t, matched, "ByID ref should match singleton in switch")
	})
}
