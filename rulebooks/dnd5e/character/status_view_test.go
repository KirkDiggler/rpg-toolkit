// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// TestStatusViewProjectsFighterWithoutPersistenceJSON confirms the fighter
// StatusView is built from the live sheet and feature Status reports — never
// from persistence JSON. A level-3 fighter carries Second Wind (level 1) and
// Action Surge (level 2), both feature-private resources, plus Hit Dice.
func TestStatusViewProjectsFighterWithoutPersistenceJSON(t *testing.T) {
	fighter := newLevel3Fighter(t)

	out, err := fighter.StatusView(&StatusViewInput{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.View)

	view := out.View
	require.Equal(t, fighter.GetLevel(), view.Level)
	require.Equal(t, fighter.GetHitPoints(), view.HitPoints.Current)
	require.Equal(t, fighter.GetMaxHitPoints(), view.HitPoints.Maximum)
	require.Equal(t, fighter.GetSpeed(), view.BaseSpeedFeet)

	keys := resourceKeys(view.Resources)
	require.Contains(t, keys, resources.SecondWind, "Second Wind is feature-private and must appear")
	require.Contains(t, keys, resources.ActionSurge, "Action Surge is feature-private and must appear")
	require.Contains(t, keys, resources.HitDice, "Hit Dice is owner-owned and must appear")

	// Exactly one row per key — no duplicates.
	require.Equal(t, len(keys), len(uniqueKeys(keys)), "resources must not duplicate keys")

	// The fighter's fighting style condition is projected as a condition view.
	require.NotEmpty(t, view.Conditions)
	require.Equal(t,
		refs.Conditions.FightingStyleDefense().String(),
		view.Conditions[0].Ref.String(),
	)

	// Features are sorted by ref string: action_surge before second_wind.
	require.NotEmpty(t, view.Features)
	require.Equal(t,
		refs.Features.ActionSurge().String(),
		view.Features[0].Ref.String(),
	)
}

// TestStatusViewMonkReportsOneKiRowDespiteThreeConsumers confirms the level-3
// monk projects exactly one resources.Ki row even though Flurry of Blows,
// Patient Defense, and Step of the Wind all report the shared Ki pool.
func TestStatusViewMonkReportsOneKiRowDespiteThreeConsumers(t *testing.T) {
	monk := newLevel3Monk(t)

	out, err := monk.StatusView(&StatusViewInput{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.View)

	keys := resourceKeys(out.View.Resources)
	kiRows := 0
	for _, k := range keys {
		if k == resources.Ki {
			kiRows++
		}
	}
	require.Equal(t, 1, kiRows, "exactly one Ki row despite three consuming features")

	// The single Ki row carries the owner's level-3 maximum.
	var kiRow *ResourceView
	for i := range out.View.Resources {
		if out.View.Resources[i].Key == resources.Ki {
			kiRow = &out.View.Resources[i]
			break
		}
	}
	require.NotNil(t, kiRow)
	require.Equal(t, "Ki", kiRow.Name)
	require.Equal(t, 3, kiRow.Maximum)

	// Three Ki-consuming features are each projected, plus Deflect Missiles.
	featureRefs := refStrings(featuresToRefs(out.View.Features))
	require.Contains(t, featureRefs, refs.Features.FlurryOfBlows().String())
	require.Contains(t, featureRefs, refs.Features.PatientDefense().String())
	require.Contains(t, featureRefs, refs.Features.StepOfTheWind().String())
	require.Contains(t, featureRefs, refs.Features.DeflectMissiles().String())

	// Features are deterministically sorted by ref string.
	require.Equal(t, sortedCopy(featureRefs), featureRefs)
}

// TestStatusViewBarbarianProjectsRageChargesAndRecklessAttack confirms the
// barbarian's owner-owned RageCharges dedupes against the Rage feature's
// report (same key, same facts) and Reckless Attack projects without a
// resource.
func TestStatusViewBarbarianProjectsRageChargesAndRecklessAttack(t *testing.T) {
	barbarian := newLevel3Barbarian(t)

	out, err := barbarian.StatusView(&StatusViewInput{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.View)

	keys := resourceKeys(out.View.Resources)
	require.Contains(t, keys, resources.RageCharges)
	require.Contains(t, keys, resources.HitDice)

	rageRows := 0
	for _, k := range keys {
		if k == resources.RageCharges {
			rageRows++
		}
	}
	require.Equal(t, 1, rageRows, "RageCharges dedupes to one row")

	featureRefs := refStrings(featuresToRefs(out.View.Features))
	require.Contains(t, featureRefs, refs.Features.Rage().String())
	require.Contains(t, featureRefs, refs.Features.RecklessAttack().String())

	// Reckless Attack owns no resource, so its FeatureView has a nil key.
	for _, f := range out.View.Features {
		if f.Ref.String() == refs.Features.RecklessAttack().String() {
			require.Nil(t, f.ResourceKey, "Reckless Attack owns no resource")
		}
	}
}

// TestStatusViewRogueProjectsSneakAttackCondition confirms the rogue's Sneak
// Attack condition (carried under a feature ref) is projected via the
// rulebook-owned display catalog, and the rogue has no feature-private
// resources.
func TestStatusViewRogueProjectsSneakAttackCondition(t *testing.T) {
	rogue := newLevel3Rogue(t)

	out, err := rogue.StatusView(&StatusViewInput{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.View)

	// Sneak Attack is a condition named by a feature ref.
	foundSneakAttack := false
	for _, c := range out.View.Conditions {
		if c.Ref.String() == refs.Features.SneakAttack().String() {
			require.Equal(t, "Sneak Attack", c.Name)
			foundSneakAttack = true
		}
	}
	require.True(t, foundSneakAttack, "Sneak Attack condition must be projected")

	// Rogue has only the owner-owned Hit Dice resource.
	keys := resourceKeys(out.View.Resources)
	require.ElementsMatch(t, []coreResources.ResourceKey{resources.HitDice}, keys)
}

// TestStatusViewRejectsConflictingDuplicateResourceKey confirms that a
// feature-reported resource whose key matches an owner row but whose counts
// differ fails loudly with no partial view.
func TestStatusViewRejectsConflictingDuplicateResourceKey(t *testing.T) {
	// Drive the merge directly: an owner Ki row that disagrees with a
	// feature-reported Ki row must fail loudly with no partial output.
	ownerRows := []resourceReport{
		{Key: resources.Ki, Name: "Ki", Current: 3, Maximum: 3},
	}
	reports := []resourceReport{
		{Key: resources.Ki, Name: "Ki", Current: 2, Maximum: 3},
	}

	views, err := mergeResources(ownerRows, reports)
	require.Error(t, err)
	require.Nil(t, views, "no partial output on conflict")
}

// TestStatusViewRejectsNegativeResource confirms a negative current or maximum
// on an owner row fails without partial output.
func TestStatusViewRejectsNegativeResource(t *testing.T) {
	_, err := mergeResources(
		[]resourceReport{{Key: resources.Ki, Name: "Ki", Current: -1, Maximum: 3}},
		nil,
	)
	require.Error(t, err)
}

// TestStatusViewRejectsCurrentAboveMaximum confirms current > maximum fails.
func TestStatusViewRejectsCurrentAboveMaximum(t *testing.T) {
	_, err := mergeResources(
		[]resourceReport{{Key: resources.Ki, Name: "Ki", Current: 4, Maximum: 3}},
		nil,
	)
	require.Error(t, err)
}

func TestStatusViewRejectsUnknownSpellLikeOwnerResourceKey(t *testing.T) {
	fighter := newLevel3Fighter(t)
	fighter.resources[coreResources.ResourceKey("spell_slots")] = fighter.resources[resources.HitDice]

	out, err := fighter.StatusView(&StatusViewInput{})
	require.Error(t, err)
	require.Nil(t, out)
}

func TestStatusViewRejectsCrossClassOwnerResourceKey(t *testing.T) {
	fighter := newLevel3Fighter(t)
	fighter.resources[resources.Ki] = fighter.resources[resources.HitDice]

	out, err := fighter.StatusView(&StatusViewInput{})
	require.Error(t, err)
	require.Nil(t, out)
}

func TestStatusViewRejectsFeatureResourceKeyOrNameMismatch(t *testing.T) {
	tests := []struct {
		name     string
		resource features.ResourceStatus
	}{
		{
			name: "wrong key",
			resource: features.ResourceStatus{
				Key: resources.ActionSurge, Name: "Action Surge", Current: 1, Maximum: 1,
			},
		},
		{
			name: "wrong name",
			resource: features.ResourceStatus{
				Key: resources.SecondWind, Name: "Wind Points", Current: 1, Maximum: 1,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fighter := newLevel3Fighter(t)
			resource := tc.resource
			fighter.features = append(fighter.features, &reportedStatusFeature{status: features.Status{
				Ref: *refs.Features.SecondWind(), Name: "Second Wind", Resource: &resource,
			}})

			out, err := fighter.StatusView(&StatusViewInput{})
			require.Error(t, err)
			require.Nil(t, out)
		})
	}
}

func TestStatusViewRejectsFeaturePrivateResourceOnWrongClass(t *testing.T) {
	monk := newLevel3Monk(t)
	monk.features = append(monk.features, &reportedStatusFeature{status: features.Status{
		Ref:  *refs.Features.ActionSurge(),
		Name: "Action Surge",
		Resource: &features.ResourceStatus{
			Key: resources.ActionSurge, Name: "Action Surge", Current: 1, Maximum: 1,
		},
	}})

	out, err := monk.StatusView(&StatusViewInput{})
	require.Error(t, err)
	require.Nil(t, out)
}

func TestStatusViewRejectsSameKeyAndCountsWithConflictingName(t *testing.T) {
	views, err := mergeResources(
		[]resourceReport{{Key: resources.Ki, Name: "Ki", Current: 3, Maximum: 3}},
		[]resourceReport{{Key: resources.Ki, Name: "Focus", Current: 3, Maximum: 3}},
	)
	require.Error(t, err)
	require.Nil(t, views)
}

// TestStatusViewRejectsUnknownConditionRef confirms an unknown condition ref
// returns an error and no partial view.
func TestStatusViewRejectsUnknownConditionRef(t *testing.T) {
	fighter := newLevel3Fighter(t)

	// Append a condition with a ref the display catalog does not know.
	fighter.conditions = append(fighter.conditions, &stubCondition{
		ref: &core.Ref{Module: refs.Module, Type: refs.TypeConditions, ID: "totally_unknown"},
	})

	out, err := fighter.StatusView(&StatusViewInput{})
	require.Error(t, err)
	require.Nil(t, out, "no partial output on unknown condition ref")
}

// TestStatusViewRejectsMalformedFeatureStatus confirms a feature whose Status
// returns an error fails the whole projection without partial output.
func TestStatusViewRejectsMalformedFeatureStatus(t *testing.T) {
	fighter := newLevel3Fighter(t)
	// Inject a feature that always errors on Status. It implements the full
	// Feature surface via stubStatusFeature.
	fighter.features = append(fighter.features, &malformedStatusFeature{})

	out, err := fighter.StatusView(&StatusViewInput{})
	require.Error(t, err)
	require.Nil(t, out, "no partial output on malformed feature status")
}

// TestStatusViewExcludesSpellSlotsAndClassResources confirms that non-empty
// SpellSlots and legacy ClassResources never surface as status resources —
// excluded by construction, locked by test.
func TestStatusViewExcludesSpellSlotsAndClassResources(t *testing.T) {
	fighter := newLevel3Fighter(t)

	fighter.spellSlots = map[int]SpellSlotData{
		1: {Max: 2, Used: 0},
	}
	fighter.classResources = map[shared.ClassResourceType]ResourceData{
		shared.ClassResourceType(1): {Name: "sorcery_points", Current: 1, Max: 1},
	}

	out, err := fighter.StatusView(&StatusViewInput{})
	require.NoError(t, err)
	require.NotNil(t, out)

	for _, r := range out.View.Resources {
		require.NotEqual(t, coreResources.ResourceKey("spell_slots"), r.Key)
		require.NotEqual(t, coreResources.ResourceKey("sorcery_points"), r.Key)
	}
}

// TestStatusViewConditionsSortedByRef confirms conditions are sorted by ref.
func TestStatusViewConditionsSortedByRef(t *testing.T) {
	fighter := newLevel3Fighter(t)
	// Add a second known condition (Dodging) alongside the fighting style.
	fighter.conditions = append(fighter.conditions, mustNewDodgingCondition(fighter.GetID()))

	out, err := fighter.StatusView(&StatusViewInput{})
	require.NoError(t, err)

	condRefs := refStrings(conditionsToRefs(out.View.Conditions))
	require.Equal(t, sortedCopy(condRefs), condRefs, "conditions sorted by ref string")
}

// TestStatusViewResourcesSortedByKey confirms resources are sorted by key.
func TestStatusViewResourcesSortedByKey(t *testing.T) {
	monk := newLevel3Monk(t)

	out, err := monk.StatusView(&StatusViewInput{})
	require.NoError(t, err)

	keys := resourceKeys(out.View.Resources)
	require.Equal(t, sortedKeysCopy(keys), keys, "resources sorted by key")
}

// TestStatusViewReturnsDetachedValues confirms mutating the sheet after
// projecting does not change the previously returned view.
func TestStatusViewReturnsDetachedValues(t *testing.T) {
	fighter := newLevel3Fighter(t)

	out, err := fighter.StatusView(&StatusViewInput{})
	require.NoError(t, err)
	require.NotNil(t, out)

	hpBefore := out.View.HitPoints.Current
	// Mutate the live sheet.
	fighter.hitPoints = fighter.hitPoints - 1
	require.Equal(t, hpBefore, out.View.HitPoints.Current, "view is detached from the live sheet")
}

// --- helpers ---

// newLevel3Fighter finalizes the existing Fighter draft fixture, then promotes
// the sheet to level 3 by re-running class resource initialization and adding
// the level-2 Action Surge feature through the features factory — never by
// hand-authoring a runtime feature object.
func newLevel3Fighter(t *testing.T) *Character {
	t.Helper()
	draft := newFighterDraft(t)
	char, err := draft.ToCharacter(context.Background(), "fighter-3", events.NewEventBus())
	require.NoError(t, err)
	require.NotNil(t, char)

	promoteToLevel(t, draft, char, 3)

	// Action Surge is a level-2 fighter feature.
	actionSurge, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.ActionSurge().String(),
		CharacterID: char.GetID(),
	})
	require.NoError(t, err)
	char.features = append(char.features, actionSurge.Feature)
	return char
}

// newLevel3Barbarian finalizes the Barbarian draft fixture and promotes to
// level 3, adding the level-2 Reckless Attack feature.
func newLevel3Barbarian(t *testing.T) *Character {
	t.Helper()
	draft := newBarbarianDraft(t)
	char, err := draft.ToCharacter(context.Background(), "barbarian-3", events.NewEventBus())
	require.NoError(t, err)
	require.NotNil(t, char)

	promoteToLevel(t, draft, char, 3)

	reckless, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.RecklessAttack().String(),
		CharacterID: char.GetID(),
	})
	require.NoError(t, err)
	char.features = append(char.features, reckless.Feature)
	return char
}

