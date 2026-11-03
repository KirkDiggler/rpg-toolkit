// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// TrackerSuite pins newTracker's seam inside the package, where the signal
// lives: errNoGoals never crosses the API, so an external test cannot see it.
type TrackerSuite struct {
	suite.Suite
}

func TestTrackerSuite(t *testing.T) {
	suite.Run(t, new(TrackerSuite))
}

// TestNoGoalsIsASignalNotAFailure pins #2220's ruling at this seam: a world
// with no goals is not a failure — newTracker answers errNoGoals, the signal
// New consumes so the world runs without a tracker. The (nil, nil) this
// replaces could not be told from a wiring failure, which is the hole the
// ruling closes.
func (s *TrackerSuite) TestNoGoalsIsASignalNotAFailure() {
	tracker, err := newTracker(Config{})
	s.Nil(tracker)
	s.ErrorIs(err, errNoGoals)
}
