package character_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

func ptr[T any](value T) *T {
	return &value
}

func completeAppearance() *customization.Appearance {
	return &customization.Appearance{
		Hair: &customization.HairCustomization{
			Scalp: &customization.StyleSelection{
				Kind:     customization.StyleSelectionStyle,
				StyleRef: "provider:hair:38",
			},
			FacialHair: &customization.StyleSelection{
				Kind: customization.StyleSelectionNone,
			},
			ColorSRGB: ptr(uint32(0)),
			Roughness: ptr(float32(0)),
		},
		Outfit: &customization.OutfitCustomization{
			PrimaryColorSRGB:   ptr(uint32(0x102030)),
			SecondaryColorSRGB: ptr(uint32(0xFFFFFF)),
		},
	}
}

func mutateAppearance(appearance *customization.Appearance) {
	appearance.Hair.Scalp.StyleRef = "changed:hair"
	appearance.Hair.FacialHair.Kind = customization.StyleSelectionStyle
	appearance.Hair.FacialHair.StyleRef = "changed:facial-hair"
	*appearance.Hair.ColorSRGB = 0x123456
	*appearance.Hair.Roughness = 1
	*appearance.Outfit.PrimaryColorSRGB = 0x654321
	*appearance.Outfit.SecondaryColorSRGB = 0
}

func requireAppearancePointersDetached(t *testing.T, source, copy *customization.Appearance) {
	t.Helper()
	require.NotSame(t, source, copy)
	require.NotSame(t, source.Hair, copy.Hair)
	require.NotSame(t, source.Hair.Scalp, copy.Hair.Scalp)
	require.NotSame(t, source.Hair.FacialHair, copy.Hair.FacialHair)
	require.NotSame(t, source.Hair.ColorSRGB, copy.Hair.ColorSRGB)
	require.NotSame(t, source.Hair.Roughness, copy.Hair.Roughness)
	require.NotSame(t, source.Outfit, copy.Outfit)
	require.NotSame(t, source.Outfit.PrimaryColorSRGB, copy.Outfit.PrimaryColorSRGB)
	require.NotSame(t, source.Outfit.SecondaryColorSRGB, copy.Outfit.SecondaryColorSRGB)
}

func validRogueClassInput() *character.SetClassInput {
	return &character.SetClassInput{
		ClassID: classes.Rogue,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{
				skills.Stealth,
				skills.Perception,
				skills.Investigation,
				skills.Acrobatics,
			},
			Expertise: []skills.Skill{skills.Stealth, skills.Perception},
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.RogueWeaponsPrimary, OptionID: choices.RogueWeaponRapier},
				{ChoiceID: choices.RogueWeaponsSecondary, OptionID: choices.RogueSecondaryShortbow},
				{ChoiceID: choices.RoguePack, OptionID: choices.RoguePackBurglar},
			},
		},
	}
}

func newDraft(t *testing.T) *character.Draft {
	t.Helper()
	return character.LoadDraftFromData(&character.DraftData{
		ID:       "draft-appearance",
		PlayerID: "player-appearance",
	})
}

func completeDraft(t *testing.T, appearance *customization.Appearance) *character.Draft {
	t.Helper()
	draft := newDraft(t)
	require.NoError(t, draft.SetName(&character.SetNameInput{Name: "Appearance Hero"}))
	require.NoError(t, draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 10,
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
	}))
	require.NoError(t, draft.SetRace(&character.SetRaceInput{
		RaceID: races.Human,
		Choices: character.RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	}))
	require.NoError(t, draft.SetClass(validRogueClassInput()))
	require.NoError(t, draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	}))
	require.NoError(t, draft.SetAppearance(&character.SetAppearanceInput{
		Appearance: appearance,
	}))
	return draft
}

func TestDraftSetAppearanceValidatesAtomicallyAndPreservesClassCarryover(t *testing.T) {
	draft := newDraft(t)
	inputAppearance := completeAppearance()

	require.NoError(t, draft.SetAppearance(&character.SetAppearanceInput{Appearance: inputAppearance}))
	stored := draft.Appearance()
	inputAppearance.Outfit.PrimaryColorSRGB = ptr(uint32(0xFFFFFF))
	require.Equal(t, uint32(0x102030), *stored.Outfit.PrimaryColorSRGB)
	require.Equal(t, stored, draft.Appearance())

	expected := customization.CloneAppearance(draft.Appearance())
	returned := draft.Appearance()
	mutateAppearance(returned)
	require.Equal(t, expected, draft.Appearance())

	beforeProgress := draft.Progress()
	beforeUpdatedAt := draft.UpdatedAt()
	malformed := &customization.Appearance{Hair: &customization.HairCustomization{
		Scalp: &customization.StyleSelection{Kind: customization.StyleSelectionStyle},
	}}
	expectedErr := customization.ValidateAppearance(malformed)
	err := draft.SetAppearance(&character.SetAppearanceInput{Appearance: malformed})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
	require.Equal(t, expectedErr.Error(), err.Error())
	require.Equal(t, stored, draft.Appearance())
	require.Equal(t, beforeProgress, draft.Progress())
	require.Equal(t, beforeUpdatedAt, draft.UpdatedAt())

	for _, input := range []*character.SetAppearanceInput{
		nil,
		{Appearance: nil},
	} {
		err = draft.SetAppearance(input)
		require.Error(t, err)
		require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
		require.Equal(t, stored, draft.Appearance())
		require.Equal(t, beforeProgress, draft.Progress())
		require.Equal(t, beforeUpdatedAt, draft.UpdatedAt())
	}

	replacement := &customization.Appearance{Outfit: &customization.OutfitCustomization{
		PrimaryColorSRGB: ptr(uint32(0)),
	}}
	require.NoError(t, draft.SetAppearance(&character.SetAppearanceInput{Appearance: replacement}))
	require.Equal(t, replacement, draft.Appearance())
	require.Nil(t, draft.Appearance().Hair)
	require.True(t, draft.UpdatedAt().After(beforeUpdatedAt))

	require.NoError(t, draft.SetClass(validRogueClassInput()))
	require.Equal(t, replacement, draft.Appearance(), "changing class must carry appearance over")
}

