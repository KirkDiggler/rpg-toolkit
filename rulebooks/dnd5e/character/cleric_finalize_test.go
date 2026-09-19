package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// ClericFinalizeSuite exercises the creation-only contribution to rpg-project#406.
// It does not attest spellcasting, preparation, or domain healing support.
type ClericFinalizeSuite struct{ suite.Suite }

func (s *ClericFinalizeSuite) classInput() *SetClassInput {
	return &SetClassInput{
		ClassID: classes.Cleric, SubclassID: classes.LifeDomain,
		Choices: ClassChoices{
			Skills:   []skills.Skill{skills.Medicine, skills.Religion},
			Cantrips: []spells.Spell{spells.SacredFlame, spells.Guidance, spells.Light},
			Spells: []spells.Spell{
				spells.Bane, spells.Command, spells.HealingWord, spells.Sanctuary,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.ClericWeapons, OptionID: choices.ClericWeaponMace},
				{ChoiceID: choices.ClericArmor, OptionID: choices.ClericArmorChainMail},
				{ChoiceID: choices.ClericSecondaryWeapon, OptionID: choices.ClericSecondaryShortbow},
				{ChoiceID: choices.ClericPack, OptionID: choices.ClericPackExplorer},
				{ChoiceID: choices.ClericHolySymbol, OptionID: choices.ClericHolyAmulet},
			},
		},
	}
}

func (s *ClericFinalizeSuite) draft(input *SetClassInput) *Draft {
	draft, err := NewDraft(&DraftConfig{ID: "cleric-draft", PlayerID: "player-1"})
	s.Require().NoError(err)
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Mercy"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.Human, Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	s.Require().NoError(draft.SetClass(input))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 10, abilities.CON: 13,
			abilities.INT: 8, abilities.WIS: 15, abilities.CHA: 12,
		}, Method: "standard-array",
	}))
	return draft
}

func (s *ClericFinalizeSuite) TestTempestWrathUsesFinalWisdomAndPersists() {
	input := s.classInput()
	input.SubclassID = classes.TempestDomain
	draft := s.draft(input)
	char, err := draft.ToCharacter(context.Background(), "tempest-wrath", events.NewEventBus())
	s.Require().NoError(err)
	data := char.ToData()
	s.Equal(3, char.GetResource(resources.WrathOfTheStorm).Maximum())
	s.Require().NoError(char.GetResource(resources.WrathOfTheStorm).Use(1))
	s.Equal(2, char.GetResource(resources.WrathOfTheStorm).Current())
	encoded, err := json.Marshal(data)
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(encoded, &stored))
	loaded, err := LoadFromData(context.Background(), &stored, events.NewEventBus())
	s.Require().NoError(err)
	s.Equal(2, loaded.GetResource(resources.WrathOfTheStorm).Current())
	s.Equal(3, loaded.GetResource(resources.WrathOfTheStorm).Maximum())
}

