// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

func (s *assemblySuite) TestQueryContributionsRemainVisibleThroughoutEnd() {
	terminating := counterPart("terminating", 1, effects.EndApplication)
	query := queryInput("")
	state := terminating.Compose.Recipe.States["active"]
	state.Outputs = query.Recipe.States["active"].Outputs
	terminating.Compose.Recipe.States["active"] = state
	terminating.Compose.OutputVocabulary = query.OutputVocabulary
	query.HolderID = holder
	var assembly *effects.Assembly
	var during []string
	var err error
	assembly, err = effects.Assemble(effects.AssemblyInput{Key: "owner", Parts: []effects.PartInput{
		terminating, {Key: "sibling", Compose: query, OnTerminal: effects.EndPart},
	}, Lifecycle: effects.AssemblyLifecycle{End: func(context.Context, events.EventBus, effects.AssemblyEnd) error {
		for _, out := range assembly.CurrentOutputs() {
			description, err := out.Output.Amount(nil)
			if err != nil {
				return err
			}
			during = append(during, out.PartKey+":"+description.Dice)
		}
		return nil
	}}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().Len(assembly.CurrentOutputs(), 2)
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Equal([]string{"terminating:1d4", "sibling:1d4"}, during,
		"both pre-terminal and still-live query contributions answer until end reporting finishes")
	s.Empty(assembly.CurrentOutputs())
	s.False(assembly.IsApplied())
}

func (s *assemblySuite) TestReentrantAndQueuedSiblingReactionsFreezeDuringEnd() {
	for _, tc := range []struct {
		name      string
		remaining int
	}{
		{"terminal sibling", 1},
		{"staying sibling", 3},
	} {
		s.Run(tc.name, func() {
			sibling := counterPart("sibling", tc.remaining, effects.EndPart)
			sibling.Compose.Recipe.Bag["seen"] = false
			sibling.Compose.Recipe.Trackers = []machine.Tracker{{On: beatTurnEnd, About: selSubject, Set: "seen"}}
			var assembly *effects.Assembly
			var during []int
			var inside effects.AssemblySnapshot
			var err error
			assembly, err = effects.Assemble(effects.AssemblyInput{Key: "owner", Parts: []effects.PartInput{
				counterPart("ending", 1, effects.EndApplication), sibling,
			}, Lifecycle: effects.AssemblyLifecycle{End: func(ctx context.Context, bus events.EventBus, _ effects.AssemblyEnd) error {
				if err := turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}); err != nil {
					return err
				}
				inside = assembly.Snapshot()
				got, err := damage(ctx, bus, testDamage{AttackerID: holder, Frame: strMelee(9), Total: 10})
				during = append(during, got)
				return err
			}}})
			s.Require().NoError(err)
			ctx, bus := context.Background(), events.NewEventBus()
			s.Require().NoError(assembly.Apply(ctx, bus))
			s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
			s.Equal([]int{16}, during, "reentrant transport cannot detach a sibling mid-report")
			for _, snapshot := range []effects.AssemblySnapshot{inside, assembly.Snapshot()} {
				s.Equal("active", snapshot.Parts["sibling"].State)
				s.Equal(tc.remaining, snapshot.Parts["sibling"].Bag["remaining"])
				s.Equal(false, snapshot.Parts["sibling"].Bag["seen"], "trackers and transitions are both gated")
			}
			s.False(assembly.IsApplied())
			got, err := damage(ctx, bus, testDamage{AttackerID: holder, Frame: strMelee(9), Total: 10})
			s.Require().NoError(err)
			s.Equal(10, got)
		})
	}
}
