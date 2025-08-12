// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

type LoaderTestSuite struct {
	suite.Suite
}

func (s *LoaderTestSuite) TestLoadConditionData() {
	data := json.RawMessage(`{
		"ref": "test:condition:poisoned",
		"name": "Poisoned",
		"description": "Disadvantage on attacks",
		"metadata": {"save_dc": 13}
	}`)

	condData, err := conditions.Load(data)
	s.Require().NoError(err)
	s.Assert().NotNil(condData)

	// Check ref
	ref := condData.Ref()
	s.Assert().Equal("test", ref.Module)
	s.Assert().Equal("condition", ref.Type)
	s.Assert().Equal("poisoned", ref.Value)

	// Check we have the full JSON
	s.Assert().Equal(data, condData.JSON())
}

func (s *LoaderTestSuite) TestLoadMultipleConditions() {
	data := json.RawMessage(`[
		{
			"ref": "test:condition:poisoned",
			"name": "Poisoned"
		},
		{
			"ref": "test:condition:stunned",
			"name": "Stunned"
		},
		{
			"ref": "test:condition:blinded",
			"name": "Blinded"
		}
	]`)

	conditions, err := conditions.LoadAll(data)
	s.Require().NoError(err)
	s.Assert().Len(conditions, 3)

	// Verify each condition
	s.Assert().Equal("poisoned", conditions[0].Ref().Value)
	s.Assert().Equal("stunned", conditions[1].Ref().Value)
	s.Assert().Equal("blinded", conditions[2].Ref().Value)
}

func (s *LoaderTestSuite) TestLoadInvalidJSON() {
	data := json.RawMessage(`not valid json`)

	_, err := conditions.Load(data)
	s.Assert().Error(err)

	var loadErr *conditions.LoadError
	s.Assert().ErrorAs(err, &loadErr)
}

func (s *LoaderTestSuite) TestLoadMissingRef() {
	data := json.RawMessage(`{
		"name": "Poisoned",
		"description": "No ref provided"
	}`)

	_, err := conditions.Load(data)
	s.Assert().Error(err)
	s.Assert().ErrorIs(err, conditions.ErrInvalidRef)
}

func (s *LoaderTestSuite) TestRoutingExample() {
	// This test demonstrates how to use the loader for routing
	data := json.RawMessage(`{
		"ref": "dnd5e:condition:exhaustion",
		"name": "Exhaustion",
		"level": 3,
		"metadata": {"max_level": 6}
	}`)

	condData, err := conditions.Load(data)
	s.Require().NoError(err)

	// Route based on ref
	switch condData.Ref().Module {
	case "dnd5e":
		switch condData.Ref().Value {
		case "exhaustion":
			// Would create DnD5eExhaustionCondition here
			s.Assert().Equal("exhaustion", condData.Ref().Value)
		default:
			s.Fail("Unknown D&D 5e condition")
		}
	default:
		s.Fail("Unknown game system")
	}
}

func (s *LoaderTestSuite) TestRefStringParsing() {
	// This test shows that the loader delegates ref parsing to core.ParseString
	// Currently core enforces 3 segments, but when core evolves to support
	// more complex refs, this loader won't need any changes!
	
	// Valid refs (current 3-segment format)
	validRefs := []string{
		"dnd5e:condition:poisoned",
		"homebrew:condition:affliction",
		"pathfinder:condition:stunned",
	}

	for _, ref := range validRefs {
		s.Run(ref, func() {
			data := json.RawMessage(fmt.Sprintf(`{"ref": "%s", "name": "Test"}`, ref))
			
			condData, err := conditions.Load(data)
			s.Require().NoError(err, "Failed to load ref: %s", ref)
			s.Assert().NotNil(condData.Ref())
		})
	}

	// Future evolution examples (will work when core.ParseString evolves)
	// These currently fail, showing that evolution is controlled by core
	futureRefs := []string{
		"dnd5e:condition:poisoned:v2",
		"homebrew:condition:affliction:experimental",
	}

	for _, ref := range futureRefs {
		s.Run("future_"+ref, func() {
			data := json.RawMessage(fmt.Sprintf(`{"ref": "%s", "name": "Test"}`, ref))
			
			_, err := conditions.Load(data)
			// These should fail for now - core enforces 3 segments
			s.Assert().Error(err)
			s.Assert().Contains(err.Error(), "too many segments")
		})
	}
}

func TestLoaderSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}
