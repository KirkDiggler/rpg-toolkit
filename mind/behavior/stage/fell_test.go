// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package stage_test

// fell_test.go is the fall's filing law on a real store (rpg-project#716,
// ruled 2026-10-29): a fall is filed by who fell as well as who caused it,
// so one figure's two falls are two memories; every other verb still keeps
// only an actor's freshest deed of its kind.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/stage"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// heldOf is every deed of one verb a witness holds, as a table reads it.
func heldOf(t *testing.T, store *perception.Perception, witness core.EntityID, verb string) []behavior.HeldDeed {
	t.Helper()
	held, err := store.Held(witness)
	require.NoError(t, err)
	var out []behavior.HeldDeed
	for _, h := range held {
		if h.Channel != deed.Channel {
			continue
		}
		d, derr := deed.Decode(h.Payload)
		require.NoError(t, derr)
		if d.Verb == verb {
			out = append(out, behavior.HeldDeed{Kind: d.Verb, Actor: d.Actor, Target: d.Target, At: h.Confirmed})
		}
	}

	return out
}

// TestAWitnessHoldsEveryFallOneFigureCaused is the ruling's scene: the hero
// drops the king, then a loyalist, in sight of the watcher. Filed by actor
// and verb alone the second fall replaced the first; filed by who fell, the
// watcher holds both, and a row naming either one holds.
func TestAWitnessHoldsEveryFallOneFigureCaused(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)
	present := []core.EntityID{"hero", "king", "loyalist"}
	look(t, store, eyesOn{"watcher": present}, present, 1)

	for fallen, at := range map[core.EntityID]uint64{"king": 2, "loyalist": 3} {
		require.NoError(t, stage.Land(&stage.LandInput{
			Store:     store,
			Deed:      deed.Deed{Verb: deed.VerbFell, Actor: "hero", Target: fallen, Where: "throne"},
			Witnesses: []core.EntityID{"watcher"},
			At:        at,
		}))
	}

	falls := heldOf(t, store, "watcher", deed.VerbFell)
	require.ElementsMatch(t, []behavior.HeldDeed{
		{Kind: deed.VerbFell, Actor: "hero", Target: "king", At: 2},
		{Kind: deed.VerbFell, Actor: "hero", Target: "loyalist", At: 3},
	}, falls, "two falls by one figure are two memories")

	for _, who := range []core.EntityID{"king", "loyalist"} {
		chosen, perr := behavior.Pick(context.Background(), &behavior.PickInput{
			Key: behavior.KeyTime,
			Table: behavior.Table{behavior.KeyTime: {
				{Weight: 1, Hold: true, When: []behavior.When{{Deed: "fell", Within: 3, Who: who, Scope: behavior.ScopeAlly}}},
				{Weight: 1, Hold: true},
			}},
			Facts: behavior.Facts{Now: 4, AllyDeeds: falls},
			Die:   firstFace{},
		})
		require.NoError(t, perr)
		require.Equal(t, 0, chosen.Entry, "`fell: { who: %s }` still holds", who)
	}
}

// TestTwoBlowsByOneFigureStillEvict: the other verbs keep their law. The
// hero strikes the king and then the loyalist; the watcher holds the
// freshest attack of the hero's and nothing before it.
func TestTwoBlowsByOneFigureStillEvict(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)
	present := []core.EntityID{"hero", "king", "loyalist"}
	look(t, store, eyesOn{"watcher": present}, present, 4)

	// In order: the king first, then the loyalist.
	for _, blow := range []struct {
		struck core.EntityID
		at     uint64
	}{{"king", 5}, {"loyalist", 6}} {
		require.NoError(t, stage.Land(&stage.LandInput{
			Store:     store,
			Deed:      deed.Deed{Verb: "attack", Actor: "hero", Target: blow.struck, Where: "throne"},
			Witnesses: []core.EntityID{"watcher"},
			At:        blow.at,
		}))
	}

	require.Equal(t, []behavior.HeldDeed{{Kind: "attack", Actor: "hero", Target: "loyalist", At: 6}},
		heldOf(t, store, "watcher", "attack"), "the second blow replaced the first, as it always did")
	_, _, held := heldDeed(t, store, "watcher", "hero", "attack")
	require.True(t, held, "filed under the actor and verb alone")
}

// firstFace is a die that always lands on 1.
type firstFace struct{}

func (firstFace) Roll(context.Context, int) (int, error) { return 1, nil }
