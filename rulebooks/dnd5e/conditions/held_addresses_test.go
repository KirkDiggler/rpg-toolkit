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
		oaOf("gob"),
	}, held)
}

func (s *heldAddressesSuite) TestLeavesOutATraitWhichIsNotACondition() {
	trait := json.RawMessage(`{"ref":{"module":"dnd5e","type":"monster_traits","id":"vulnerability"}}`)

	held, err := HeldAddresses("gob", []json.RawMessage{trait, s.blob(NewProneCondition("gob"))})
	s.Require().NoError(err)

	s.Equal([]dnd5eEvents.ConditionAddress{
		{MemberID: "gob", ConditionRef: refs.Conditions.Prone().String()}, oaOf("gob"),
	}, held)
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

// TestAnEmptySheetHoldsOnlyWhatACombatantCarries: a sheet with nothing stored
// still holds the free reactions every combatant carries by existing — the
// attach gives it them before it acts, whether or not they were written back.
func (s *heldAddressesSuite) TestAnEmptySheetHoldsOnlyWhatACombatantCarries() {
	held, err := HeldAddresses("gob", nil)
	s.Require().NoError(err)

	s.Equal([]dnd5eEvents.ConditionAddress{oaOf("gob")}, held)
}

// TestACarriedReactionIsNotListedTwice: once the sheet has been saved with its
// opportunity attack, the stored one is the one listed — the same holdings as
// before it was written back, so a save is never read as a change.
func (s *heldAddressesSuite) TestACarriedReactionIsNotListedTwice() {
	before, err := HeldAddresses("gob", []json.RawMessage{s.blob(NewProneCondition("gob"))})
	s.Require().NoError(err)
	after, err := HeldAddresses("gob", []json.RawMessage{
		s.blob(NewProneCondition("gob")), s.blob(NewOpportunityAttackCondition("gob")),
	})
	s.Require().NoError(err)

	s.Equal(before, after)
}

// oaOf is the opportunity attack a combatant carries, as its address.
func oaOf(member string) dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{MemberID: member, ConditionRef: refs.Conditions.OpportunityAttack().String()}
}
