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

	held := HeldAddresses("gob", []json.RawMessage{s.blob(NewProneCondition("gob")), s.blob(ff)})

	s.Equal([]dnd5eEvents.ConditionAddress{
		{MemberID: "gob", ConditionRef: refs.Conditions.Prone().String()},
		{MemberID: "gob", ConditionRef: refs.Conditions.FaerieFire().String(), SourceID: "cleric"},
	}, held)
}

func (s *heldAddressesSuite) TestLeavesOutWhatTheSheetDoesNotLoad() {
	trait := json.RawMessage(`{"ref":{"module":"dnd5e","type":"monster_traits","id":"vulnerability"}}`)
	corrupt := json.RawMessage(`{"ref":"nonsense","x":`)

	held := HeldAddresses("gob", []json.RawMessage{trait, s.blob(NewProneCondition("gob")), corrupt})

	s.Equal([]dnd5eEvents.ConditionAddress{{MemberID: "gob", ConditionRef: refs.Conditions.Prone().String()}}, held)
}

func (s *heldAddressesSuite) TestAnEmptySheetIsKnownToHoldNothing() {
	held := HeldAddresses("gob", nil)

	s.NotNil(held, "a sheet was read")
	s.Empty(held)
}
