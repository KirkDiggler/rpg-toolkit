// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

type observationsSuite struct{ suite.Suite }

// Missing Changed on a nonterminal transition loses the first turn's persisted
// state; notifying before attachment exposes the old state's damage instead.
func (s *observationsSuite) TestNonterminalTransitionPublishesChangedAfterAttachment() {
	ov := outputVocabulary()
	var rg *rig
	var states []string
	var damageAtChange []int
	ov.Changed = func(_ context.Context, _ events.EventBus, _ effects.Host) error {
		states = append(states, rg.c.State())
		damageAtChange = append(damageAtChange, rg.swing())
		return nil
	}
	rg = newRig(&s.Suite, freshless(), ov)
	s.Empty(states, "attachment is not a gameplay state change")
	rg.turnEnd(1)
	s.Equal([]string{stRaging}, states)
	s.Equal([]int{13}, damageAtChange, "the new state's bonus is attached before notification")
	rg.turnEnd(2)
	s.Equal(stEnded, rg.c.State())
	s.Equal([]string{stRaging}, states, "terminal removal does not also publish Changed")
	s.Equal(10, rg.swing())
}

func (s *observationsSuite) TestNonterminalChangeErrorReturnsToThePublisher() {
	ov := outputVocabulary()
	refused := errors.New("host refused changed state")
	ov.Changed = func(context.Context, events.EventBus, effects.Host) error { return refused }
	rg := newRig(&s.Suite, freshless(), ov)
	err := turnEndTopic.On(rg.bus).Publish(rg.ctx, turnEnd{SubjectID: holder, Round: 1})
	s.ErrorIs(err, refused, "the host must be able to abort its failed working state")
	s.Equal(stRaging, rg.c.State(), "notification failure is not runtime rollback")
	s.Equal(13, rg.swing())
}

func (s *observationsSuite) TestFailedNewOutputAttachmentDoesNotPublishChanged() {
	ov := outputVocabulary()
	refused := errors.New("refuse new state's output")
	kind := ov.Kinds[kindDamageBonus]
	kind.Attach = func(context.Context, events.EventBus, *effects.Bound) (effects.Detach, error) {
		return nil, refused
	}
	ov.Kinds[kindDamageBonus] = kind
	changed := false
	ov.Changed = func(context.Context, events.EventBus, effects.Host) error { changed = true; return nil }
	rg := newRig(&s.Suite, freshless(), ov)
	err := turnEndTopic.On(rg.bus).Publish(rg.ctx, turnEnd{SubjectID: holder, Round: 1})
	s.ErrorIs(err, refused)
	s.False(changed, "a failed output transition is not a successful state notification")
}

// Returning the live bag either changes an older snapshot when a beat arrives,
// or lets a reader sustain a condition by mutating what it read.
func (s *observationsSuite) TestSnapshotDoesNotMutateOrFollowLiveState() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	before := rg.c.Snapshot()
	rg.turnEnd(1)
	s.Equal(stFresh, before.State)
	s.Equal(0, before.Bag[machine.KeyTurnsActive])
	s.Equal(false, before.Bag[machine.KeySawTurnEnd])
	current := rg.c.Snapshot()
	s.Equal(stRaging, current.State)
	s.Equal(1, current.Bag[machine.KeyTurnsActive])
	current.State = stEnded
	current.Bag[keyDidAttack] = true
	current.Bag[machine.KeyTurnsActive] = 999
	delete(current.Bag, keyWasHit)
	rg.turnEnd(2)
	s.Equal(stEnded, rg.c.State(), "editing a snapshot cannot sustain the real Rage")
	s.Equal(10, rg.swing())
}

// Borrowed backing slices/params/refs would let an inspection poison a later
// application, even though reading is not a mutation operation.
func (s *observationsSuite) TestOutputObservationsCannotChangeRuntime() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	outs := rg.c.CurrentOutputs()
	s.Require().NotEmpty(outs)
	first := outs[0]
	s.Equal(kindDamageBonus, first.Kind())
	params := first.Params()
	s.Require().NotEmpty(params)
	params[0] = '!'
	host := first.Host()
	host.Ref.ID = "corrupted-condition"
	host.Source.ID = "corrupted-source"
	outs[0] = nil
	next := rg.c.CurrentOutputs()
	s.Require().NotNil(next[0])
	s.Equal("raging", next[0].Host().Ref.ID)
	s.Equal("rage", next[0].Host().Source.ID)
	s.Require().NoError(rg.c.Remove(rg.ctx, rg.bus))
	s.Require().NoError(rg.c.Apply(rg.ctx, rg.bus), "inspection must not corrupt the next attachment")
	s.Equal(13, rg.swing())
	rg.turnEnd(1)
	s.Equal(kindDamageBonus, rg.c.CurrentOutputs()[0].Kind())
	s.Require().NoError(restTopic.On(rg.bus).Publish(rg.ctx, rest{CharacterID: holder}))
	s.Empty(rg.c.CurrentOutputs(), "ended has no current outputs")
}

func (s *observationsSuite) TestFullOutputValidationRefusesBeforeAttachment() {
	ov := outputVocabulary()
	kind := ov.Kinds[kindDamageBonus]
	kind.Check = func(out effects.Output) error {
		if out.Kind != kindDamageBonus || out.When == nil || len(out.Params) == 0 {
			return errors.New("full output context is required")
		}
		if out.Amount == nil {
			return errors.New("damage amount is required")
		}
		return nil
	}
	ov.Kinds[kindDamageBonus] = kind
	r := rageRecipe()
	good, err := effects.Compose(effects.ComposeInput{
		Recipe: r, MachineVocabulary: rageVocabulary(), OutputVocabulary: ov, HolderID: holder,
	})
	s.Require().NoError(err)
	s.False(good.IsApplied())
	st := r.States[stFresh]
	var out effects.Output
	s.Require().NoError(json.Unmarshal(st.Outputs[0], &out))
	out.Amount = nil
	st.Outputs[0] = output(out)
	r.States[stFresh] = st
	refused, err := effects.Compose(effects.ComposeInput{
		Recipe: r, MachineVocabulary: rageVocabulary(), OutputVocabulary: ov, HolderID: holder,
	})
	s.ErrorIs(err, effects.ErrInvalidOutput)
	s.ErrorContains(err, "damage amount is required")
	s.Nil(refused)
}

func TestCompositionObservations(t *testing.T) { suite.Run(t, new(observationsSuite)) }
