// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// errUnserializable is what the failing stubs below answer from ToJSON.
var errUnserializable = errors.New("stub refuses to serialize")

// unserializableCondition is a condition that exists on a sheet and cannot be
// written down.
type unserializableCondition struct{ keptCondition }

func (*unserializableCondition) ToJSON() (json.RawMessage, error) { return nil, errUnserializable }

// unserializableFeature is the feature-side twin.
type unserializableFeature struct{ stubFeature }

func (*unserializableFeature) ToJSON() (json.RawMessage, error) { return nil, errUnserializable }

// ToDataTestSuite pins rpg-toolkit#1965 tier 1 #5: ToData refuses a sheet it
// cannot write whole.
//
// It used to `continue` past any feature or condition whose ToJSON failed and
// return the rest, so the record a host saved had silently lost an effect.
// The save-side twin of a loader that drops what it cannot read (#948).
type ToDataTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestToDataTestSuite(t *testing.T) { suite.Run(t, new(ToDataTestSuite)) }

func (s *ToDataTestSuite) SetupTest() { s.ctx = context.Background() }

func (s *ToDataTestSuite) loaded() *Character {
	loaded, err := Load(s.ctx, &Data{
		ID:               "char-1",
		PlayerID:         "player-1",
		Name:             "Serialized",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:      12,
		MaxHitPoints:   12,
		EquipmentSlots: EquipmentSlots{},
	})
	s.Require().NoError(err)

	return loaded
}

func (s *ToDataTestSuite) TestAConditionThatCannotSerializeFailsTheWrite() {
	char := s.loaded()
	char.conditions = append(char.conditions, &keptCondition{}, &unserializableCondition{})

	data, err := char.ToData()

	s.Require().ErrorIs(err, errUnserializable)
	s.ErrorContains(err, `serialize condition "`+(&keptCondition{}).Ref().String()+`"`,
		"the refusal names the effect it could not write, not its position")
	s.Nil(data, "nothing is handed back to be written: a record missing a condition is not a smaller truth")
}

func (s *ToDataTestSuite) TestAFeatureThatCannotSerializeFailsTheWrite() {
	char := s.loaded()
	char.features = append(char.features, &stubFeature{}, &unserializableFeature{})

	data, err := char.ToData()

	s.Require().ErrorIs(err, errUnserializable)
	s.ErrorContains(err, `serialize feature "`+stubFeatureRef.String()+`"`,
		"the refusal names the effect it could not write, not its position")
	s.Nil(data)
}

// The other half: the same sheet with only serializable effects writes, and
// the effect is in the record. Without this, a ToData that always refused
// would pass above.
func (s *ToDataTestSuite) TestASerializableSheetWritesItsEffects() {
	char := s.loaded()
	char.conditions = append(char.conditions, &keptCondition{})
	char.features = append(char.features, &stubFeature{})

	data, err := char.ToData()

	s.Require().NoError(err)
	s.Require().NotNil(data)
	kept, err := (&keptCondition{}).ToJSON()
	s.Require().NoError(err)
	s.Contains(data.Conditions, kept)
	stub, err := (&stubFeature{}).ToJSON()
	s.Require().NoError(err)
	s.Contains(data.Features, stub)
}