// newLevel3Monk finalizes the Monk draft fixture and promotes to level 3,
// adding the three level-2 Ki features and the level-3 Deflect Missiles
// feature. initializeClassResources at level 3 creates the shared Ki pool.
func newLevel3Monk(t *testing.T) *Character {
	t.Helper()
	draft := newMonkDraft(t)
	char, err := draft.ToCharacter(context.Background(), "monk-3", events.NewEventBus())
	require.NoError(t, err)
	require.NotNil(t, char)

	promoteToLevel(t, draft, char, 3)

	for _, ref := range []*core.Ref{
		refs.Features.FlurryOfBlows(),
		refs.Features.PatientDefense(),
		refs.Features.StepOfTheWind(),
		refs.Features.DeflectMissiles(),
	} {
		out, err := features.CreateFromRef(&features.CreateFromRefInput{
			Ref:         ref.String(),
			CharacterID: char.GetID(),
		})
		require.NoError(t, err)
		char.features = append(char.features, out.Feature)
	}
	return char
}

// newLevel3Rogue finalizes the Rogue draft fixture and promotes to level 3.
// The rogue carries the Sneak Attack condition and no feature-private
// resources through level 3.
func newLevel3Rogue(t *testing.T) *Character {
	t.Helper()
	draft := newRogueDraft(t)
	char, err := draft.ToCharacter(context.Background(), "rogue-3", events.NewEventBus())
	require.NoError(t, err)
	require.NotNil(t, char)

	promoteToLevel(t, draft, char, 3)
	return char
}

