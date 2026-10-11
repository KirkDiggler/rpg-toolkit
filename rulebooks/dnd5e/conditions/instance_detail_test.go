// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// These tests use literal words: the spells package imports conditions, so its
// word constants would be an import cycle.
type InstanceDetailSuite struct {
	suite.Suite
}

func TestInstanceDetailSuite(t *testing.T) {
	suite.Run(t, new(InstanceDetailSuite))
}

func (s *InstanceDetailSuite) TestCommandedTellsItsWordAsItsDetail() {
	for _, word := range []string{"approach", "flee", "grovel"} {
		s.Run(word, func() {
			c, err := NewCommandedCondition("goblin", "", "bard", word, 1)
			s.Require().NoError(err)
			s.Equal(word, InstanceDetailOf(c))
		})
	}
}

func (s *InstanceDetailSuite) TestALoadedCommandedTellsTheSameWord() {
	c, err := NewCommandedCondition("goblin", "", "bard", "grovel", 1)
	s.Require().NoError(err)
	raw, err := c.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(json.RawMessage(raw))
	s.Require().NoError(err)
	s.Equal("grovel", InstanceDetailOf(loaded))
}

func (s *InstanceDetailSuite) TestAConditionWithNothingToSayHasNoDetail() {
	ref, err := core.ParseString("dnd5e:spells:bless")
	s.Require().NoError(err)
	blessed, err := NewBlessedCondition(NewBlessedConditionInput{
		MemberID: "goblin", SourceID: "cleric", SourceRef: ref,
	})
	s.Require().NoError(err)
	s.Equal("", InstanceDetailOf(blessed))
}
