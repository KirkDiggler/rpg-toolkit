// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// DeedReachSuite holds every deed to a recorded reach, so a deed's sentence
// can say a verb that is true of every way it can be held.
type DeedReachSuite struct{ suite.Suite }

func TestDeedReachSuite(t *testing.T) { suite.Run(t, new(DeedReachSuite)) }

// TestEveryWhenWordHasAReach walks every word a `when` deed condition may
// name: one without a reach would read back as a sentence nobody checked.
func (s *DeedReachSuite) TestEveryWhenWordHasAReach() {
	for _, w := range WhenDeeds {
		s.NotEmpty(DeedReachOf(w), "the `when` deed word %q has no reach recorded in deedReaches", w)
	}
}

// TestEveryVerbHasAReach walks every deed verb this composition lands.
func (s *DeedReachSuite) TestEveryVerbHasAReach() {
	for _, v := range []string{DeedAttack, DeedIntimidate, DeedPersuade, DeedUse, DeedFled, DeedFell} {
		s.NoError(reachRecorded(v))
	}
	s.ErrorContains(reachRecorded("shoved"), `the verb "shoved" has no reach recorded`)
}

// TestTheReachesAreTheLandings pins each reach to how its landing chooses
// its audience (deed.go, answer.go, option.go, interact_prop.go).
func (s *DeedReachSuite) TestTheReachesAreTheLandings() {
	s.Equal([]DeedReach{ReachSeen}, DeedReachOf("attacked"))
	s.Equal([]DeedReach{ReachSeen}, DeedReachOf("intimidated"))
	s.Equal([]DeedReach{ReachSeen}, DeedReachOf("persuaded"))
	s.Equal([]DeedReach{ReachOwn}, DeedReachOf("fled"))
	s.Equal([]DeedReach{ReachSeen, ReachCaused}, DeedReachOf("fell"))
	s.Nil(DeedReachOf("shoved"))
}
