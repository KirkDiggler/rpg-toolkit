// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// A rulebook output that requests new work (such as a follow-up save) needs a
// different answer from a contribution that must still answer during end.
func (s *assemblySuite) TestEndStopsNewWorkWhileContributionsStillAnswer() {
	var assembly *effects.Assembly
	var during []bool
	var err error
	assembly, err = effects.Assemble(effects.AssemblyInput{Key: "scope", Parts: []effects.PartInput{
		counterPart("ending", 1, effects.EndApplication), counterPart("sibling", 3, effects.EndPart),
	}, Lifecycle: effects.AssemblyLifecycle{End: func(context.Context, events.EventBus, effects.AssemblyEnd) error {
		for _, output := range assembly.CurrentOutputs() {
			during = append(during, output.Output.CanReact())
		}
		return nil
	}}})
	s.Require().NoError(err)
	outputs := assembly.CurrentOutputs()
	s.Require().Len(outputs, 2)
	s.False(outputs[0].Output.CanReact(), "unattached information is not permission to request work")
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.True(outputs[0].Output.CanReact())
	s.True(outputs[1].Output.CanReact())
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Equal([]bool{false, false}, during)
	s.False(outputs[0].Output.CanReact())
	s.False(outputs[1].Output.CanReact())
}
