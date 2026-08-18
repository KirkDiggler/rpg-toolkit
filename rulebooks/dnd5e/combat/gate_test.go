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

// spyLedger is a ledger that counts writes, so "a refused payment changed
// nothing" can be asserted as NOTHING WAS WRITTEN rather than inferred from a
// state comparison that a compensating pair of writes would pass.
type spyLedger struct {
	inCombat bool
	slots    map[coreCombat.ActionType]int
	capacity map[combat.CapacityType]int
	pools    map[coreResources.ResourceKey]int

	writes int
}

func newSpyLedger() *spyLedger {
	return &spyLedger{
		inCombat: true,
		slots:    map[coreCombat.ActionType]int{},
		capacity: map[combat.CapacityType]int{},
		pools:    map[coreResources.ResourceKey]int{},
	}
}

func (l *spyLedger) InCombat() bool { return l.inCombat }

func (l *spyLedger) SlotsLeft(slot coreCombat.ActionType) int { return l.slots[slot] }

func (l *spyLedger) CapacityLeft(key combat.CapacityType) int { return l.capacity[key] }

func (l *spyLedger) PoolLeft(key coreResources.ResourceKey) int { return l.pools[key] }

func (l *spyLedger) SpendSlots(slot coreCombat.ActionType, n int) {
	l.writes++
	l.slots[slot] -= n
}

func (l *spyLedger) SpendCapacity(key combat.CapacityType, n int) {
	l.writes++
	l.capacity[key] -= n
}

func (l *spyLedger) SpendPool(key coreResources.ResourceKey, n int) {
	l.writes++
	l.pools[key] -= n
}

func (l *spyLedger) BankCapacity(key combat.CapacityType, n int) {
	l.writes++
	l.capacity[key] += n
}

// GateTestSuite pins the gate's two pure functions: what they read, what they
// write, and — the property the whole slice turns on — that a refused payment
// is indistinguishable from one that was never attempted.
type GateTestSuite struct {
	suite.Suite
}

func TestGateSuite(t *testing.T) {
	suite.Run(t, new(GateTestSuite))
}

// mixed is a profile that costs three different currencies at once. Nothing in
// v1 compiles one — it is what the ki-spending monk's bonus strike will be —
// and it exists here because atomicity is only observable across currencies.
func (s *GateTestSuite) mixed() *combat.SpendProfile {
	return &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Capacity: map[combat.CapacityType]int{combat.CapacityFlurryStrike: 1},
		Pools:    map[coreResources.ResourceKey]int{"ki": 2},
	}
}

// A payment that cannot be met in one currency spends nothing in the others.
// The pool is short by one; the slot and the capacity are both affordable and
// both must survive untouched.
func (s *GateTestSuite) TestPayIsAtomicAcrossCurrencies() {
	ledger := newSpyLedger()
	ledger.slots[coreCombat.ActionBonus] = 1
	ledger.capacity[combat.CapacityFlurryStrike] = 1
	ledger.pools["ki"] = 1

	s.False(combat.CanPay(ledger, s.mixed()))

	err := combat.Pay(ledger, s.mixed())
	s.Require().Error(err)

	s.Zero(ledger.writes, "a refused payment must not write")
	s.Equal(1, ledger.slots[coreCombat.ActionBonus])
	s.Equal(1, ledger.capacity[combat.CapacityFlurryStrike])
	s.Equal(1, ledger.pools["ki"])
}

// The affordable case debits every currency exactly once.
func (s *GateTestSuite) TestPayDebitsEveryCurrency() {
	ledger := newSpyLedger()
	ledger.slots[coreCombat.ActionBonus] = 1
	ledger.capacity[combat.CapacityFlurryStrike] = 3
	ledger.pools["ki"] = 3

	s.True(combat.CanPay(ledger, s.mixed()))
	s.Require().NoError(combat.Pay(ledger, s.mixed()))

	s.Equal(0, ledger.slots[coreCombat.ActionBonus])
	s.Equal(2, ledger.capacity[combat.CapacityFlurryStrike])
	s.Equal(1, ledger.pools["ki"])
}

// CanPay is a read. Asking whether an action is affordable must never be the
// reason it stops being affordable.
func (s *GateTestSuite) TestCanPayWritesNothing() {
	ledger := newSpyLedger()
	ledger.slots[coreCombat.ActionBonus] = 1
	ledger.capacity[combat.CapacityFlurryStrike] = 1
	ledger.pools["ki"] = 2

	s.True(combat.CanPay(ledger, s.mixed()))
	s.True(combat.CanPay(ledger, s.mixed()))

	s.Zero(ledger.writes)
	s.Equal(1, ledger.slots[coreCombat.ActionBonus])
	s.Equal(1, ledger.capacity[combat.CapacityFlurryStrike])
	s.Equal(2, ledger.pools["ki"])
}

// Banked capacity is added to what is already there, not assigned over it: a
// second Attack action in one turn (Action Surge) banks more attacks rather
// than resetting the bank to the same number.
func (s *GateTestSuite) TestGrantsBankOnTopOfWhatIsThere() {
	ledger := newSpyLedger()
	ledger.slots[coreCombat.ActionStandard] = 2
	ledger.capacity[combat.CapacityAttack] = 1

	profile := &combat.SpendProfile{
		Slots:  map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Grants: map[combat.CapacityType]int{combat.CapacityAttack: 2},
	}

	s.Require().NoError(combat.Pay(ledger, profile))
	s.Equal(3, ledger.capacity[combat.CapacityAttack])
	s.Equal(1, ledger.slots[coreCombat.ActionStandard])
}

