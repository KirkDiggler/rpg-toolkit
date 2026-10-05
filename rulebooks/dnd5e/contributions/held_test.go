// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

// heldSuite covers what a frame says members hold and see.
type heldSuite struct{ suite.Suite }

func TestHeldSuite(t *testing.T) { suite.Run(t, new(heldSuite)) }

const faerieFire = "dnd5e:conditions:faerie_fire"

func heldFrame() contributions.Frame {
	frame := validFrame()
	frame.Held = []contributions.MemberHeld{
		{Member: "goblin", Conditions: []contributions.HeldCondition{{Ref: faerieFire, SourceID: "cleric"}}},
		{Member: "fighter", Conditions: []contributions.HeldCondition{}},
	}
	return frame
}

func (s *heldSuite) TestHeldByAbsentMemberIsUnknown() {
	frame := heldFrame()
	s.Require().NoError(frame.Validate())

	held, known := frame.HeldBy("x")
	s.False(known, "a member the frame does not list is unknown")
	s.Nil(held)

	held, known = frame.HeldBy("fighter")
	s.True(known, "a listed member with no conditions is known to hold nothing")
	s.Empty(held)

	held, known = frame.HeldBy("goblin")
	s.True(known)
	s.Equal([]contributions.HeldCondition{{Ref: faerieFire, SourceID: "cleric"}}, held)
}

func (s *heldSuite) TestFrameValidateRejectsMalformedHeld() {
	for name, held := range map[string][]contributions.MemberHeld{
		"empty member":    {{Member: "", Conditions: nil}},
		"repeated member": {{Member: "goblin"}, {Member: "goblin"}},
		"empty ref":       {{Member: "goblin", Conditions: []contributions.HeldCondition{{Ref: ""}}}},
		"not a ref":       {{Member: "goblin", Conditions: []contributions.HeldCondition{{Ref: "not a ref"}}}},
		"repeated condition": {{Member: "goblin", Conditions: []contributions.HeldCondition{
			{Ref: faerieFire, SourceID: "cleric"}, {Ref: faerieFire, SourceID: "cleric"},
		}}},
	} {
		frame := validFrame()
		frame.Held = held
		s.Error(frame.Validate(), name)
	}

	twoSources := validFrame()
	twoSources.Held = []contributions.MemberHeld{{Member: "goblin", Conditions: []contributions.HeldCondition{
		{Ref: faerieFire, SourceID: "cleric"}, {Ref: faerieFire, SourceID: "druid"},
	}}}
	s.NoError(twoSources.Validate(), "one condition from two sources is two holdings")
}

func (s *heldSuite) TestFrameCloneDetachesHeld() {
	frame := heldFrame()
	clone := frame.Clone()

	clone.Held[0].Conditions[0].SourceID = "changed"
	clone.Held[1].Member = "changed"

	s.Equal("cleric", frame.Held[0].Conditions[0].SourceID)
	s.Equal("fighter", frame.Held[1].Member)
}

func (s *heldSuite) TestPairSeesKnownFalseIsNotUnknown() {
	frame := validFrame()
	frame.Pairs[0].Sees = contributions.Known(false)

	sees, known := frame.Pair("goblin", "fighter").Sees.Get()
	s.True(known)
	s.False(sees)

	_, known = frame.Pair("fighter", "goblin").Sees.Get()
	s.False(known, "no reverse fact is inferred")
}
