// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// StepShapeSuite holds one step's shape connections where no compiled site
// can reach: the engine refuses two picks on one step today, and the
// read-back still says each pick it is given (rpg-project#994).
type StepShapeSuite struct{ suite.Suite }

func TestStepShapeSuite(t *testing.T) { suite.Run(t, new(StepShapeSuite)) }

func (s *StepShapeSuite) TestTwoPicksAreTwoConnections() {
	conns := stepShape("posted", machine.Transition{On: "time", Do: []machine.Act{{Pick: "post"}, {Pick: "idle"}}})
	s.Contains(conns, Connection{Kind: ConnTable, ID: "post", Role: RolePicks})
	s.Contains(conns, Connection{Kind: ConnTable, ID: "idle", Role: RolePicks})
}

func (s *StepShapeSuite) TestTheBeatIsTheAuthoredWord() {
	for _, on := range []machine.Beat{"deed:x", "clock:world", "heard"} {
		conns := stepShape("posted", machine.Transition{On: on})
		s.Contains(conns, Connection{Kind: ConnBeat, ID: string(on), Role: RoleOn}, on)
	}
}