func (s *ClericFinalizeSuite) TestCreationAndPersistence() {
	draft := s.draft(s.classInput())
	s.True(draft.IsClassComplete(), "the selected domain must reach validation")
	encoded, err := json.Marshal(draft.ToData())
	s.Require().NoError(err)
	var saved DraftData
	s.Require().NoError(json.Unmarshal(encoded, &saved))
	char, err := LoadDraftFromData(&saved).ToCharacter(context.Background(), "cleric-1", events.NewEventBus())
	s.Require().NoError(err)
	s.Equal(2, char.GetResource(resources.SpellSlotLevel1).Current())
	s.Equal(2, char.GetResource(resources.SpellSlotLevel1).Maximum())
	s.Require().NoError(char.UseResource(resources.SpellSlotLevel1, 1))
	data := char.ToData()
	s.Equal(classes.LifeDomain, data.SubclassID)
	s.ElementsMatch([]proficiencies.Armor{
		proficiencies.ArmorLight, proficiencies.ArmorMedium, proficiencies.ArmorShields, proficiencies.ArmorHeavy,
	}, data.ArmorProficiencies)
	s.ElementsMatch([]proficiencies.Weapon{proficiencies.WeaponSimple}, data.WeaponProficiencies)
	s.Equal(10, data.MaxHitPoints, "d8 plus the Human-adjusted Constitution modifier")
	s.Contains(data.ToolProficiencies, proficiencies.ToolHerbalism, "background grants coexist")
	carried := map[string]int{}
	for _, item := range data.Inventory {
		carried[item.ID] += item.Quantity
	}
	s.Equal(1, carried[string(armor.Shield)], "fixed class grant")
	s.Equal(1, carried[string(armor.ChainMail)])
	s.Equal(1, carried[string(weapons.Mace)])
	s.Equal(1, carried[string(weapons.LightCrossbow)])
	s.Equal(1, carried["bolts-20"], "one bundle of twenty bolts")
	s.Equal(1, carried["holy-symbol"])
	s.Equal(10, carried["torch"], "the chosen explorer pack is materialized once")
	s.Equal(1, carried["herbalism-kit"], "background equipment survives")
	s.Empty(data.EquipmentSlots, "carrying armor does not equip it")
	s.ElementsMatch([]string{
		refs.Spells.SacredFlame().String(), refs.Spells.Guidance().String(), refs.Spells.Light().String(),
	}, data.KnownCantrips)
	s.ElementsMatch([]string{
		refs.Spells.Bane().String(), refs.Spells.Bless().String(), refs.Spells.Command().String(),
		refs.Spells.CureWounds().String(), refs.Spells.HealingWord().String(), refs.Spells.Sanctuary().String(),
	}, data.KnownSpells)
	encoded, err = json.Marshal(data)
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(encoded, &stored))
	stored.HitPoints = 4
	loaded, err := LoadFromData(context.Background(), &stored, events.NewEventBus())
	s.Require().NoError(err)
	back := loaded.ToData()
	s.Equal(data.SubclassID, back.SubclassID)
	s.Equal(data.ArmorProficiencies, back.ArmorProficiencies)
	s.Equal(data.WeaponProficiencies, back.WeaponProficiencies)
	s.Equal(data.Inventory, back.Inventory)
	s.Equal(data.KnownCantrips, back.KnownCantrips)
	s.Equal(data.KnownSpells, back.KnownSpells)
	for _, spell := range loaded.KnownSpells() {
		definition := loaded.CastDefinition(spells.Spell(spell.ID))
		s.Require().NotNil(definition)
		s.Require().NoError(definition.Validate())
	}
	s.Equal(data.Resources, back.Resources)
	s.Equal(4, back.HitPoints, "loading is not a rest")
	s.Equal(1, loaded.GetResource(resources.SpellSlotLevel1).Current())
	s.Require().NoError(loaded.LongRest(context.Background()))
	s.Equal(2, loaded.GetResource(resources.SpellSlotLevel1).Current())
	s.Equal(data.KnownSpells, loaded.ToData().KnownSpells, "rest restores slots without choosing spells")
}

func (s *ClericFinalizeSuite) TestStatusProjectionAfterFinalizationAndReload() {
	char, err := s.draft(s.classInput()).ToCharacter(context.Background(), "cleric-status", events.NewEventBus())
	s.Require().NoError(err)
	out, err := char.StatusView(&StatusViewInput{})
	s.Require().NoError(err)
	s.Equal([]ResourceView{
		{Key: resources.HitDice, Name: "Hit Dice", Current: 1, Maximum: 1},
		{Key: resources.SpellSlotLevel1, Name: "1st-level Spell Slots", Current: 2, Maximum: 2},
	}, out.View.Resources)

	s.Require().NoError(char.UseResource(resources.SpellSlotLevel1, 1))
	for _, source := range []string{"caster-a", "caster-b"} {
		blessed, createErr := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
			MemberID: char.GetID(), SourceID: source, SourceRef: refs.Spells.Bless(),
		})
		s.Require().NoError(createErr)
		char.conditions = append(char.conditions, blessed)
	}
	baned, err := conditions.NewBanedCondition(conditions.NewBanedConditionInput{
		MemberID: char.GetID(), SourceID: "caster-c", SourceRef: refs.Spells.Bane(),
	})
	s.Require().NoError(err)
	char.conditions = append(char.conditions, baned)
	encoded, err := json.Marshal(char.ToData())
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(encoded, &stored))
	loaded, err := LoadFromData(context.Background(), &stored, events.NewEventBus())
	s.Require().NoError(err)
	out, err = loaded.StatusView(&StatusViewInput{})
	s.Require().NoError(err)
	s.Equal(1, out.View.Resources[1].Current, "projection preserves spent slots")
	sources := map[string][]string{}
	for _, condition := range out.View.Conditions {
		s.Require().NotNil(condition.SourceMember)
		sources[condition.Ref.String()] = append(sources[condition.Ref.String()], *condition.SourceMember)
	}
	s.ElementsMatch([]string{"caster-a", "caster-b"}, sources[refs.Conditions.Blessed().String()])
	s.Equal([]string{"caster-c"}, sources[refs.Conditions.Baned().String()])
	s.Require().NoError(loaded.LongRest(context.Background()))
	rested, err := loaded.StatusView(&StatusViewInput{})
	s.Require().NoError(err)
	s.Equal(2, rested.View.Resources[1].Current)
	s.Empty(rested.View.Conditions)
	s.Equal(1, out.View.Resources[1].Current, "prior projection is detached")
}

