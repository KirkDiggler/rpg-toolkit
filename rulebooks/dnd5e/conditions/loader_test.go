// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// LoaderTestSuite tests the condition loader functionality
type LoaderTestSuite struct {
	suite.Suite
}

func TestLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}

func (s *LoaderTestSuite) TestLoadRagingCondition() {
	// Create a raging condition
	original := &RagingCondition{
		CharacterID:       "barbarian-1",
		DamageBonus:       2,
		Level:             5,
		Source:            "dnd5e:features:rage",
		TurnsActive:       3,
		WasHitThisTurn:    true,
		DidAttackThisTurn: true,
	}

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Load from JSON
	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	// Verify it's a RagingCondition
	raging, ok := loaded.(*RagingCondition)
	s.Require().True(ok, "Expected *RagingCondition")

	// Verify all fields match
	s.Equal(original.CharacterID, raging.CharacterID)
	s.Equal(original.DamageBonus, raging.DamageBonus)
	s.Equal(original.Level, raging.Level)
	s.Equal(original.Source, raging.Source)
	s.Equal(original.TurnsActive, raging.TurnsActive)
	s.Equal(original.WasHitThisTurn, raging.WasHitThisTurn)
	s.Equal(original.DidAttackThisTurn, raging.DidAttackThisTurn)
}

func (s *LoaderTestSuite) TestLoadBrutalCriticalCondition() {
	// Create a brutal critical condition
	original := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       13,
	})

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Load from JSON
	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	// Verify it's a BrutalCriticalCondition
	brutal, ok := loaded.(*BrutalCriticalCondition)
	s.Require().True(ok, "Expected *BrutalCriticalCondition")

	// Verify all fields match
	s.Equal(original.CharacterID, brutal.CharacterID)
	s.Equal(original.Level, brutal.Level)
	s.Equal(original.ExtraDice, brutal.ExtraDice)
}

func (s *LoaderTestSuite) TestLoadUnarmoredDefenseCondition() {
	// Create an unarmored defense condition
	original := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barbarian-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Load from JSON
	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	// Verify it's an UnarmoredDefenseCondition
	unarmored, ok := loaded.(*UnarmoredDefenseCondition)
	s.Require().True(ok, "Expected *UnarmoredDefenseCondition")

	// Verify all fields match
	s.Equal(original.CharacterID, unarmored.CharacterID)
	s.Equal(original.Type, unarmored.Type)
	s.Equal(original.Source, unarmored.Source)
}

func (s *LoaderTestSuite) TestLoadMonkUnarmoredDefense() {
	// Verify monk variant loads correctly
	original := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "monk-1",
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})

	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	unarmored, ok := loaded.(*UnarmoredDefenseCondition)
	s.Require().True(ok)
	s.Equal(UnarmoredDefenseMonk, unarmored.Type)
}

func (s *LoaderTestSuite) TestLoadUnknownCondition() {
	// Test loading unknown condition ref
	jsonData := []byte(`{"ref":{"module":"dnd5e","type":"conditions","value":"unknown"}}`)

	_, err := LoadJSON(jsonData)
	s.Error(err)
	s.Contains(err.Error(), "unknown condition ref")
}

func (s *LoaderTestSuite) TestLoadInvalidJSON() {
	// Test loading invalid JSON
	jsonData := []byte(`{invalid json}`)

	_, err := LoadJSON(jsonData)
	s.Error(err)
	s.Contains(err.Error(), "failed to peek at condition ref")
}