// promoteToLevel bumps a finalized level-1 sheet to the given level and
// re-runs class resource initialization so owner-owned pools (Hit Dice, Ki,
// RageCharges) match the new level.
func promoteToLevel(t *testing.T, draft *Draft, char *Character, level int) {
	t.Helper()
	char.level = level
	// Re-run resource initialization against the new level. The draft's class
	// is what gates which pools are created.
	draft.initializeClassResources(char)
}

// newFighterDraft mirrors the fighter_finalize_test fixture: a Human Fighter
// with the Defense fighting style.
func newFighterDraft(t *testing.T) *Draft {
	t.Helper()
	draft, err := NewDraft(&DraftConfig{ID: "fighter-draft", PlayerID: "player-1"})
	require.NoError(t, err)

	require.NoError(t, draft.SetName(&SetNameInput{Name: "Arthur"}))
	require.NoError(t, draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Common}},
	}))
	require.NoError(t, draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.History},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
			FightingStyle: fightingstyles.Defense,
		},
	}))
	require.NoError(t, draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Soldier}))
	require.NoError(t, draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 15, abilities.DEX: 14, abilities.CON: 13,
			abilities.INT: 12, abilities.WIS: 10, abilities.CHA: 8,
		},
		Method: "standard-array",
	}))
	return draft
}