func (s *ClericFinalizeSuite) TestStatusProjectionStillRejectsCrossClassResources() {
	char, err := s.draft(s.classInput()).ToCharacter(context.Background(), "cleric-invalid-resource", events.NewEventBus())
	s.Require().NoError(err)
	char.resources[resources.Inspiration] = char.resources[resources.HitDice]
	out, err := char.StatusView(&StatusViewInput{})
	s.Require().ErrorContains(err, "not in the cleric status-view owner catalog")
	s.Nil(out, "invalid resources must not produce a partial sheet")
}

func (s *ClericFinalizeSuite) TestStabilizationPersistsWithoutHealingAndAllowsLaterDamageAndHealing() {
	ctx := context.Background()
	char, err := s.draft(s.classInput()).ToCharacter(ctx, "stabilize-cleric", events.NewEventBus())
	s.Require().NoError(err)
	char.ApplyDamage(ctx, &combat.ApplyDamageInput{
		Instances: []combat.DamageInstance{{Amount: char.GetMaxHitPoints(), Type: "slashing"}},
	})
	char.deathSaveState = &saves.DeathSaveState{Successes: 1, Failures: 2}
	markSaved(char)
	resourcesBefore := char.ToData().Resources
	s.True(char.CanStabilize())
	result, err := char.Stabilize()
	s.Require().NoError(err)
	s.Equal(combat.LifeStateDying, result.Before)
	s.Equal(combat.LifeStateStabilized, result.After)
	s.Zero(result.HitPoints)
	s.Zero(result.Progress.Successes)
	s.Zero(result.Progress.Failures)
	s.True(result.Progress.Stabilized)
	s.True(char.IsDirty())
	s.Equal(resourcesBefore, char.ToData().Resources)
	s.False(CanMakeDeathSave(char))
	s.True(combat.ParticipationFor(char.ParticipationView().LifeState).AutoPassesTurn)

	encoded, err := json.Marshal(char.ToData())
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(encoded, &stored))
	bus := events.NewEventBus()
	loaded, err := LoadFromData(ctx, &stored, bus)
	s.Require().NoError(err)
	view, err := loaded.StatusView(&StatusViewInput{})
	s.Require().NoError(err)
	s.Equal(combat.LifeStateStabilized, view.View.LifeState)
	s.Zero(view.View.HitPoints.Current)
	s.True(loaded.CanStabilize(), "already stable remains eligible")
	repeated, err := loaded.Stabilize()
	s.Require().NoError(err)
	s.Equal(combat.LifeStateStabilized, repeated.Before)
	s.Equal(result.Progress, repeated.Progress)
	loaded.ApplyDamage(ctx, &combat.ApplyDamageInput{
		Instances: []combat.DamageInstance{{Amount: 1, Type: "slashing"}},
	})
	s.Equal(combat.LifeStateDying, loaded.ParticipationView().LifeState)
	s.Equal(1, loaded.GetDeathSaveState().Failures)
	s.Zero(loaded.GetDeathSaveState().Successes)
	s.True(result.Progress.Stabilized, "returned progress is detached")
	_, err = loaded.Stabilize()
	s.Require().NoError(err)
	s.Require().NoError(dnd5eEvents.HealingReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: loaded.GetID(), Amount: 1, Source: "test-healing",
	}))
	s.Equal(combat.LifeStateConscious, loaded.ParticipationView().LifeState)
	s.False(loaded.CanStabilize())
}

func (s *ClericFinalizeSuite) TestStabilizationRejectsIneligibleRecipientsWithoutMutation() {
	for _, state := range []string{"conscious", "dead"} {
		s.Run(state, func() {
			char, err := s.draft(s.classInput()).ToCharacter(context.Background(), "invalid-stabilize", events.NewEventBus())
			s.Require().NoError(err)
			if state == "dead" {
				char.hitPoints = 0
				char.deathSaveState = &saves.DeathSaveState{Failures: 3, Dead: true}
			}
			markSaved(char)
			before := char.ToData()
			s.False(char.CanStabilize())
			out, err := char.Stabilize()
			s.Require().Error(err)
			s.Nil(out)
			after := char.ToData()
			// ToData stamps serialization time even when no game state changed.
			after.UpdatedAt = before.UpdatedAt
			s.Equal(before, after)
			s.False(char.IsDirty())
		})
	}
	var absent *Character
	s.False(absent.CanStabilize())
	out, err := absent.Stabilize()
	s.Require().Error(err)
	s.Nil(out)
}

