package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// newLevel1Bard finalizes a level-1 bard with Charisma 16, so the inspiration
// pool is three.
func newLevel1Bard(t *testing.T) *Character {
	t.Helper()

	draft, err := NewDraft(&DraftConfig{ID: "bard-draft", PlayerID: "player-1"})
	require.NoError(t, err)
	require.NoError(t, draft.SetName(&SetNameInput{Name: "Scanlan"}))
	require.NoError(t, draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	require.NoError(t, draft.SetClass(&SetClassInput{
		ClassID: classes.Bard,
		Choices: ClassChoices{
			Skills:   []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
			Tools:    []shared.SelectionID{"lute", "flute", "drum"},
			Cantrips: []shared.SelectionID{spells.TrueStrike, spells.ViciousMockery},
			Spells:   []spells.Spell{spells.Bane},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BardWeaponsPrimary, OptionID: choices.BardWeaponRapier},
				{ChoiceID: choices.BardPack, OptionID: choices.BardPackDiplomat},
				{ChoiceID: choices.BardInstrument, OptionID: choices.BardInstrumentLute},
			},
		},
	}))
	require.NoError(t, draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	require.NoError(t, draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 13,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 16,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(context.Background(), "bard-1", events.NewEventBus())
	require.NoError(t, err)
	return char
}

// TestABardProjectsAStatusView is the walk finding, at its source: the bard's
// status was unavailable in the dungeon and their equipment did not render.
//
// THE WHOLE VIEW IS REFUSED OR NONE OF IT IS. This projection validates every
// feature's reported resource against a closed catalog and returns an error
// when one is outside it, so a single missing arm does not hide one pool — it
// removes the panel. Bardic Inspiration reported the pool it owns, the catalog
// had never heard of it, and every player looking at that bard saw nothing.
func TestABardProjectsAStatusView(t *testing.T) {
	char := newLevel1Bard(t)

	out, err := char.StatusView(&StatusViewInput{})

	require.NoError(t, err, "a bard's status must project like anybody else's")
	require.NotNil(t, out.View)
	require.Equal(t, 1, out.View.Level)
}

// TestTheBardsStatusCarriesTheInspirationPool — the pool the panel shows, with
// the name the rulebook authors.
func TestTheBardsStatusCarriesTheInspirationPool(t *testing.T) {
	out, err := newLevel1Bard(t).StatusView(&StatusViewInput{})
	require.NoError(t, err)

	byKey := map[coreResources.ResourceKey]ResourceView{}
	for _, resource := range out.View.Resources {
		byKey[resource.Key] = resource
	}

	inspiration, ok := byKey[resources.Inspiration]
	require.True(t, ok, "the bard's own pool is in the view")
	require.Equal(t, "Bardic Inspiration", inspiration.Name)
	require.Equal(t, 3, inspiration.Current, "Charisma 16 is a +3 modifier")
	require.Equal(t, 3, inspiration.Maximum)

	spellSlots, ok := byKey[resources.SpellSlotLevel1]
	require.True(t, ok, "the canonical spell-slot pool is in the generic resource view")
	require.Equal(t, "1st-level Spell Slots", spellSlots.Name)
	require.Equal(t, 2, spellSlots.Current)
	require.Equal(t, 2, spellSlots.Maximum)

	hitDice, ok := byKey[resources.HitDice]
	require.True(t, ok, "and the hit dice every character carries")
	require.Equal(t, 1, hitDice.Maximum)
}

// TestTheBardsFeatureNamesItsOwnPool — the feature reports the resource it
// spends, which is what the two catalogs have to agree about.
func TestTheBardsFeatureNamesItsOwnPool(t *testing.T) {
	out, err := newLevel1Bard(t).StatusView(&StatusViewInput{})
	require.NoError(t, err)

	var found bool
	for _, feature := range out.View.Features {
		if feature.Ref.String() != refs.Features.BardicInspiration().String() {
			continue
		}
		found = true
		require.Equal(t, "Bardic Inspiration", feature.Name)
		require.NotNil(t, feature.ResourceKey)
		require.Equal(t, resources.Inspiration, *feature.ResourceKey)
	}
	require.True(t, found, "the bard's one feature is in the view")
}

// TestTheBardsEquipmentProjectsToo. The walk reported the status panel
// unavailable AND no equipment; equipment has no per-class catalog of its own,
// so the second symptom was the first one's shadow. Pinned here so the pair is
// checked together rather than assumed.
func TestTheBardsEquipmentProjectsToo(t *testing.T) {
	view, err := newLevel1Bard(t).EquipmentView(context.Background())

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotEmpty(t, view.Items, "a bard finalizes carrying what they chose")
}

// TestEveryFeatureWithAPoolIsInBothCatalogsItBelongsIn is the pin that would
// have caught this before the walk did.
//
// # Two catalogs, and only one of them is about every pool
//
// featureResourceCatalog answers "which class does this feature's pool belong
// to". ownerResourceAllowed answers "which pools may this class carry IN ITS
// OWN MAP" — and that second question does not apply to a feature-private
// pool. Second Wind and Action Surge own their resources inside the feature
// object and reach the view through mergeResources' feature rows, bypassing
// the owner gate entirely; Rage, Ki and Inspiration live on the Character and
// go through it.
//
// So the table names which half each pool is in, because that distinction was
// invisible and is exactly what a new class gets wrong: a bard's pool is
// owner-owned, the arm was missing, and the whole view was refused.
func TestEveryFeatureWithAPoolIsInBothCatalogsItBelongsIn(t *testing.T) {
	for _, tc := range []struct {
		feature    *core.Ref
		ownerOwned bool
	}{
		{refs.Features.Rage(), true},
		{refs.Features.FlurryOfBlows(), true},
		{refs.Features.PatientDefense(), true},
		{refs.Features.StepOfTheWind(), true},
		{refs.Features.BardicInspiration(), true},
		// Feature-private: the feature holds the resource, the Character's own
		// map never carries it, and the owner gate is not asked.
		{refs.Features.SecondWind(), false},
		{refs.Features.ActionSurge(), false},
	} {
		class, key, known := featureResourceCatalog(*tc.feature)
		require.True(t, known, "%s has no class/pool row", tc.feature)

		name, named := resources.DisplayName(key)
		require.True(t, named, "%s has no display name", key)
		require.NotEmpty(t, name)

		require.Equal(t, tc.ownerOwned, ownerResourceAllowed(class, key),
			"%s spends %s; owner-owned is %v", tc.feature, key, tc.ownerOwned)
	}
}

// TestEveryClassWithAPoolAllowsItsHitDice — every character has hit dice, so a
// class arm that forgot them would refuse the whole view for a reason that has
// nothing to do with the class's own feature.
func TestEveryClassWithAPoolAllowsItsHitDice(t *testing.T) {
	for _, class := range []classes.Class{
		classes.Barbarian, classes.Fighter, classes.Rogue, classes.Monk, classes.Bard,
	} {
		require.True(t, ownerResourceAllowed(class, resources.HitDice),
			"%s carries hit dice like everybody else", class)
	}
}
