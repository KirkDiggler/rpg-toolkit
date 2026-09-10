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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

func (s *ClericFinalizeSuite) TestCreationAndPersistence() {
	draft := s.draft(s.classInput())
	s.True(draft.IsClassComplete(), "the selected domain must reach validation")
	encoded, err := json.Marshal(draft.ToData())
	s.Require().NoError(err)
	var saved DraftData
	s.Require().NoError(json.Unmarshal(encoded, &saved))
	char, err := LoadDraftFromData(&saved).ToCharacter(context.Background(), "cleric-1", events.NewEventBus())
	s.Require().NoError(err)
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
	s.Equal(data.Resources, back.Resources)
	s.Equal(4, back.HitPoints, "loading is not a rest")
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
		s.Require().NotNil(definition.Cast.Save)
		s.Equal(13, definition.Cast.Save.DC.DC(saves.DCInput{}))
	}
	s.Equal([]string{refs.Spells.SacredFlame().String()}, supported)
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