func (s *ClericFinalizeSuite) TestExistingSheetDoesNotReceiveImplicitSpellGrantsOnLoad() {
	char, err := s.draft(s.classInput()).ToCharacter(context.Background(), "older-cleric", events.NewEventBus())
	s.Require().NoError(err)
	data := char.ToData()
	data.KnownSpells = nil
	delete(data.Resources, resources.SpellSlotLevel1)
	loaded, err := Load(context.Background(), data)
	s.Require().NoError(err)
	s.Empty(loaded.KnownSpells())
	s.NotContains(loaded.ToData().Resources, resources.SpellSlotLevel1)
}

func (s *ClericFinalizeSuite) TestChosenSpareTheDyingCompilesAfterDraftAndCharacterReload() {
	input := s.classInput()
	input.Choices.Cantrips = []spells.Spell{spells.SacredFlame, spells.SpareTheDying, spells.Guidance}
	draft := s.draft(input)
	raw, err := json.Marshal(draft.ToData())
	s.Require().NoError(err)
	var storedDraft DraftData
	s.Require().NoError(json.Unmarshal(raw, &storedDraft))
	char, err := LoadDraftFromData(&storedDraft).ToCharacter(context.Background(), "stabilizing-cleric", events.NewEventBus())
	s.Require().NoError(err)
	raw, err = json.Marshal(char.ToData())
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(raw, &stored))
	loaded, err := Load(context.Background(), &stored)
	s.Require().NoError(err)
	s.Len(loaded.KnownCantrips(), 3)
	s.Contains(stored.KnownCantrips, refs.Spells.SpareTheDying().String())
	definition := loaded.CastDefinition(spells.SpareTheDying)
	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.True(definition.Cast.Stabilize)
	s.Empty(definition.Cost.Pools)
	s.Equal(2, loaded.GetResource(resources.SpellSlotLevel1).Current())
	_, err = loaded.StatusView(&StatusViewInput{})
	s.NoError(err, "private-sheet projection accepts the normally finalized cleric")
}

func (s *ClericFinalizeSuite) TestChosenResistanceCompilesAfterDraftAndCharacterReload() {
	input := s.classInput()
	input.Choices.Cantrips = []spells.Spell{spells.SacredFlame, spells.Guidance, spells.Resistance}
	draft := s.draft(input)
	raw, err := json.Marshal(draft.ToData())
	s.Require().NoError(err)
	var storedDraft DraftData
	s.Require().NoError(json.Unmarshal(raw, &storedDraft))
	char, err := LoadDraftFromData(&storedDraft).ToCharacter(context.Background(), "resistant-cleric", events.NewEventBus())
	s.Require().NoError(err)
	raw, err = json.Marshal(char.ToData())
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(raw, &stored))
	loaded, err := Load(context.Background(), &stored)
	s.Require().NoError(err)
	s.Len(loaded.KnownCantrips(), 3)
	s.Contains(stored.KnownCantrips, refs.Spells.Resistance().String())
	definition := loaded.CastDefinition(spells.Resistance)
	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Nil(definition.Cast.Save, "the saving throw the die joins is the saver's own, not this cast's gate")
	s.Require().NotNil(definition.Cast.Concentration)
	s.Empty(definition.Cost.Pools)
	_, err = loaded.StatusView(&StatusViewInput{})
	s.NoError(err, "private-sheet projection accepts the normally finalized cleric")
}

