// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// kiFlurry is the profile v1 never compiles and the type must already hold: a
// pool cost and a precondition, side by side with the currencies that do get
// compiled. It is a package-level declaration on purpose — if either field
// stops existing, or stops being keyed, this file stops building, which is the
// pin. Nothing exercises it as content; the shape is what ships.
var kiFlurry = &combat.SpendProfile{
	Slots:    map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
	Pools:    map[coreResources.ResourceKey]int{"ki": 1},
	Requires: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	Grants:   map[combat.CapacityType]int{combat.CapacityFlurryStrike: 2},
}

// SpendProfileTestSuite pins the profile's shape rather than any content: that
// every currency is a map keyed by an existing vocabulary, and that a profile
// nobody can charge is refused before it can be half-charged.
type SpendProfileTestSuite struct {
	suite.Suite
}

func TestSpendProfileSuite(t *testing.T) {
	suite.Run(t, new(SpendProfileTestSuite))
}

// The ki-shaped profile validates. Expressible means expressible — a shape the
// type can hold but the gate would refuse is not a shape that ships.
func (s *SpendProfileTestSuite) TestPoolCostAndPreconditionAreLegal() {
	s.Require().NoError(kiFlurry.Validate())
}

// A nil profile is a free action, and free actions are legal.
func (s *SpendProfileTestSuite) TestNilProfileValidates() {
	var free *combat.SpendProfile
	s.Require().NoError(free.Validate())
}

// The empty profile is the same thing spelled differently.
func (s *SpendProfileTestSuite) TestEmptyProfileValidates() {
	s.Require().NoError((&combat.SpendProfile{}).Validate())
}

// The per-turn slots are exactly the three the sheet keeps. Movement is not a
// slot — it is keyed capacity — and free is the absence of a slot cost rather
// than a slot that costs nothing.
func (s *SpendProfileTestSuite) TestOnlyTheThreeTurnSlotsAreSlots() {
	legal := []coreCombat.ActionType{
		coreCombat.ActionStandard,
		coreCombat.ActionBonus,
		coreCombat.ActionReaction,
	}
	for _, slot := range legal {
		profile := &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{slot: 1}}
		s.Require().NoErrorf(profile.Validate(), "slot %q", slot)
	}

	illegal := []coreCombat.ActionType{
		coreCombat.ActionFree,
		coreCombat.ActionMovement,
		coreCombat.ActionType("nonsense"),
		coreCombat.ActionType(""),
	}
	for _, slot := range illegal {
		profile := &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{slot: 1}}
		s.Require().Errorf(profile.Validate(), "slot %q", slot)
	}
}

// Every declared capacity is a legal key for a cost, a grant and a
// requirement. A vocabulary member the profile cannot name is a spend that has
// nowhere to be written down.
func (s *SpendProfileTestSuite) TestEveryDeclaredCapacityIsALegalKey() {
	for _, key := range combat.CapacityTypes() {
		s.Require().NoErrorf(
			(&combat.SpendProfile{Capacity: map[combat.CapacityType]int{key: 1}}).Validate(),
			"cost keyed %q", key,
		)
		s.Require().NoErrorf(
			(&combat.SpendProfile{Grants: map[combat.CapacityType]int{key: 1}}).Validate(),
			"grant keyed %q", key,
		)
		s.Require().NoErrorf(
			(&combat.SpendProfile{Requires: map[combat.CapacityType]int{key: 1}}).Validate(),
			"requirement keyed %q", key,
		)
	}
}

// CapacityNone is the answer an action gives when it consumes no capacity. It
// is not a currency, so it cannot be a key.
func (s *SpendProfileTestSuite) TestCapacityNoneIsNotACurrency() {
	s.NotContains(combat.CapacityTypes(), combat.CapacityNone)
}
