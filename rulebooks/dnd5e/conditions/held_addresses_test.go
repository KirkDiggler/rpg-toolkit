// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// heldAddressesSuite covers the reader a seam asks "what does this member
// hold".
type heldAddressesSuite struct{ suite.Suite }

func TestHeldAddressesSuite(t *testing.T) { suite.Run(t, new(heldAddressesSuite)) }

func (s *heldAddressesSuite) blob(condition dnd5eEvents.ConditionBehavior) json.RawMessage {
	data, err := condition.ToJSON()
	s.Require().NoError(err)
	return data
}

func (s *heldAddressesSuite) TestReportsEachAddressInStoredOrder() {
	ff, err := NewFaerieFireCondition(NewFaerieFireConditionInput{MemberID: "gob", SourceID: "cleric", SourceRef: refs.Spells.FaerieFire()})
	s.Require().NoError(err)

	held, err := HeldAddresses("gob", []json.RawMessage{s.blob(NewProneCondition("gob")), s.blob(ff)})
	s.Require().NoError(err)

	s.Equal([]dnd5eEvents.ConditionAddress{
		{MemberID: "gob", ConditionRef: refs.Conditions.Prone().String()},
		{MemberID: "gob", ConditionRef: refs.Conditions.FaerieFire().String(), SourceID: "cleric"},
	}, held)
}

func (s *heldAddressesSuite) TestLeavesOutATraitWhichIsNotACondition() {
	trait := json.RawMessage(`{"ref":{"module":"dnd5e","type":"monster_traits","id":"vulnerability"}}`)

	held, err := HeldAddresses("gob", []json.RawMessage{trait, s.blob(NewProneCondition("gob"))})
	s.Require().NoError(err)

	s.Equal([]dnd5eEvents.ConditionAddress{{MemberID: "gob", ConditionRef: refs.Conditions.Prone().String()}}, held)
}

// TestACorruptConditionIsAnErrorNotAGap: a blob that cannot be read, or that
// names a condition and will not load, is refused — a list that left it out
// would claim, known, that the member does not hold it.
func (s *heldAddressesSuite) TestACorruptConditionIsAnErrorNotAGap() {
	for name, blob := range map[string]json.RawMessage{
		"unreadable":             json.RawMessage(`{"ref":"nonsense","x":`),
		"a condition that fails": json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"faerie_fire"},"member_id":""}`),
	} {
		held, err := HeldAddresses("gob", []json.RawMessage{s.blob(NewProneCondition("gob")), blob})
		s.Error(err, name)
		s.Nil(held, name)
	}
}

func (s *heldAddressesSuite) TestAnEmptySheetIsKnownToHoldNothing() {
	held, err := HeldAddresses("gob", nil)
	s.Require().NoError(err)

	s.NotNil(held, "a sheet was read")
	s.Empty(held)
}