func (s *ClericFinalizeSuite) TestChosenTollTheDeadCompilesAfterDraftAndCharacterReload() {
	input := s.classInput()
	input.Choices.Cantrips = []spells.Spell{spells.SacredFlame, spells.Guidance, spells.TollTheDead}
	draft := s.draft(input)
	raw, err := json.Marshal(draft.ToData())
	s.Require().NoError(err)
	var storedDraft DraftData
	s.Require().NoError(json.Unmarshal(raw, &storedDraft))
	char, err := LoadDraftFromData(&storedDraft).ToCharacter(context.Background(), "tolling-cleric", events.NewEventBus())
	s.Require().NoError(err)
	raw, err = json.Marshal(char.ToData())
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(raw, &stored))
	loaded, err := Load(context.Background(), &stored)
	s.Require().NoError(err)
	s.Len(loaded.KnownCantrips(), 3)
	s.Contains(stored.KnownCantrips, refs.Spells.TollTheDead().String())
	definition := loaded.CastDefinition(spells.TollTheDead)
	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Require().NotNil(definition.Cast.Save)
	s.Equal([]abilities.Ability{abilities.WIS}, definition.Cast.Save.Abilities)
	s.Require().Len(definition.Cast.Damage, 1)
	s.Equal("1d8", definition.Cast.Damage[0].Dice)
	s.Require().Len(definition.Cast.DamageIfInjured, 1)
	s.Equal("1d12", definition.Cast.DamageIfInjured[0].Dice)
	s.Empty(definition.Cost.Pools)
	_, err = loaded.StatusView(&StatusViewInput{})
	s.NoError(err, "private-sheet projection accepts the normally finalized cleric")
}

func (s *ClericFinalizeSuite) TestChosenWordOfRadianceCompilesAfterDraftAndCharacterReload() {
	input := s.classInput()
	input.Choices.Cantrips = []spells.Spell{spells.SacredFlame, spells.Guidance, spells.WordOfRadiance}
	draft := s.draft(input)
	raw, err := json.Marshal(draft.ToData())
	s.Require().NoError(err)
	var storedDraft DraftData
	s.Require().NoError(json.Unmarshal(raw, &storedDraft))
	char, err := LoadDraftFromData(&storedDraft).ToCharacter(context.Background(), "radiant-cleric", events.NewEventBus())
	s.Require().NoError(err)
	raw, err = json.Marshal(char.ToData())
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(raw, &stored))
	loaded, err := Load(context.Background(), &stored)
	s.Require().NoError(err)
	s.Len(loaded.KnownCantrips(), 3)
	s.Contains(stored.KnownCantrips, refs.Spells.WordOfRadiance().String())
	definition := loaded.CastDefinition(spells.WordOfRadiance)
	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Require().NotNil(definition.Cast.Save)
	s.Equal([]abilities.Ability{abilities.CON}, definition.Cast.Save.Abilities)
	s.Require().Len(definition.Cast.Damage, 1)
	s.Equal("1d6", definition.Cast.Damage[0].Dice)
	s.Equal(1, definition.Cast.MinTargets)
	s.Equal(32, definition.Cast.MaxTargets)
	s.Empty(definition.Cost.Pools)
	_, err = loaded.StatusView(&StatusViewInput{})
	s.NoError(err, "private-sheet projection accepts the normally finalized cleric")
}

func (s *ClericFinalizeSuite) TestKnownSacredFlameUsesTheClericsWisdomAfterReload() {
	draft := s.draft(s.classInput())
	char, err := draft.ToCharacter(context.Background(), "sacred-flame-cleric", events.NewEventBus())
	s.Require().NoError(err)
	loaded, err := LoadFromData(context.Background(), char.ToData(), events.NewEventBus())
	s.Require().NoError(err)
	s.Equal(13, loaded.SpellSaveDC(), "8 + level-one proficiency 2 + Wisdom 16 modifier 3")

	var supported []string
	for _, known := range loaded.KnownCantrips() {
		definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Spell(known.ID), SpellSaveDC: loaded.SpellSaveDC()})
		if definition == nil {
			continue
		}
		supported = append(supported, definition.Ref.String())
		s.Require().NoError(definition.Validate())
		s.Require().NotNil(definition.Cast)
		if definition.Cast.Save == nil {
			// Guidance is supported content now too, and carries no save DC —
			// this loop's DC assertion below is Sacred Flame's own check.
			continue
		}
		s.Equal(13, definition.Cast.Save.DC.DC(saves.DCInput{}))
	}
	s.ElementsMatch([]string{refs.Spells.SacredFlame().String(), refs.Spells.Guidance().String()}, supported,
		"Light remains an unsupported choice; Sacred Flame and Guidance are executable content")
	s.Len(loaded.KnownCantrips(), 3, "unsupported choices remain known")
}

