// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

type ResourceTestSuite struct {
	suite.Suite
}

func TestResourceSuite(t *testing.T) {
	suite.Run(t, new(ResourceTestSuite))
}

func (s *ResourceTestSuite) TestNewResource() {
	r := resources.NewResource("rage", 3)

	s.Equal("rage", r.ID)
	s.Equal(3, r.Current)
	s.Equal(3, r.Maximum)
	s.True(r.IsAvailable())
	s.True(r.IsFull())
	s.False(r.IsEmpty())
}

func (s *ResourceTestSuite) TestUse() {
	r := resources.NewResource("spell_slots", 4)

	// Use some
	err := r.Use(2)
	s.NoError(err)
	s.Equal(2, r.Current)
	s.True(r.IsAvailable())
	s.False(r.IsFull())

	// Use rest
	err = r.Use(2)
	s.NoError(err)
	s.Equal(0, r.Current)
	s.False(r.IsAvailable())
	s.True(r.IsEmpty())

	// Try to use more than available
	err = r.Use(1)
	s.Error(err)
	s.Contains(err.Error(), "insufficient")

	// Try negative
	err = r.Use(-1)
	s.Error(err)
	s.Contains(err.Error(), "negative")
}

func (s *ResourceTestSuite) TestRestore() {
	r := resources.NewResource("ki", 5)
	r.SetCurrent(1)

	// Partial restore
	r.Restore(2)
	s.Equal(3, r.Current)

	// Restore beyond max (should cap)
	r.Restore(10)
	s.Equal(5, r.Current)
	s.True(r.IsFull())

	// Restore negative (should ignore)
	r.SetCurrent(3)
	r.Restore(-2)
	s.Equal(3, r.Current)
}

func (s *ResourceTestSuite) TestRestoreToFull() {
	r := resources.NewResource("hit_dice", 10)
	r.SetCurrent(3)

	r.RestoreToFull()
	s.Equal(10, r.Current)
	s.True(r.IsFull())
}

func (s *ResourceTestSuite) TestSetCurrent() {
	r := resources.NewResource("test", 10)

	// Normal set
	r.SetCurrent(5)
	s.Equal(5, r.Current)

	// Beyond max (should cap)
	r.SetCurrent(15)
	s.Equal(10, r.Current)

	// Negative (should floor at 0)
	r.SetCurrent(-5)
	s.Equal(0, r.Current)
}

func (s *ResourceTestSuite) TestSetMaximum() {
	r := resources.NewResource("test", 10)
	r.SetCurrent(8)

	// Increase max
	r.SetMaximum(15)
	s.Equal(15, r.Maximum)
	s.Equal(8, r.Current) // Current unchanged

	// Decrease max below current
	r.SetMaximum(5)
	s.Equal(5, r.Maximum)
	s.Equal(5, r.Current) // Current capped

	// Negative max (should floor at 0)
	r.SetMaximum(-1)
	s.Equal(0, r.Maximum)
	s.Equal(0, r.Current)
}
