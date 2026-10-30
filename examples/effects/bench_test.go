// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

// BenchmarkGuardAnswer is the Rage damage guard (three facts under all)
// answered over a known Strength melee weapon frame.
func BenchmarkGuardAnswer(b *testing.B) {
	p, err := rageRecipe().Compile(rageVocabulary())
	if err != nil {
		b.Fatal(err)
	}
	g, err := p.CompileGuard(&machine.Guard{All: []machine.Guard{
		{Fact: &machine.FactLeaf{Name: factMelee, Op: machine.OpEq, Value: true}},
		{Fact: &machine.FactLeaf{Name: factWeaponPool, Op: machine.OpEq, Value: true}},
		{Fact: &machine.FactLeaf{Name: factAbility, Op: machine.OpEq, Value: abilitySTR}},
	}}, "")
	if err != nil {
		b.Fatal(err)
	}
	frame := strMelee(9)
	for b.Loop() {
		if v, _ := g.Answer(frame, nil, nil); v != machine.Applies {
			b.Fatal(v)
		}
	}
}

// BenchmarkBeatTurnEnd is one turn end on a composed Rage in raging with its
// flags set: the sustain check, the cap, the clock check and the upkeep
// transition, with the four outputs attached. Each iteration first sets the
// flag with one post-attack-roll (the tracker), since the turn end clears
// it; BenchmarkBeatTurnEnd/tracker_only measures that part alone.
func BenchmarkBeatTurnEnd(b *testing.B) {
	ctx := context.Background()
	newRaging := func(b *testing.B) events.EventBus {
		bag := machine.Bag{machine.KeyRoundActivated: 1, machine.KeySawTurnEnd: true}
		c, err := effects.Compose(rageRecipe(), rageVocabulary(), outputVocabulary(), holder, nil, bag, stRaging)
		if err != nil {
			b.Fatal(err)
		}
		bus := events.NewEventBus()
		if err := c.Apply(ctx, bus); err != nil {
			b.Fatal(err)
		}
		return bus
	}
	attack := func(b *testing.B, bus events.EventBus) {
		if _, err := postAttackRollChain.On(bus).PublishWithChain(ctx, postAttackRoll{AttackerID: holder},
			events.NewStagedChain[postAttackRoll]([]chain.Stage{})); err != nil {
			b.Fatal(err)
		}
	}
	b.Run("tracker_then_turn_end", func(b *testing.B) {
		bus := newRaging(b)
		turns := turnEndTopic.On(bus)
		for b.Loop() {
			attack(b, bus)
			if err := turns.Publish(ctx, turnEnd{SubjectID: holder, Round: 2}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("tracker_only", func(b *testing.B) {
		bus := newRaging(b)
		for b.Loop() {
			attack(b, bus)
		}
	})
}
