package equipment_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// ResolveEquipmentDetailSuite tests the ResolveEquipmentDetail function
type ResolveEquipmentDetailSuite struct {
	suite.Suite
}

func TestResolveEquipmentDetailSuite(t *testing.T) {
	suite.Run(t, new(ResolveEquipmentDetailSuite))
}

func (s *ResolveEquipmentDetailSuite) TestWeapon_Longsword() {
	detail := equipment.ResolveEquipmentDetail(weapons.Longsword)

	s.Require().NotNil(detail)
	s.Assert().Equal("Longsword", detail.Name)
	s.Assert().Equal(shared.EquipmentTypeWeapon, detail.Type)
	s.Assert().Equal(float64(3), detail.Weight)
	s.Assert().Equal("15 gp", detail.Cost)

	s.Require().NotNil(detail.Weapon)
	s.Assert().Equal(weapons.CategoryMartialMelee, detail.Weapon.Category)
	s.Assert().Equal("1d8", detail.Weapon.Damage)
	s.Assert().Equal(damage.Slashing, detail.Weapon.DamageType)
	s.Assert().Equal([]weapons.WeaponProperty{weapons.PropertyVersatile}, detail.Weapon.Properties)
	s.Assert().Nil(detail.Weapon.Range)

	s.Assert().Nil(detail.Armor)
}

func (s *ResolveEquipmentDetailSuite) TestWeapon_Longbow_HasRange() {
	detail := equipment.ResolveEquipmentDetail(weapons.Longbow)

	s.Require().NotNil(detail)
	s.Assert().Equal("Longbow", detail.Name)
	s.Assert().Equal(shared.EquipmentTypeWeapon, detail.Type)
	s.Assert().Equal(float64(2), detail.Weight)
	s.Assert().Equal("50 gp", detail.Cost)

	s.Require().NotNil(detail.Weapon)
	s.Assert().Equal(weapons.CategoryMartialRanged, detail.Weapon.Category)
	s.Assert().Equal("1d8", detail.Weapon.Damage)
	s.Assert().Equal(damage.Piercing, detail.Weapon.DamageType)
	s.Assert().Contains(detail.Weapon.Properties, weapons.PropertyAmmunition)
	s.Assert().Contains(detail.Weapon.Properties, weapons.PropertyHeavy)
	s.Assert().Contains(detail.Weapon.Properties, weapons.PropertyTwoHanded)

	s.Require().NotNil(detail.Weapon.Range)
	s.Assert().Equal(150, detail.Weapon.Range.Normal)
	s.Assert().Equal(600, detail.Weapon.Range.Long)
}

func (s *ResolveEquipmentDetailSuite) TestArmor_ChainMail() {
	detail := equipment.ResolveEquipmentDetail(armor.ChainMail)

	s.Require().NotNil(detail)
	s.Assert().Equal("Chain Mail", detail.Name)
	s.Assert().Equal(shared.EquipmentTypeArmor, detail.Type)
	s.Assert().Equal(float64(55), detail.Weight)
	s.Assert().Equal("75 gp", detail.Cost)

	s.Require().NotNil(detail.Armor)
	s.Assert().Equal(armor.CategoryHeavy, detail.Armor.Category)
	s.Assert().Equal(16, detail.Armor.BaseAC)
	s.Assert().False(detail.Armor.DexBonus)
	s.Require().NotNil(detail.Armor.MaxDexBonus)
	s.Assert().Equal(0, *detail.Armor.MaxDexBonus)
	s.Assert().Equal(13, detail.Armor.StrengthRequirement)
	s.Assert().True(detail.Armor.StealthDisadvantage)

	s.Assert().Nil(detail.Weapon)
}

func (s *ResolveEquipmentDetailSuite) TestArmor_Shield() {
	detail := equipment.ResolveEquipmentDetail(armor.Shield)

	s.Require().NotNil(detail)
	s.Assert().Equal("Shield", detail.Name)
	s.Assert().Equal(shared.EquipmentTypeArmor, detail.Type)
	s.Assert().Equal(float64(6), detail.Weight)
	s.Assert().Equal("10 gp", detail.Cost)

	s.Require().NotNil(detail.Armor)
	s.Assert().Equal(armor.CategoryShield, detail.Armor.Category)
	s.Assert().Equal(2, detail.Armor.BaseAC)
	s.Assert().True(detail.Armor.DexBonus)
	s.Assert().Nil(detail.Armor.MaxDexBonus)
	s.Assert().Equal(0, detail.Armor.StrengthRequirement)
	s.Assert().False(detail.Armor.StealthDisadvantage)
}

func (s *ResolveEquipmentDetailSuite) TestTool_ThievesTools() {
	detail := equipment.ResolveEquipmentDetail(tools.ThievesTools)

	s.Require().NotNil(detail)
	s.Assert().Equal("Thieves' Tools", detail.Name)
	s.Assert().Equal(shared.EquipmentTypeTool, detail.Type)
	s.Assert().Equal(float64(1), detail.Weight)
	s.Assert().Equal("25 gp", detail.Cost)

	s.Assert().Nil(detail.Weapon)
	s.Assert().Nil(detail.Armor)
}

func (s *ResolveEquipmentDetailSuite) TestPack_ExplorerPack() {
	detail := equipment.ResolveEquipmentDetail(packs.ExplorerPack)

	s.Require().NotNil(detail)
	s.Assert().Equal("Explorer's Pack", detail.Name)
	s.Assert().Equal(shared.EquipmentTypePack, detail.Type)
	s.Assert().Equal(float64(59), detail.Weight)
	s.Assert().Equal("10 gp", detail.Cost)

	s.Assert().Nil(detail.Weapon)
	s.Assert().Nil(detail.Armor)
}

func (s *ResolveEquipmentDetailSuite) TestAmmunition_Arrows() {
	detail := equipment.ResolveEquipmentDetail(ammunition.Arrows20)

	s.Require().NotNil(detail)
	s.Assert().Equal("Arrows (20)", detail.Name)
	s.Assert().Equal(shared.EquipmentTypeAmmunition, detail.Type)
	s.Assert().Equal(float64(1), detail.Weight)
	s.Assert().Equal("1 gp", detail.Cost)

	s.Assert().Nil(detail.Weapon)
	s.Assert().Nil(detail.Armor)
}

func (s *ResolveEquipmentDetailSuite) TestUnknownID_ReturnsNil() {
	detail := equipment.ResolveEquipmentDetail("nonexistent-item-xyz")

	s.Assert().Nil(detail)
}
