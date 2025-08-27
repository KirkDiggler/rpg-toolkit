// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

type PoolTestSuite struct {
	suite.Suite
}

func TestPoolSuite(t *testing.T) {
	suite.Run(t, new(PoolTestSuite))
}

func (s *PoolTestSuite) TestNewPool() {
	pool := resources.NewPool()
	s.NotNil(pool)
	s.NotNil(pool.Resources)
	s.NotNil(pool.Counters)
	s.Empty(pool.Resources)
	s.Empty(pool.Counters)
}

func (s *PoolTestSuite) TestAddGetResource() {
	pool := resources.NewPool()

	// Add resources
	spell := resources.NewResource("spell_slots", 3)
	rage := resources.NewResource("rage", 2)

	pool.AddResource(spell)
	pool.AddResource(rage)

	// Get existing
	r, ok := pool.GetResource("spell_slots")
	s.True(ok)
	s.Equal(spell, r)

	r, ok = pool.GetResource("rage")
	s.True(ok)
	s.Equal(rage, r)

	// Get non-existing
	r, ok = pool.GetResource("not_there")
	s.False(ok)
	s.Nil(r)

	// Add nil (should be ignored)
	pool.AddResource(nil)
	s.Len(pool.Resources, 2)
}

func (s *PoolTestSuite) TestAddGetCounter() {
	pool := resources.NewPool()

	// Add counters
	death := resources.NewCounter("death_saves", 3)
	attacks := resources.NewCounter("attacks", 0)

	pool.AddCounter(death)
	pool.AddCounter(attacks)

	// Get existing
	c, ok := pool.GetCounter("death_saves")
	s.True(ok)
	s.Equal(death, c)

	c, ok = pool.GetCounter("attacks")
	s.True(ok)
	s.Equal(attacks, c)

	// Get non-existing
	c, ok = pool.GetCounter("not_there")
	s.False(ok)
	s.Nil(c)

	// Add nil (should be ignored)
	pool.AddCounter(nil)
	s.Len(pool.Counters, 2)
}

func (s *PoolTestSuite) TestRemoveResource() {
	pool := resources.NewPool()

	r1 := resources.NewResource("resource1", 5)
	r2 := resources.NewResource("resource2", 10)

	pool.AddResource(r1)
	pool.AddResource(r2)
	s.Len(pool.Resources, 2)

	// Remove existing
	pool.RemoveResource("resource1")
	s.Len(pool.Resources, 1)

	_, ok := pool.GetResource("resource1")
	s.False(ok)

	_, ok = pool.GetResource("resource2")
	s.True(ok)

	// Remove non-existing (should not panic)
	pool.RemoveResource("not_there")
	s.Len(pool.Resources, 1)
}

func (s *PoolTestSuite) TestRemoveCounter() {
	pool := resources.NewPool()

	c1 := resources.NewCounter("counter1", 3)
	c2 := resources.NewCounter("counter2", 0)

	pool.AddCounter(c1)
	pool.AddCounter(c2)
	s.Len(pool.Counters, 2)

	// Remove existing
	pool.RemoveCounter("counter1")
	s.Len(pool.Counters, 1)

	_, ok := pool.GetCounter("counter1")
	s.False(ok)

	_, ok = pool.GetCounter("counter2")
	s.True(ok)

	// Remove non-existing (should not panic)
	pool.RemoveCounter("not_there")
	s.Len(pool.Counters, 1)
}

func (s *PoolTestSuite) TestClear() {
	pool := resources.NewPool()

	// Add some items
	pool.AddResource(resources.NewResource("res1", 5))
	pool.AddResource(resources.NewResource("res2", 10))
	pool.AddCounter(resources.NewCounter("cnt1", 3))
	pool.AddCounter(resources.NewCounter("cnt2", 0))

	s.Len(pool.Resources, 2)
	s.Len(pool.Counters, 2)

	// Clear all
	pool.Clear()

	s.Empty(pool.Resources)
	s.Empty(pool.Counters)
	s.NotNil(pool.Resources) // Maps should still exist
	s.NotNil(pool.Counters)
}

func (s *PoolTestSuite) TestRestoreAllResources() {
	pool := resources.NewPool()

	// Add resources with depleted values
	spell := resources.NewResource("spell_slots", 3)
	spell.SetCurrent(1)

	rage := resources.NewResource("rage", 2)
	rage.SetCurrent(0)

	ki := resources.NewResource("ki", 5)
	// ki is already full

	pool.AddResource(spell)
	pool.AddResource(rage)
	pool.AddResource(ki)

	// Restore all
	pool.RestoreAllResources()

	// Check all are full
	r, _ := pool.GetResource("spell_slots")
	s.True(r.IsFull())
	s.Equal(3, r.Current)

	r, _ = pool.GetResource("rage")
	s.True(r.IsFull())
	s.Equal(2, r.Current)

	r, _ = pool.GetResource("ki")
	s.True(r.IsFull())
	s.Equal(5, r.Current)
}

func (s *PoolTestSuite) TestResetAllCounters() {
	pool := resources.NewPool()

	// Add counters with various counts
	death := resources.NewCounter("death_saves", 3)
	death.Count = 2

	attacks := resources.NewCounter("attacks", 0)
	attacks.Count = 10

	conc := resources.NewCounter("concentration_fails", 2)
	// conc is already at 0

	pool.AddCounter(death)
	pool.AddCounter(attacks)
	pool.AddCounter(conc)

	// Reset all
	pool.ResetAllCounters()

	// Check all are zero
	c, _ := pool.GetCounter("death_saves")
	s.True(c.IsZero())
	s.Equal(0, c.Count)

	c, _ = pool.GetCounter("attacks")
	s.True(c.IsZero())
	s.Equal(0, c.Count)

	c, _ = pool.GetCounter("concentration_fails")
	s.True(c.IsZero())
	s.Equal(0, c.Count)
}

func (s *PoolTestSuite) TestPoolUsageExample() {
	// Example of typical usage
	pool := resources.NewPool()

	// Character resources
	pool.AddResource(resources.NewResource("hit_points", 45))
	pool.AddResource(resources.NewResource("spell_slots_1", 4))
	pool.AddResource(resources.NewResource("spell_slots_2", 3))
	pool.AddResource(resources.NewResource("ki", 5))
	pool.AddResource(resources.NewResource("rage", 3))

	// Combat counters
	pool.AddCounter(resources.NewCounter("death_saves", 3))
	pool.AddCounter(resources.NewCounter("death_fails", 3))
	pool.AddCounter(resources.NewCounter("attacks_this_turn", 0))

	// Use some resources
	hp, _ := pool.GetResource("hit_points")
	err := hp.Use(10) // Take damage
	s.NoError(err)
	s.Equal(35, hp.Current)

	spell1, _ := pool.GetResource("spell_slots_1")
	err = spell1.Use(1) // Cast a spell
	s.NoError(err)
	s.Equal(3, spell1.Current)

	// Track attacks
	attacks, _ := pool.GetCounter("attacks_this_turn")
	err = attacks.Increment()
	s.NoError(err)
	err = attacks.Increment()
	s.NoError(err)
	s.Equal(2, attacks.Count)

	// Long rest - restore everything
	pool.RestoreAllResources()
	pool.ResetAllCounters()

	// Verify reset
	s.True(hp.IsFull())
	s.True(spell1.IsFull())
	s.True(attacks.IsZero())
}
