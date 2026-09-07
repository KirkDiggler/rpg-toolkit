package backgrounds_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// GrantsSuite covers backgrounds.GetGrants — the first test file this
// package has ever had (rpg-toolkit#615 flagged the absence). Table-driven
// over all 13 switch cases (covering all 17 background constants — 4
// variants share their base's case), asserting the fixed skill/tool/
// equipment/gold grants this wave adds are exactly right. Choice-bearing
// backgrounds (Entertainer, Folk Hero, Guild Artisan/Merchant, Criminal/
// Spy, Outlander, Noble/Knight, Soldier, Charlatan) only assert their
// *fixed* items here — the choices themselves are #1554 PR 3's scope.
type GrantsSuite struct {
	suite.Suite
}

func TestGrantsSuite(t *testing.T) {
	suite.Run(t, new(GrantsSuite))
}

func (s *GrantsSuite) TestUnknownBackgroundReturnsNil() {
	s.Nil(backgrounds.GetGrants(backgrounds.Background("not-a-real-background")))
}

func (s *GrantsSuite) TestGrants() {
	tests := []struct {
		name         string
		bg           backgrounds.Background
		skills       []skills.Skill
		tools        []proficiencies.Tool
		equipment    []backgrounds.EquipmentItem
		startingGold currency.Money
	}{
		{
			name:   "Acolyte",
			bg:     backgrounds.Acolyte,
			skills: []skills.Skill{skills.Insight, skills.Religion},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.HolySymbol, Quantity: 1},
				{ID: items.Incense, Quantity: 1},
				{ID: items.Vestments, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(15),
		},
		{
			name:   "Criminal",
			bg:     backgrounds.Criminal,
			skills: []skills.Skill{skills.Deception, skills.Stealth},
			tools:  []proficiencies.Tool{proficiencies.ToolThieves},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.Crowbar, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(15),
		},
		{
			name:   "Spy shares Criminal's case",
			bg:     backgrounds.Spy,
			skills: []skills.Skill{skills.Deception, skills.Stealth},
			tools:  []proficiencies.Tool{proficiencies.ToolThieves},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.Crowbar, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(15),
		},
		{
			name:   "Entertainer",
			bg:     backgrounds.Entertainer,
			skills: []skills.Skill{skills.Acrobatics, skills.Performance},
			tools:  []proficiencies.Tool{proficiencies.ToolDisguiseKit},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.Costume, Quantity: 1},
			},
			startingGold: currency.FromGold(15),
		},
		{
			name:   "Folk Hero",
			bg:     backgrounds.FolkHero,
			skills: []skills.Skill{skills.AnimalHandling, skills.Survival},
			tools:  []proficiencies.Tool{proficiencies.ToolVehicleLand},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.Shovel, Quantity: 1},
				{ID: items.IronPot, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(10),
		},
		{
			name:   "Guild Artisan",
			bg:     backgrounds.GuildArtisan,
			skills: []skills.Skill{skills.Insight, skills.Persuasion},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.ClothesTraveler, Quantity: 1},
			},
			startingGold: currency.FromGold(15),
		},
		{
			name:   "Guild Merchant shares Guild Artisan's case",
			bg:     backgrounds.GuildMerchant,
			skills: []skills.Skill{skills.Insight, skills.Persuasion},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.ClothesTraveler, Quantity: 1},
			},
			startingGold: currency.FromGold(15),
		},
		{
			name:   "Hermit",
			bg:     backgrounds.Hermit,
			skills: []skills.Skill{skills.Medicine, skills.Religion},
			tools:  []proficiencies.Tool{proficiencies.ToolHerbalism},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.CaseMap, Quantity: 1},
				{ID: items.Blanket, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
				{ID: tools.HerbalismKit, Quantity: 1},
			},
			startingGold: currency.FromGold(5),
		},
		{
			name:   "Noble",
			bg:     backgrounds.Noble,
			skills: []skills.Skill{skills.History, skills.Persuasion},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.FineClothes, Quantity: 1},
				{ID: items.SignetRing, Quantity: 1},
			},
			startingGold: currency.FromGold(25),
		},
		{
			name:   "Knight shares Noble's case",
			bg:     backgrounds.Knight,
			skills: []skills.Skill{skills.History, skills.Persuasion},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.FineClothes, Quantity: 1},
				{ID: items.SignetRing, Quantity: 1},
			},
			startingGold: currency.FromGold(25),
		},
		{
			name:   "Outlander",
			bg:     backgrounds.Outlander,
			skills: []skills.Skill{skills.Athletics, skills.Survival},
			equipment: []backgrounds.EquipmentItem{
				{ID: weapons.Quarterstaff, Quantity: 1},
				{ID: items.HuntingTrap, Quantity: 1},
				{ID: items.ClothesTraveler, Quantity: 1},
			},
			startingGold: currency.FromGold(10),
		},
		{
			name:   "Sage",
			bg:     backgrounds.Sage,
			skills: []skills.Skill{skills.Arcana, skills.History},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.Ink, Quantity: 1},
				{ID: items.InkPen, Quantity: 1},
				{ID: items.SmallKnife, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(10),
		},
		{
			name:   "Sailor",
			bg:     backgrounds.Sailor,
			skills: []skills.Skill{skills.Athletics, skills.Perception},
			tools:  []proficiencies.Tool{proficiencies.ToolNavigator, proficiencies.ToolVehicleWater},
			equipment: []backgrounds.EquipmentItem{
				{ID: weapons.Club, Quantity: 1},
				{ID: items.SilkRope, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(10),
		},
		{
			name:   "Pirate shares Sailor's case",
			bg:     backgrounds.Pirate,
			skills: []skills.Skill{skills.Athletics, skills.Perception},
			tools:  []proficiencies.Tool{proficiencies.ToolNavigator, proficiencies.ToolVehicleWater},
			equipment: []backgrounds.EquipmentItem{
				{ID: weapons.Club, Quantity: 1},
				{ID: items.SilkRope, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(10),
		},
		{
			name:   "Soldier",
			bg:     backgrounds.Soldier,
			skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			tools:  []proficiencies.Tool{proficiencies.ToolVehicleLand},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(10),
		},
		{
			name:   "Urchin",
			bg:     backgrounds.Urchin,
			skills: []skills.Skill{skills.SleightOfHand, skills.Stealth},
			tools:  []proficiencies.Tool{proficiencies.ToolDisguiseKit, proficiencies.ToolThieves},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.SmallKnife, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			startingGold: currency.FromGold(10),
		},
		{
			name:   "Charlatan",
			bg:     backgrounds.Charlatan,
			skills: []skills.Skill{skills.Deception, skills.SleightOfHand},
			tools:  []proficiencies.Tool{proficiencies.ToolDisguiseKit, proficiencies.ToolForgeryKit},
			equipment: []backgrounds.EquipmentItem{
				{ID: items.FineClothes, Quantity: 1},
				{ID: tools.DisguiseKit, Quantity: 1},
			},
			startingGold: currency.FromGold(15),
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			grant := backgrounds.GetGrants(tc.bg)
			s.Require().NotNil(grant)
			s.ElementsMatch(tc.skills, grant.SkillProficiencies, "skill proficiencies")
			s.ElementsMatch(tc.tools, grant.ToolProficiencies, "tool proficiencies")
			s.ElementsMatch(tc.equipment, grant.Equipment, "equipment")
			s.Equal(tc.startingGold, grant.StartingGold, "starting gold")
		})
	}
}
