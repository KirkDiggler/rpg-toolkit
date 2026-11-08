// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestAPlainLineToAPropRingsItsBellAndPaysNoRound is rpg-project#785 R5 at a
// prop: a plain line lands, so it is the prop's deed — `clock: bell:<prop>`
// lands on whoever listens for it — and it pays the world no round. The bell
// is matched on the next raise somebody pays for; the plain line raises none.
func TestAPlainLineToAPropRingsItsBellAndPaysNoRound(t *testing.T) {
	no := false
	march := recipe(t, "waiting", map[string]machine.State{
		"waiting": {Transitions: []machine.Transition{{On: clockBeat("bell:gong"), To: "heard"}}},
		"heard":   {},
	})
	m := listener("m", 4, 1, march)
	in := wakeSetup(passDriver{}, m)
	in.Field.Props = []PropInput{{
		ID: "gong", Ref: "test:props:gong", At: spatial.Position{X: 1, Y: 1},
		BlocksMovement: &no, BlocksLineOfSight: &no,
		Options: []Option{{ID: "ring", Say: "(strike the gong)"}},
	}}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	before := enc.clock.ToData().HighWater

	out, err := enc.Speak(context.Background(), &SpeakInput{
		Actor: "alice", Prop: "gong", OptionID: "ring", Roller: wakeRollsOne{},
	})
	require.NoError(t, err)
	require.True(t, out.Beaten, "a plain line lands")
	require.Equal(t, before, enc.clock.ToData().HighWater, "and pays the world no round")
	require.Empty(t, beatsTagged(t, enc, BeatTick), "so no tick is written")
	rung := false
	for _, b := range enc.landed {
		rung = rung || b.Beat == clockBeat("bell:gong")
	}
	require.True(t, rung, "the bell landed on its listener: %v", enc.landed)
}