func (s *ClericFinalizeSuite) TestInvalidChoicesCannotFinalize() {
	for _, tc := range []struct {
		name   string
		change func(*SetClassInput)
	}{
		{"missing domain", func(in *SetClassInput) { in.SubclassID = "" }},
		{"wrong class domain", func(in *SetClassInput) { in.SubclassID = classes.Subclass("champion") }},
		{"missing cantrip", func(in *SetClassInput) { in.Choices.Cantrips = in.Choices.Cantrips[:2] }},
		{"missing spells", func(in *SetClassInput) { in.Choices.Spells = nil }},
		{"missing spell", func(in *SetClassInput) { in.Choices.Spells = in.Choices.Spells[:2] }},
		{"unsupported spell", func(in *SetClassInput) { in.Choices.Spells[0] = spells.DetectMagic }},
		{"wrong class spell", func(in *SetClassInput) { in.Choices.Spells[0] = spells.Thunderwave }},
		{"duplicate spell", func(in *SetClassInput) { in.Choices.Spells[1] = in.Choices.Spells[0] }},
		{"wrong class cantrip", func(in *SetClassInput) { in.Choices.Cantrips[0] = spells.FireBolt }},
		{"invalid skill", func(in *SetClassInput) { in.Choices.Skills[0] = skills.Athletics }},
		{"duplicate skill", func(in *SetClassInput) { in.Choices.Skills[1] = in.Choices.Skills[0] }},
		{"duplicate cantrip", func(in *SetClassInput) { in.Choices.Cantrips[1] = in.Choices.Cantrips[0] }},
		{"missing equipment", func(in *SetClassInput) { in.Choices.Equipment = in.Choices.Equipment[:4] }},
		{"untrained warhammer", func(in *SetClassInput) { in.Choices.Equipment[0].OptionID = choices.ClericWeaponWarhammer }},
	} {
		s.Run(tc.name, func() {
			input := s.classInput()
			tc.change(input)
			draft := s.draft(input)
			s.False(draft.IsClassComplete())
			_, err := draft.ToCharacter(context.Background(), "invalid-cleric", events.NewEventBus())
			s.Require().Error(err)
		})
	}
}

func (s *ClericFinalizeSuite) TestEquipmentAlternativesAndLegacyLifeChoice() {
	input := s.classInput()
	input.Choices.Equipment[1].OptionID = "cleric-armor-life"
	input.Choices.Equipment[2].OptionID = choices.ClericSecondarySimple
	input.Choices.Equipment[2].CategorySelections = []shared.EquipmentID{weapons.Dagger}
	input.Choices.Equipment[3].OptionID = choices.ClericPackPriest
	draft := s.draft(input)
	s.True(draft.IsClassComplete())
	char, err := draft.ToCharacter(context.Background(), "legacy-cleric", events.NewEventBus())
	s.Require().NoError(err)
	carried := map[string]int{}
	for _, item := range char.ToData().Inventory {
		carried[item.ID] += item.Quantity
	}
	s.Equal(1, carried[string(armor.ChainMail)])
	s.Equal(1, carried[string(armor.Shield)])
	s.Equal(1, carried[string(weapons.Dagger)])
	s.Zero(carried[string(weapons.LightCrossbow)])
	s.Zero(carried["bolts-20"])
}

func (s *ClericFinalizeSuite) TestReplacingClassAndDomainReplacesTheirGrants() {
	draft := s.draft(s.classInput())
	light := s.classInput()
	light.SubclassID = classes.LightDomain
	light.Choices.Cantrips = []spells.Spell{spells.SacredFlame, spells.Guidance, spells.Resistance}
	light.Choices.Equipment[1].OptionID = choices.ClericArmorScale
	s.Require().NoError(draft.SetClass(light))
	char, err := draft.ToCharacter(context.Background(), "light-cleric", events.NewEventBus())
	s.Require().NoError(err)
	s.NotContains(char.ToData().ArmorProficiencies, proficiencies.ArmorHeavy)
	s.Equal(classes.LightDomain, char.ToData().SubclassID)

	fighter := &SetClassInput{ClassID: classes.Fighter, Choices: ClassChoices{
		Skills: []skills.Skill{skills.Athletics, skills.Perception},
	}}
	s.Require().NoError(draft.SetClass(fighter))
	s.Empty(draft.Subclass())
	for _, choice := range draft.ToData().Choices {
		if choice.Source == shared.SourceClass {
			s.Empty(choice.SpellSelection, "cleric cantrips must not survive a class change")
			s.NotContains(string(choice.ChoiceID), "cleric")
		}
	}
	s.Require().NoError(draft.SetClass(s.classInput()))
	char, err = draft.ToCharacter(context.Background(), "cleric-again", events.NewEventBus())
	s.Require().NoError(err)
	s.Len(char.ToData().ArmorProficiencies, 4)
	s.Len(char.ToData().KnownCantrips, 3)
}