// newBarbarianDraft mirrors the barbarian_finalize_test fixture.
func newBarbarianDraft(t *testing.T) *Draft {
	t.Helper()
	draft, err := NewDraft(&DraftConfig{ID: "barbarian-draft", PlayerID: "player-1"})
	require.NoError(t, err)

	require.NoError(t, draft.SetName(&SetNameInput{Name: "Grog"}))
	require.NoError(t, draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	require.NoError(t, draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	require.NoError(t, draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Soldier}))
	require.NoError(t, draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 15,
			abilities.INT: 8, abilities.WIS: 12, abilities.CHA: 10,
		},
		Method: "standard-array",
	}))
	return draft
}

// newMonkDraft mirrors the monk draft fixture from draft_test.go.
func newMonkDraft(t *testing.T) *Draft {
	t.Helper()
	draft, err := NewDraft(&DraftConfig{ID: "monk-draft", PlayerID: "player-1"})
	require.NoError(t, err)

	require.NoError(t, draft.SetName(&SetNameInput{Name: "Li"}))
	require.NoError(t, draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Common}},
	}))
	require.NoError(t, draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	require.NoError(t, draft.SetClass(&SetClassInput{
		ClassID: classes.Monk,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Acrobatics, skills.Stealth},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.MonkWeaponsPrimary, OptionID: choices.MonkWeaponShortsword},
				{ChoiceID: choices.MonkPack, OptionID: choices.MonkPackDungeoneer},
			},
			Tools: []shared.SelectionID{shared.SelectionID(proficiencies.ToolBrewer)},
		},
	}))
	require.NoError(t, draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 15, abilities.CON: 13,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 12,
		},
		Method: "standard-array",
	}))
	return draft
}

