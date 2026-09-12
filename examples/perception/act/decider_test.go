// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package act_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/act"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// This file is the how-to. Every test below builds a situation by hand — no
// game, no projection, no world, no journal. A behaviour is a pure function of
// what one actor holds, so testing one needs nothing but the holdings.

const hearing testimony.Channel = "hearing"

// holds builds one thing an actor holds. Pass an empty name for something they
// have perceived and have no word for.
func holds(
	name belief.Name, channel testimony.Channel, where string, current bool, of string, kind content.Kind,
) act.Held {
	return act.Held{
		TrackView: reconcile.TrackView{
			ID:      testimony.TrackID("t-" + string(name) + "-" + string(channel) + "-" + of),
			Channel: channel,
			Payload: content.Encode(content.Percept{Kind: kind, Of: of}),
			Locus:   testimony.Locus{Where: where},
			Current: current,
		},
		Name:  name,
		Named: name != "",
	}
}

func situation(h ...act.Held) act.Situation {
	return act.Situation{Actor: "a-goblin", Holds: h, At: testimony.Stamp{Tick: 1}}
}

// TestChannelChangesTheAnswer: the same situation, two behaviours, and the
// difference is only which sense the testimony came in on.
func TestChannelChangesTheAnswer(t *testing.T) {
	heard := situation(holds("the intruders", hearing, "corridor", true, "", content.Noise))

	intent, acting := act.Aggressive{}.Decide(heard)
	require.True(t, acting, "it will swing at a noise")
	assert.Equal(t, "attack", intent.Verb)

	_, acting = act.Cautious{}.Decide(heard)
	assert.False(t, acting, "it has heard something and seen nothing, so it waits")

	// Now it sees them.
	seen := situation(
		holds("the intruders", hearing, "corridor", true, "", content.Noise),
		holds("the intruders", testimony.Sight, "corridor", true, "human", content.Creature),
	)

	intent, acting = act.Cautious{}.Decide(seen)
	require.True(t, acting)
	assert.Equal(t, belief.Name("the intruders"), intent.Target)
}

// TestMemoryIsNotASighting: Wary goes to look at what it only remembers, and
// fights what is actually in front of it.
func TestMemoryIsNotASighting(t *testing.T) {
	remembered := situation(
		holds("the intruders", testimony.Sight, "corridor", false, "human", content.Creature),
	)

	intent, acting := act.Wary{}.Decide(remembered)
	require.True(t, acting)
	assert.Equal(t, "approach", intent.Verb,
		"it does not know whether they are still there, and going to look is the answer")

	facing := situation(
		holds("the intruders", testimony.Sight, "corridor", false, "human", content.Creature),
		holds("the scout", testimony.Sight, "doorway", true, "human", content.Creature),
	)

	intent, acting = act.Wary{}.Decide(facing)
	require.True(t, acting)
	assert.Equal(t, "attack", intent.Verb)
	assert.Equal(t, belief.Name("the scout"), intent.Target, "the one it can actually see")
}

// TestCowardlyCountsOnlyWhatItCanPerceive: a memory is not a threat standing in
// front of you, and an intent with no target is the honest shape for fleeing.
func TestCowardlyCountsOnlyWhatItCanPerceive(t *testing.T) {
	brave := act.Cowardly{Tolerates: 1}

	_, acting := brave.Decide(situation(
		holds("the scout", testimony.Sight, "doorway", true, "human", content.Creature),
		holds("the intruders", testimony.Sight, "corridor", false, "human", content.Creature),
	))
	assert.False(t, acting, "one in front of it, one only remembered")

	intent, acting := brave.Decide(situation(
		holds("the scout", testimony.Sight, "doorway", true, "human", content.Creature),
		holds("the knight", testimony.Sight, "doorway", true, "human", content.Creature),
	))
	require.True(t, acting)
	assert.Equal(t, "flee", intent.Verb)
	assert.Empty(t, intent.Target, "fleeing is not aimed at anything")
}

// TestNothingAimsAtTheUnnamed: perceiving is not enough. A behaviour reaches for
// a name, and a name is something the actor had to work out.
func TestNothingAimsAtTheUnnamed(t *testing.T) {
	unnamed := situation(holds("", testimony.Sight, "doorway", true, "human", content.Creature))

	require.NotEmpty(t, unnamed.Current(), "it is looking right at them")
	assert.Empty(t, unnamed.Named(), "and has no word for them")

	for name, d := range map[string]act.Decider{
		"aggressive": act.Aggressive{},
		"cautious":   act.Cautious{},
		"wary":       act.Wary{},
		"cowardly":   act.Cowardly{Tolerates: 0},
	} {
		t.Run(name, func(t *testing.T) {
			_, acting := d.Decide(unnamed)
			assert.False(t, acting, "there is nothing it can aim at")
		})
	}
}

// TestWritingYourOwn is the shape an NPC author writes: a func, the holdings,
// and no imports beyond the vocabulary of belief.
func TestWritingYourOwn(t *testing.T) {
	// A sentry that raises the alarm about anything it can perceive in the
	// place it is guarding, seen or heard, and ignores everywhere else.
	sentry := func(post string) act.DeciderFunc {
		return func(s act.Situation) (act.Intent, bool) {
			for _, h := range s.Named() {
				if !h.Current || h.Locus.Where != post {
					continue
				}

				return act.Intent{Verb: "raise-the-alarm", Target: h.Name}, true
			}

			return act.Intent{}, false
		}
	}

	elsewhere := situation(holds("the intruders", testimony.Sight, "corridor", true, "human", content.Creature))

	_, acting := sentry("doorway").Decide(elsewhere)
	assert.False(t, acting, "not its post")

	intent, acting := sentry("corridor").Decide(elsewhere)
	require.True(t, acting)
	assert.Equal(t, "raise-the-alarm", intent.Verb)
}

// TestIdleIsADecision: doing nothing is what most things do most of the time,
// and it is returned rather than inferred from an empty intent.
func TestIdleIsADecision(t *testing.T) {
	_, acting := act.Idle{}.Decide(situation(
		holds("the intruders", testimony.Sight, "corridor", true, "human", content.Creature),
	))
	assert.False(t, acting)
}
