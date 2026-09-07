package choices_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// TestTheBardChoosesTwoCantripsFromItsOwnList pins the level-1 cantrip
// requirement: two, from the bard's eleven.
//
// The list is worth pinning rather than assuming because the compiler that
// turns a chosen id into a content ref on the sheet does NOT consult a second
// catalog — it composes the ref from the id. This option list is therefore the
// only gate on what can end up on a sheet, which makes its exact contents a
// contract rather than a convenience.
func TestTheBardChoosesTwoCantripsFromItsOwnList(t *testing.T) {
	req := choices.GetClassRequirements(classes.Bard).Cantrips
	require.NotNil(t, req)

	require.Equal(t, choices.BardCantrips1, req.ID)
	require.Equal(t, 2, req.Count)
	require.ElementsMatch(t, []spells.Spell{
		spells.BladeWard,
		spells.DancingLights,
		spells.Friends,
		spells.Light,
		spells.MageHand,
		spells.Mending,
		spells.Message,
		spells.MinorIllusion,
		spells.Prestidigitation,
		spells.TrueStrike,
		spells.ViciousMockery,
	}, req.Options)
}

// TestTheBardChoosesFourFirstLevelSpells pins the other half: four, from the
// twenty first-level bard spells this build has refs for.
//
// Dissonant Whispers is deliberately ABSENT. It belongs on the bard's PHB list
// and this build has no ref for it; adding one here would mint content out of
// a requirement, which is the wrong direction — the ref catalog is where a
// spell starts existing, and the option list follows it.
func TestTheBardChoosesFourFirstLevelSpells(t *testing.T) {
	req := choices.GetClassRequirements(classes.Bard).Spellbook
	require.NotNil(t, req)

	require.Equal(t, choices.BardSpells1, req.ID)
	require.Equal(t, 4, req.Count)
	require.Equal(t, 1, req.SpellLevel)
	require.ElementsMatch(t, []spells.Spell{
		spells.AnimalFriendship,
		spells.Bane,
		spells.CharmPerson,
		spells.ComprehendLanguages,
		spells.CureWounds,
		spells.DetectMagic,
		spells.DisguiseSelf,
		spells.FaerieFire,
		spells.FeatherFall,
		spells.HealingWord,
		spells.Heroism,
		spells.HideousLaughter,
		spells.Identify,
		spells.IllusoryScript,
		spells.Longstrider,
		spells.SilentImage,
		spells.Sleep,
		spells.SpeakWithAnimals,
		spells.Thunderwave,
		spells.UnseenServant,
	}, req.Options)
}

// TestEveryOfferedSpellHasARef is the check that makes composing the ref safe.
// An option with no ref behind it would become a sheet entry pointing at
// content that does not exist, and nothing downstream would say so.
func TestEveryOfferedSpellHasARef(t *testing.T) {
	requirements := choices.GetClassRequirements(classes.Bard)

	offered := append([]spells.Spell{}, requirements.Cantrips.Options...)
	offered = append(offered, requirements.Spellbook.Options...)

	for _, spell := range offered {
		require.NotNil(t, refs.Spells.ByID(string(spell)),
			"the bard offers %q and this build has no spell ref for it", spell)
	}
}