func TestDraftAppearanceRoundTripDeepCopiesEveryNestedPointer(t *testing.T) {
	draft := newDraft(t)
	require.NoError(t, draft.SetAppearance(&character.SetAppearanceInput{
		Appearance: completeAppearance(),
	}))
	expected := draft.Appearance()

	data := draft.ToData()
	requireAppearancePointersDetached(t, expected, data.Appearance)
	mutateAppearance(data.Appearance)
	require.Equal(t, expected, draft.Appearance())

	seed := draft.ToData()
	reloaded := character.LoadDraftFromData(seed)
	mutateAppearance(seed.Appearance)
	require.Equal(t, expected, reloaded.Appearance())
	require.Equal(t, expected, reloaded.ToData().Appearance)
}

func TestFinalizationCarriesAppearanceAndDeepCopiesIt(t *testing.T) {
	inputAppearance := completeAppearance()
	draft := completeDraft(t, inputAppearance)
	expected := draft.Appearance()
	bus := events.NewEventBus()

	final, err := draft.ToCharacter(context.Background(), "char-appearance", bus)
	require.NoError(t, err)
	require.Equal(t, expected, final.Appearance())

	inputAppearance.Hair.Scalp.StyleRef = "changed:input"
	require.Equal(t, expected, final.Appearance())

	returned := final.Appearance()
	mutateAppearance(returned)
	require.Equal(t, expected, final.Appearance())

	stored := final.ToData()
	requireAppearancePointersDetached(t, expected, stored.Appearance)
	mutateAppearance(stored.Appearance)
	require.Equal(t, expected, final.Appearance())
}

func TestCharacterLoadRoundTripsAppearanceAndIsolatesPersistedData(t *testing.T) {
	inputAppearance := completeAppearance()
	draft := completeDraft(t, inputAppearance)
	final, err := draft.ToCharacter(context.Background(), "char-appearance-load", events.NewEventBus())
	require.NoError(t, err)
	expected := final.Appearance()

	stored := final.ToData()
	loaded, err := character.Load(context.Background(), stored)
	require.NoError(t, err)
	require.Equal(t, expected, loaded.Appearance())
	mutateAppearance(stored.Appearance)
	require.Equal(t, expected, loaded.Appearance())

	legacyData := final.ToData()
	legacy, err := character.LoadFromData(context.Background(), legacyData, events.NewEventBus())
	require.NoError(t, err)
	require.Equal(t, expected, legacy.Appearance())
	returned := legacy.Appearance()
	mutateAppearance(returned)
	require.Equal(t, expected, legacy.Appearance())
}

func TestCharacterLoadRejectsMalformedAppearanceStrictAndLegacy(t *testing.T) {
	malformed := completeAppearance()
	malformed.Outfit.PrimaryColorSRGB = ptr(uint32(0x1000000))
	expectedErr := customization.ValidateAppearance(malformed)
	data := &character.Data{
		ID:         "bad-appearance",
		Appearance: malformed,
	}

	_, err := character.Load(context.Background(), data)
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
	require.Equal(t, expectedErr.Error(), err.Error())

	_, err = character.LoadFromData(context.Background(), data, events.NewEventBus())
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
	require.Equal(t, expectedErr.Error(), err.Error())
}

func TestCharacterAppearancePreservesPresentZeroAndNilState(t *testing.T) {
	zero := uint32(0)
	appearance := &customization.Appearance{
		Hair: &customization.HairCustomization{
			ColorSRGB: &zero,
		},
		Outfit: &customization.OutfitCustomization{
			PrimaryColorSRGB: &zero,
		},
	}
	data := &character.Data{ID: "zero-appearance", Appearance: appearance}

	loaded, err := character.Load(context.Background(), data)
	require.NoError(t, err)
	got := loaded.Appearance()
	require.NotNil(t, got)
	require.NotNil(t, got.Hair)
	require.NotNil(t, got.Hair.ColorSRGB)
	require.Zero(t, *got.Hair.ColorSRGB)
	require.Nil(t, got.Hair.Roughness)
	require.NotNil(t, got.Outfit)
	require.NotNil(t, got.Outfit.PrimaryColorSRGB)
	require.Zero(t, *got.Outfit.PrimaryColorSRGB)
	require.Nil(t, got.Outfit.SecondaryColorSRGB)
}
