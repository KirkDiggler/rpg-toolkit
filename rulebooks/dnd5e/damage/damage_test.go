package damage_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

type DamageTestSuite struct {
	suite.Suite
}

func TestDamageTestSuite(t *testing.T) {
	suite.Run(t, new(DamageTestSuite))
}

func (s *DamageTestSuite) TestHasProperty() {
	d := damage.Damage{Properties: []damage.Property{damage.DoesNotCrit}}
	s.True(d.HasProperty(damage.DoesNotCrit))
	s.False(d.HasProperty(damage.AddsAttackAbilityModifier))
}

func (s *DamageTestSuite) TestValidationAcceptsPureDicePools() {
	s.NoError(damage.Validate([]damage.Damage{
		{Dice: "2d6", Type: damage.Slashing, FlatBonus: -1},
		{Dice: "d4", Type: damage.Fire, Properties: []damage.Property{damage.DoesNotCrit}},
	}))
}

func (s *DamageTestSuite) TestValidationRejectsEmptyPools() {
	err := damage.Validate(nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "damage pool")
}

func (s *DamageTestSuite) TestValidationRejectsInvalidPoolNotation() {
	for _, notation := range []string{"", "garbage", "1d8+2", "1d8-2", "1d8+1d6"} {
		s.Run(notation, func() {
			err := damage.Validate([]damage.Damage{{Dice: notation, Type: damage.Slashing}})
			s.Require().Error(err)
			s.Contains(err.Error(), "pool 0")
			s.Contains(err.Error(), notation)
		})
	}
}

func (s *DamageTestSuite) TestValidationRejectsUnknownTypesAndProperties() {
	tests := []struct {
		name string
		pool damage.Damage
	}{
		{name: "none type", pool: damage.Damage{Dice: "1d8", Type: damage.None}},
		{name: "unknown type", pool: damage.Damage{Dice: "1d8", Type: damage.Type("forceful")}},
		{name: "unknown property", pool: damage.Damage{Dice: "1d8", Type: damage.Fire, Properties: []damage.Property{"mystery"}}},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := damage.Validate([]damage.Damage{tc.pool})
			s.Require().Error(err)
			s.Contains(err.Error(), "pool 0")
			s.Contains(err.Error(), tc.pool.Dice)
		})
	}
}

func (s *DamageTestSuite) TestValidationRejectsFusedModifierAndDuplicateAbilityMarkers() {
	s.Error(damage.Validate([]damage.Damage{{Dice: "1d8+2", Type: damage.Slashing}}))
	s.Error(damage.Validate([]damage.Damage{
		{Dice: "1d8", Type: damage.Slashing, Properties: []damage.Property{damage.AddsAttackAbilityModifier}},
		{Dice: "1d6", Type: damage.Fire, Properties: []damage.Property{damage.AddsAttackAbilityModifier}},
	}))
}

func (s *DamageTestSuite) TestValidationReportsLaterPoolContext() {
	err := damage.Validate([]damage.Damage{
		{Dice: "1d8", Type: damage.Slashing},
		{Dice: "2d6+1", Type: damage.Fire},
	})
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "pool 1"))
	s.Contains(err.Error(), "2d6+1")
}