// newRogueDraft mirrors the rogue draft fixture from draft_test.go.
func newRogueDraft(t *testing.T) *Draft {
	t.Helper()
	draft, err := NewDraft(&DraftConfig{ID: "rogue-draft", PlayerID: "player-1"})
	require.NoError(t, err)

	require.NoError(t, draft.SetName(&SetNameInput{Name: "Vex"}))
	require.NoError(t, draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Common}},
	}))
	require.NoError(t, draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Criminal}))
	require.NoError(t, draft.SetClass(&SetClassInput{
		ClassID: classes.Rogue,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Stealth, skills.Perception,
				skills.Investigation, skills.Acrobatics,
			},
			Expertise: []skills.Skill{skills.Stealth, skills.Perception},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.RogueWeaponsPrimary, OptionID: choices.RogueWeaponRapier},
				{ChoiceID: choices.RogueWeaponsSecondary, OptionID: choices.RogueSecondaryShortbow},
				{ChoiceID: choices.RoguePack, OptionID: choices.RoguePackBurglar},
			},
		},
	}))
	require.NoError(t, draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 12, abilities.WIS: 14, abilities.CHA: 10,
		},
		Method: "standard-array",
	}))
	return draft
}

// resourceKeys collects the keys from a slice of ResourceView.
func resourceKeys(rs []ResourceView) []coreResources.ResourceKey {
	out := make([]coreResources.ResourceKey, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Key)
	}
	return out
}