// A requirement is read and never debited. This is the monk's shape — the
// bonus strike needs the Attack action to have happened, and needs it to still
// be there afterwards for the second strike.
func (s *GateTestSuite) TestRequirementsAreCheckedAndNotSpent() {
	ledger := newSpyLedger()
	ledger.slots[coreCombat.ActionBonus] = 1
	ledger.capacity[combat.CapacityAttack] = 1

	profile := &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Requires: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}

	s.True(combat.CanPay(ledger, profile))
	s.Require().NoError(combat.Pay(ledger, profile))

	s.Equal(0, ledger.slots[coreCombat.ActionBonus])
	s.Equal(1, ledger.capacity[combat.CapacityAttack], "a requirement is not a cost")
}

// An unmet requirement refuses the whole payment, and refuses it the same way
// an unaffordable cost does: nothing written.
func (s *GateTestSuite) TestUnmetRequirementRefusesAndWritesNothing() {
	ledger := newSpyLedger()
	ledger.slots[coreCombat.ActionBonus] = 1

	profile := &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Requires: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}

	s.False(combat.CanPay(ledger, profile))
	s.Require().Error(combat.Pay(ledger, profile))
	s.Zero(ledger.writes)
	s.Equal(1, ledger.slots[coreCombat.ActionBonus])
}

// A free action is a nil cost — the doc's own phrasing. It is always payable
// and moves nothing.
func (s *GateTestSuite) TestNilProfileIsFree() {
	ledger := newSpyLedger()

	s.True(combat.CanPay(ledger, nil))
	s.Require().NoError(combat.Pay(ledger, nil))
	s.Zero(ledger.writes)
}

// A ledger that is not in combat has no economy to spend from, and that has to
// refuse even a profile that only GRANTS — otherwise the grant lands nowhere
// and the caller is told it succeeded.
func (s *GateTestSuite) TestNotInCombatPaysNothing() {
	ledger := newSpyLedger()
	ledger.inCombat = false

	profile := &combat.SpendProfile{
		Grants: map[combat.CapacityType]int{combat.CapacityAttack: 2},
	}

	s.False(combat.CanPay(ledger, profile))
	s.Require().Error(combat.Pay(ledger, profile))
	s.Zero(ledger.writes)
}

// A malformed profile is refused rather than partly honoured. Each of these
// names a cost the ledger has no way to charge, and silently charging nothing
// is how an action becomes free by accident.
func (s *GateTestSuite) TestMalformedProfilesAreRefused() {
	rich := func() *spyLedger {
		l := newSpyLedger()
		l.slots[coreCombat.ActionStandard] = 9
		l.slots[coreCombat.ActionBonus] = 9
		l.capacity[combat.CapacityAttack] = 9
		l.pools["ki"] = 9
		return l
	}

	cases := map[string]*combat.SpendProfile{
		"capacity keyed to none": {
			Capacity: map[combat.CapacityType]int{combat.CapacityNone: 1},
		},
		"grant keyed to none": {
			Grants: map[combat.CapacityType]int{combat.CapacityNone: 1},
		},
		"requirement keyed to none": {
			Requires: map[combat.CapacityType]int{combat.CapacityNone: 1},
		},
		"capacity outside the vocabulary": {
			Capacity: map[combat.CapacityType]int{combat.CapacityType("arcane_recovery_die"): 1},
		},
		"grant outside the vocabulary": {
			Grants: map[combat.CapacityType]int{combat.CapacityType("arcane_recovery_die"): 1},
		},
		"slot keyed to free": {
			Slots: map[coreCombat.ActionType]int{coreCombat.ActionFree: 1},
		},
		"slot keyed to movement": {
			Slots: map[coreCombat.ActionType]int{coreCombat.ActionMovement: 1},
		},
		"pool with no key": {
			Pools: map[coreResources.ResourceKey]int{"": 1},
		},
		"zero slot cost": {
			Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 0},
		},
		"negative capacity cost": {
			Capacity: map[combat.CapacityType]int{combat.CapacityAttack: -1},
		},
		"negative grant": {
			Grants: map[combat.CapacityType]int{combat.CapacityAttack: -1},
		},
		"negative pool cost": {
			Pools: map[coreResources.ResourceKey]int{"ki": -1},
		},
	}

	for name, profile := range cases {
		s.Run(name, func() {
			ledger := rich()
			s.False(combat.CanPay(ledger, profile))
			s.Require().Error(combat.Pay(ledger, profile))
			s.Zero(ledger.writes)
		})
	}
}

// CanPay and Pay answer from the same check, so there is no state in which one
// says yes and the other refuses.
func (s *GateTestSuite) TestCanPayAndPayAgree() {
	for available := 0; available <= 2; available++ {
		ledger := newSpyLedger()
		ledger.slots[coreCombat.ActionStandard] = available

		profile := &combat.SpendProfile{
			Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		}

		can := combat.CanPay(ledger, profile)
		err := combat.Pay(ledger, profile)
		s.Equal(can, err == nil, "available=%d", available)
	}
}