func TestClericFinalizeSuite(t *testing.T) { suite.Run(t, new(ClericFinalizeSuite)) }

func (s *ClericFinalizeSuite) TestGuidingBoltCompilesFromNativeClericAfterReload() {
	c, err := s.draft(s.classInput()).ToCharacter(context.Background(), "cleric-bolt", events.NewEventBus())
	s.Require().NoError(err)
	c, err = Load(context.Background(), c.ToData())
	s.Require().NoError(err)
	d := c.CastDefinition(spells.GuidingBolt)
	s.Require().NotNil(d)
	s.Require().NoError(d.Validate())
	s.Equal(5, d.Cast.Attack.AttackBonus, "Wisdom 16 plus proficiency 2")
	s.Nil(d.Cast.Attack.Ability, "spellcasting modifier must not be added to radiant damage")
	s.Equal("4d6", d.Cast.Attack.Damage[0].Dice)
	_, err = c.StatusView(&StatusViewInput{})
	s.NoError(err)
}

func (s *ClericFinalizeSuite) TestInflictWoundsCompilesFromNativeClericAfterReload() {
	c, err := s.draft(s.classInput()).ToCharacter(context.Background(), "cleric-bolt", events.NewEventBus())
	s.Require().NoError(err)
	c, err = Load(context.Background(), c.ToData())
	s.Require().NoError(err)
	d := c.CastDefinition(spells.InflictWounds)
	s.Require().NotNil(d)
	s.Require().NoError(d.Validate())
	s.Equal(5, d.Cast.Attack.AttackBonus, "Wisdom 16 plus proficiency 2")
	s.Nil(d.Cast.Attack.Ability, "spellcasting modifier must not be added to necrotic damage")
	s.Equal("3d10", d.Cast.Attack.Damage[0].Dice)
	_, err = c.StatusView(&StatusViewInput{})
	s.NoError(err)
}

func (s *ClericFinalizeSuite) TestShieldOfFaithCompilesFromNativeClericAfterReload() {
	c, err := s.draft(s.classInput()).ToCharacter(context.Background(), "cleric-faith", events.NewEventBus())
	s.Require().NoError(err)
	c, err = Load(context.Background(), c.ToData())
	s.Require().NoError(err)
	d := c.CastDefinition(spells.ShieldOfFaith)
	s.Require().NotNil(d)
	s.Require().NoError(d.Validate())
	s.Equal(combat.SpellCastingBonusAction, d.Cast.Casting.Time)
	_, err = c.StatusView(&StatusViewInput{})
	s.NoError(err)
}

func (s *ClericFinalizeSuite) TestFourPreparationsPlusLifeDomainGrantsSurviveReload() {
	input := s.classInput()
	input.Choices.Spells = []spells.Spell{spells.GuidingBolt, spells.InflictWounds, spells.ShieldOfFaith, spells.HealingWord}
	c, err := s.draft(input).ToCharacter(context.Background(), "prepared-cleric", events.NewEventBus())
	s.Require().NoError(err)
	loaded, err := Load(context.Background(), c.ToData())
	s.Require().NoError(err)
	s.Len(loaded.KnownSpells(), 6)
	s.ElementsMatch([]string{
		refs.Spells.GuidingBolt().String(), refs.Spells.InflictWounds().String(),
		refs.Spells.ShieldOfFaith().String(), refs.Spells.HealingWord().String(),
		refs.Spells.Bless().String(), refs.Spells.CureWounds().String(),
	}, loaded.ToData().KnownSpells)
	for _, choice := range loaded.ToData().Levels[0].Choices {
		if choice.Category == shared.ChoiceSpells {
			s.ElementsMatch(input.Choices.Spells, choice.SpellSelection)
			s.Len(choice.SpellSelection, 4, "domain spells never become preparation choices")
		}
	}
	_, err = loaded.StatusView(&StatusViewInput{})
	s.NoError(err)
}

func (s *ClericFinalizeSuite) TestPreparationCountAndAutomaticGrantCannotBeSpentAsChoices() {
	for _, prepared := range [][]spells.Spell{
		{spells.Bane, spells.Command, spells.HealingWord},
		{spells.Bane, spells.Command, spells.HealingWord, spells.Sanctuary, spells.GuidingBolt},
		{spells.Bless, spells.Command, spells.HealingWord, spells.Sanctuary},
		{spells.Bane, spells.Bane, spells.HealingWord, spells.Sanctuary},
	} {
		draft := s.draft(s.classInput())
		invalid := s.classInput()
		invalid.Choices.Spells = prepared
		err := draft.SetClass(invalid)
		if err == nil {
			_, err = draft.ToCharacter(context.Background(), "invalid-preparation", events.NewEventBus())
		}
		s.Error(err)
	}
}

