// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestTheCastlePlaysFromItsBlob pins the harness's reload mode
// (rpg-project#667): ending a turn plays on from the encounter saved and
// loaded back, unless the route or the run opts out. A harness that silently
// stopped reloading would let every blob defect back past the castle.
func TestTheCastlePlaysFromItsBlob(t *testing.T) {
	t.Parallel()
	if !*castleReload {
		t.Skip("the run opted out with -castle.reload=false")
	}

	s := openTheScript(t, gatehouseSeat, nil)
	s.struck(kingGate1, false)
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "precondition: a fight to end a turn in")
	before := s.enc
	s.endTurn()
	require.NotSame(t, before, s.enc, "the turn ended on the encounter loaded back from its blob")

	kept := openTheCastleAs(t, castleHost{at: gatehouseSeat, noReload: true})
	kept.struck(kingGate1, false)
	before = kept.enc
	kept.endTurn()
	require.Same(t, before, kept.enc, "a route that opts out plays on the encounter it opened")
}