func featuresToRefs(fs []FeatureView) []core.Ref {
	out := make([]core.Ref, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Ref)
	}
	return out
}

func conditionsToRefs(cs []ConditionView) []core.Ref {
	out := make([]core.Ref, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Ref)
	}
	return out
}

func refStrings(refs []core.Ref) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.String())
	}
	return out
}

func uniqueKeys(keys []coreResources.ResourceKey) []coreResources.ResourceKey {
	seen := make(map[coreResources.ResourceKey]struct{}, len(keys))
	out := make([]coreResources.ResourceKey, 0, len(keys))
	for _, k := range keys {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	// simple deterministic sort
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func sortedKeysCopy(in []coreResources.ResourceKey) []coreResources.ResourceKey {
	out := append([]coreResources.ResourceKey(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// mustNewDodgingCondition creates a Dodging condition for the projection tests.
func mustNewDodgingCondition(characterID string) dnd5eEvents.ConditionBehavior {
	return conditions.NewDodgingCondition(characterID)
}

// stubCondition is a ConditionBehavior that names itself by an arbitrary ref,
// used to exercise the unknown-ref error path.
type stubCondition struct {
	ref *core.Ref
}

func (s *stubCondition) Ref() *core.Ref                                { return s.ref }
func (s *stubCondition) IsApplied() bool                               { return true }
func (s *stubCondition) Apply(context.Context, events.EventBus) error  { return nil }
func (s *stubCondition) Remove(context.Context, events.EventBus) error { return nil }
func (s *stubCondition) ToJSON() (json.RawMessage, error)              { return nil, nil }

// malformedStatusFeature is a Feature whose Status always errors, used to
// exercise the malformed-feature-status error path. It implements the full
// features.Feature surface with no-op activations.
type malformedStatusFeature struct{}

func (m *malformedStatusFeature) GetID() string { return "malformed" }
func (m *malformedStatusFeature) GetType() core.EntityType {
	return core.EntityType("feature")
}
func (m *malformedStatusFeature) CanActivate(context.Context, core.Entity, features.FeatureInput) error {
	return nil
}
func (m *malformedStatusFeature) Activate(context.Context, core.Entity, features.FeatureInput) error {
	return nil
}
func (m *malformedStatusFeature) Ref() *core.Ref {
	return &core.Ref{Module: refs.Module, Type: refs.TypeFeatures, ID: "malformed"}
}
func (m *malformedStatusFeature) Name() string                      { return "Malformed" }
func (m *malformedStatusFeature) ToJSON() (json.RawMessage, error)  { return nil, nil }
func (m *malformedStatusFeature) ActionType() coreCombat.ActionType { return "" }
func (m *malformedStatusFeature) Status(*features.StatusInput) (*features.StatusOutput, error) {
	return nil, errMalformedStatusForTest
}

// reportedStatusFeature supplies one authored status report so catalog
// rejection tests can exercise provider key/name/class mismatches end to end.
type reportedStatusFeature struct {
	malformedStatusFeature
	status features.Status
}

func (f *reportedStatusFeature) GetID() string  { return f.status.Ref.ID }
func (f *reportedStatusFeature) Ref() *core.Ref { return &f.status.Ref }
func (f *reportedStatusFeature) Name() string   { return f.status.Name }
func (f *reportedStatusFeature) Status(*features.StatusInput) (*features.StatusOutput, error) {
	status := f.status
	return &features.StatusOutput{Status: &status}, nil
}

// errMalformedStatusForTest is the sentinel a malformedStatusFeature returns.
var errMalformedStatusForTest = newSentinelErr("malformed feature status for test")

func newSentinelErr(msg string) error {
	return &sentinelError{msg: msg}
}

type sentinelError struct{ msg string }

func (e *sentinelError) Error() string { return e.msg }
