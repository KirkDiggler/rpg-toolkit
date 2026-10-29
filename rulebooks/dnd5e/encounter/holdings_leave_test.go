// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// holdings_leave_test.go is the two endings a departure can fire, asked in
// ONE declaration-order pass (rpg-toolkit#2155 review): a `left` ending and
// an exited-holding ending at the same door, both holding on the same
// departure, end the run on the one the author listed first.

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// walkedAway is a `left` ending at the front gate: the party walked out.
func walkedAway() encounter.EndingInput {
	return encounter.EndingInput{
		Key: "walked-away", Trigger: encounter.TriggerLeft{Exit: frontGate}, Result: encounter.ResultFailure,
	}
}

// carryOutTheLast has the partner leave by the gate first, then the raider
// pick up the heirloom and leave by the gate last, holding it: every ending
// at the gate holds on that last departure. It returns what closed.
func (s *HoldingsSuite) carryOutTheLast(enc *encounter.Encounter) *encounter.Outcome {
	s.walkTo(enc, partner, raiderCell)
	out, err := enc.Exit(&encounter.ExitInput{Member: partner})
	s.Require().NoError(err)
	s.Require().Nil(out.Closed, "the raider is still inside")
	s.walkTo(enc, raider, heirloomCell)
	_, err = enc.Hold(&encounter.HoldInput{Member: raider, Target: heirloom})
	s.Require().NoError(err)
	s.walkTo(enc, raider, raiderCell)
	out, err = enc.Exit(&encounter.ExitInput{Member: raider})
	s.Require().NoError(err)
	s.Require().NotNil(out.Closed, "the last one out, holding the heirloom, ends it")

	return out.Closed
}

// TestALeftEndingDeclaredFirstOutranksAnExitedHoldingOne is the review's
// probe, kept as the regression test: `walked-away` is listed first, so it
// is the ending, with its outcome.
func (s *HoldingsSuite) TestALeftEndingDeclaredFirstOutranksAnExitedHoldingOne() {
	closed := s.carryOutTheLast(s.open(false, walkedAway(), recoverEnding()))

	s.Equal("walked-away", closed.Ending)
	s.Equal(encounter.ResultFailure, closed.Result)
}

// TestAnExitedHoldingEndingDeclaredFirstOutranksALeftOne is the mirror:
// the same departure, the order reversed, and the heirloom's ending wins.
func (s *HoldingsSuite) TestAnExitedHoldingEndingDeclaredFirstOutranksALeftOne() {
	closed := s.carryOutTheLast(s.open(false, recoverEnding(), walkedAway()))

	s.Equal(recovered, closed.Ending)
}
