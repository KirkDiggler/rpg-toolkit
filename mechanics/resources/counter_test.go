// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

type CounterTestSuite struct {
	suite.Suite
}

func TestCounterSuite(t *testing.T) {
	suite.Run(t, new(CounterTestSuite))
}

func (s *CounterTestSuite) TestNewCounter() {
	// With limit
	c := resources.NewCounter("death_saves", 3)
	s.Equal("death_saves", c.ID)
	s.Equal(0, c.Count)
	s.Equal(3, c.Limit)
	s.True(c.IsZero())
	s.False(c.AtLimit())
	s.True(c.HasLimit())

	// No limit
	c2 := resources.NewCounter("attacks", 0)
	s.Equal(0, c2.Limit)
	s.False(c2.HasLimit())
	s.False(c2.AtLimit())
}

func (s *CounterTestSuite) TestIncrement() {
	c := resources.NewCounter("concentration_fails", 3)

	// Normal increments
	err := c.Increment()
	s.NoError(err)
	s.Equal(1, c.Count)

	err = c.Increment()
	s.NoError(err)
	s.Equal(2, c.Count)

	err = c.Increment()
	s.NoError(err)
	s.Equal(3, c.Count)
	s.True(c.AtLimit())

	// Try to exceed limit
	err = c.Increment()
	s.Error(err)
	s.Contains(err.Error(), "at limit")
	s.Equal(3, c.Count)
}

func (s *CounterTestSuite) TestIncrementBy() {
	c := resources.NewCounter("test", 10)

	// Normal increment
	err := c.IncrementBy(5)
	s.NoError(err)
	s.Equal(5, c.Count)

	// Would exceed limit
	err = c.IncrementBy(6)
	s.Error(err)
	s.Contains(err.Error(), "would exceed limit")
	s.Equal(5, c.Count)

	// Exactly to limit
	err = c.IncrementBy(5)
	s.NoError(err)
	s.Equal(10, c.Count)
	s.True(c.AtLimit())

	// Negative amount
	err = c.IncrementBy(-1)
	s.Error(err)
	s.Contains(err.Error(), "negative amount")
}

func (s *CounterTestSuite) TestIncrementNoLimit() {
	c := resources.NewCounter("attacks", 0)

	// Should increment indefinitely
	for i := 0; i < 100; i++ {
		err := c.Increment()
		s.NoError(err)
	}
	s.Equal(100, c.Count)
	s.False(c.AtLimit())
}

func (s *CounterTestSuite) TestDecrement() {
	c := resources.NewCounter("test", 5)
	c.Count = 3

	// Normal decrement
	c.Decrement()
	s.Equal(2, c.Count)

	c.Decrement()
	s.Equal(1, c.Count)

	c.Decrement()
	s.Equal(0, c.Count)

	// Can't go negative
	c.Decrement()
	s.Equal(0, c.Count)
	s.True(c.IsZero())
}

func (s *CounterTestSuite) TestDecrementBy() {
	c := resources.NewCounter("test", 10)
	c.Count = 7

	// Normal decrement
	c.DecrementBy(3)
	s.Equal(4, c.Count)

	// Would go negative (floors at 0)
	c.DecrementBy(10)
	s.Equal(0, c.Count)

	// Negative amount (ignored)
	c.Count = 5
	c.DecrementBy(-2)
	s.Equal(5, c.Count)
}

func (s *CounterTestSuite) TestSetCount() {
	c := resources.NewCounter("test", 5)

	// Normal set
	err := c.SetCount(3)
	s.NoError(err)
	s.Equal(3, c.Count)

	// At limit
	err = c.SetCount(5)
	s.NoError(err)
	s.Equal(5, c.Count)
	s.True(c.AtLimit())

	// Exceed limit
	err = c.SetCount(6)
	s.Error(err)
	s.Contains(err.Error(), "exceeds limit")
	s.Equal(5, c.Count)

	// Negative
	err = c.SetCount(-1)
	s.Error(err)
	s.Contains(err.Error(), "cannot be negative")
}

func (s *CounterTestSuite) TestReset() {
	c := resources.NewCounter("test", 5)
	c.Count = 4

	c.Reset()
	s.Equal(0, c.Count)
	s.True(c.IsZero())
}
