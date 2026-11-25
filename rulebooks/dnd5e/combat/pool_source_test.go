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

// spySource is a pool source that owns named pools and counts its writes.
type spySource struct {
	pools  map[coreResources.ResourceKey]int
	writes int
}

func newSpySource() *spySource {
	return &spySource{pools: map[coreResources.ResourceKey]int{}}
}

func (p *spySource) HasPool(key coreResources.ResourceKey) bool {
	_, ok := p.pools[key]
	return ok
}

func (p *spySource) PoolLeft(key coreResources.ResourceKey) int { return p.pools[key] }

func (p *spySource) SpendPool(key coreResources.ResourceKey, n int) {
	p.writes++
	p.pools[key] -= n
}

// PoolSourceTestSuite pins that one payment through a composed ledger spends
// the payer's slots and a second budget's pool together, or neither.
type PoolSourceTestSuite struct {
	suite.Suite
}

func TestPoolSourceSuite(t *testing.T) {
	suite.Run(t, new(PoolSourceTestSuite))
}

func (s *PoolSourceTestSuite) bonusAndCharge() *combat.SpendProfile {
	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools: map[coreResources.ResourceKey]int{"charges": 1},
	}
}

func (s *PoolSourceTestSuite) TestPaysBonusActionAndSourcePoolTogether() {
	base := newSpyLedger()
	base.slots[coreCombat.ActionBonus] = 1
	source := newSpySource()
	source.pools["charges"] = 1

	s.Require().NoError(combat.Pay(combat.WithPools(base, source), s.bonusAndCharge()))

	s.Equal(0, base.slots[coreCombat.ActionBonus])
	s.Equal(0, source.pools["charges"])
	_, touched := base.pools["charges"]
	s.False(touched, "the base must never be asked to spend a source pool")
	s.Equal(1, base.writes)
	s.Equal(1, source.writes)
}

func (s *PoolSourceTestSuite) TestSourceEmptyRefusesAndSpendsNothing() {
	base := newSpyLedger()
	base.slots[coreCombat.ActionBonus] = 1
	source := newSpySource()
	source.pools["charges"] = 0

	err := combat.Pay(combat.WithPools(base, source), s.bonusAndCharge())

	s.Require().Error(err)
	s.Contains(err.Error(), "charges: 1 needed, 0 left")
	s.Equal(1, base.slots[coreCombat.ActionBonus])
	s.Zero(base.writes)
	s.Zero(source.writes)
}

func (s *PoolSourceTestSuite) TestBaseOutOfSlotsLeavesSourceUntouched() {
	base := newSpyLedger()
	source := newSpySource()
	source.pools["charges"] = 1

	err := combat.Pay(combat.WithPools(base, source), s.bonusAndCharge())

	s.Require().Error(err)
	s.Equal(1, source.pools["charges"])
	s.Zero(source.writes)
	s.Zero(base.writes)
}

func (s *PoolSourceTestSuite) TestBaseNotInCombatRefusesAndSourceUntouched() {
	base := newSpyLedger()
	base.inCombat = false
	base.slots[coreCombat.ActionBonus] = 1
	source := newSpySource()
	source.pools["charges"] = 1

	err := combat.Pay(combat.WithPools(base, source), s.bonusAndCharge())

	s.Require().Error(err)
	s.Contains(err.Error(), "not in combat")
	s.Equal(1, source.pools["charges"])
	s.Zero(source.writes)
	s.Zero(base.writes)
}

func (s *PoolSourceTestSuite) TestPoolTheSourceDoesNotOwnGoesToBase() {
	base := newSpyLedger()
	base.pools["second_wind"] = 1
	source := newSpySource()
	source.pools["charges"] = 1

	profile := &combat.SpendProfile{
		Pools: map[coreResources.ResourceKey]int{"second_wind": 1, "charges": 1},
	}
	s.Require().NoError(combat.Pay(combat.WithPools(base, source), profile))

	s.Equal(0, base.pools["second_wind"])
	s.Equal(0, source.pools["charges"])
	_, touched := base.pools["charges"]
	s.False(touched)
	s.Equal(1, base.writes)
	s.Equal(1, source.writes)
}

func (s *PoolSourceTestSuite) TestSourceOwnPoolWinsOverBaseSameName() {
	base := newSpyLedger()
	base.pools["charges"] = 5
	source := newSpySource()
	source.pools["charges"] = 1

	profile := &combat.SpendProfile{
		Pools: map[coreResources.ResourceKey]int{"charges": 1},
	}
	s.Require().NoError(combat.Pay(combat.WithPools(base, source), profile))

	s.Equal(0, source.pools["charges"])
	s.Equal(5, base.pools["charges"])
	s.Zero(base.writes)
}

func (s *PoolSourceTestSuite) TestNilSourceIsTheBase() {
	base := newSpyLedger()
	base.slots[coreCombat.ActionBonus] = 1
	base.pools["charges"] = 1

	composed := combat.WithPools(base, nil)
	s.Same(base, composed)

	s.Require().NoError(combat.Pay(composed, s.bonusAndCharge()))
	s.Equal(0, base.slots[coreCombat.ActionBonus])
	s.Equal(0, base.pools["charges"])
	s.Equal(2, base.writes)
}

func (s *PoolSourceTestSuite) TestNilBaseIsAnUntypedNilLedger() {
	source := newSpySource()
	source.pools["charges"] = 1

	composed := combat.WithPools(nil, source)
	s.True(composed == nil, "a nil base must come back as an untyped nil Ledger")

	err := combat.Pay(composed, s.bonusAndCharge())
	s.Require().Error(err)
	s.Contains(err.Error(), "no ledger to pay from")
	s.Zero(source.writes)

	var typed *spyLedger
	s.True(combat.WithPools(typed, source) != nil, "a typed-nil base is not the nil case")
}

func (s *PoolSourceTestSuite) TestEverythingElseReachesTheBase() {
	profile := &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Capacity: map[combat.CapacityType]int{combat.CapacityFlurryStrike: 1},
		Requires: map[combat.CapacityType]int{combat.CapacityFlurryStrike: 1},
		Grants:   map[combat.CapacityType]int{combat.CapacityAttack: 2},
	}
	fill := func(l *spyLedger) {
		l.slots[coreCombat.ActionBonus] = 1
		l.capacity[combat.CapacityFlurryStrike] = 2
	}

	alone := newSpyLedger()
	fill(alone)
	s.Require().NoError(combat.Pay(alone, profile))

	through := newSpyLedger()
	fill(through)
	source := newSpySource()
	source.pools["charges"] = 1
	s.Require().NoError(combat.Pay(combat.WithPools(through, source), profile))

	s.Equal(alone.writes, through.writes)
	s.Equal(alone.slots, through.slots)
	s.Equal(alone.capacity, through.capacity)
	s.Equal(alone.pools, through.pools)
	s.Zero(source.writes)
}