func (s *ClericFinalizeSuite) TestLightDomainAddsDeferredLightBeyondThreeCantrips() {
	input := s.classInput()
	input.SubclassID = classes.LightDomain
	input.Choices.Equipment[1].OptionID = choices.ClericArmorScale
	input.Choices.Cantrips = []spells.Spell{spells.Guidance, spells.SacredFlame, spells.Resistance}
	c, err := s.draft(input).ToCharacter(context.Background(), "light-domain", events.NewEventBus())
	s.Require().NoError(err)
	loaded, err := Load(context.Background(), c.ToData())
	s.Require().NoError(err)
	s.Len(loaded.KnownCantrips(), 4)
	s.Contains(loaded.ToData().KnownCantrips, refs.Spells.Light().String())
	s.Nil(loaded.CastDefinition(spells.Light), "Light awaits object targeting, not a no-op cast")
	s.Len(loaded.KnownSpells(), 6)
	s.Contains(loaded.ToData().KnownSpells, refs.Spells.BurningHands().String())
	s.Contains(loaded.ToData().KnownSpells, refs.Spells.FaerieFire().String())
	s.NotContains(loaded.ToData().KnownSpells, refs.Spells.CureWounds().String())
}

func (s *ClericFinalizeSuite) TestWarProficienciesSurviveCreationReloadAndDriveAttacks() {
	input := s.classInput()
	input.SubclassID = classes.WarDomain
	draft := s.draft(input)
	encoded, err := json.Marshal(draft.ToData())
	s.Require().NoError(err)
	var saved DraftData
	s.Require().NoError(json.Unmarshal(encoded, &saved))
	char, err := LoadDraftFromData(&saved).ToCharacter(context.Background(), "war-cleric", events.NewEventBus())
	s.Require().NoError(err)
	encoded, err = json.Marshal(char.ToData())
	s.Require().NoError(err)
	var stored Data
	s.Require().NoError(json.Unmarshal(encoded, &stored))
	loaded, err := LoadFromData(context.Background(), &stored, events.NewEventBus())
	s.Require().NoError(err)
	s.ElementsMatch([]proficiencies.Armor{proficiencies.ArmorLight, proficiencies.ArmorMedium, proficiencies.ArmorShields, proficiencies.ArmorHeavy}, loaded.ToData().ArmorProficiencies)
	s.ElementsMatch([]proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial}, loaded.ToData().WeaponProficiencies)
	for _, id := range []shared.EquipmentID{weapons.Longsword, weapons.Longbow, weapons.Mace} {
		weapon, err := weapons.GetByID(id)
		s.Require().NoError(err)
		s.True(loaded.IsProficientWith(&weapon), string(id))
	}
	// Exercise the attack compiler with a martial melee weapon, then a ranged
	// one; the persisted proficiency must contribute +2 to accuracy only.
	for _, id := range []shared.EquipmentID{weapons.Longsword, weapons.Longbow} {
		attackData := loaded.ToData()
		attackData.Inventory = append(attackData.Inventory, InventoryItemData{Type: shared.EquipmentTypeWeapon, ID: string(id), Quantity: 1})
		attackData.EquipmentSlots = EquipmentSlots{SlotMainHand: string(id)}
		loaded, err = LoadFromData(context.Background(), attackData, events.NewEventBus())
		s.Require().NoError(err)
		trained, err := AssembleAttack(loaded, &AssembleAttackInput{Slot: SlotMainHand})
		s.Require().NoError(err)
		loaded.weaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponSimple}
		untrained, err := AssembleAttack(loaded, &AssembleAttackInput{Slot: SlotMainHand})
		s.Require().NoError(err)
		s.Equal(2, trained.Attack.AttackBonus-untrained.Attack.AttackBonus)
		s.Equal(trained.Attack.Damage, untrained.Attack.Damage)
		loaded.weaponProficiencies = stored.WeaponProficiencies
	}
	// Switching away before finalization must not leak War's proficiencies.
	life := s.classInput()
	s.Require().NoError(draft.SetClass(life))
	char, err = draft.ToCharacter(context.Background(), "life-cleric", events.NewEventBus())
	s.Require().NoError(err)
	s.NotContains(char.ToData().WeaponProficiencies, proficiencies.WeaponMartial)
}
