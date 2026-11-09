// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

func (s *assemblySuite) TestAttachmentEndStopsOnlyItsLocalParts() {
	var ended []effects.AssemblyEnd
	a, err := effects.Assemble(effects.AssemblyInput{Key: "a", Parts: []effects.PartInput{
		counterPart("rest", 1, effects.EndAttachment), counterPart("benefit", 3, effects.EndPart),
	}, Lifecycle: effects.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end effects.AssemblyEnd) error {
		ended = append(ended, end)
		return nil
	}}})
	s.Require().NoError(err)
	b, err := effects.Assemble(effects.AssemblyInput{Key: "b", Parts: []effects.PartInput{counterPart("independent", 3, effects.EndPart)}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(a.Apply(ctx, bus))
	s.Require().NoError(b.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Require().Len(ended, 1)
	s.Equal(effects.EndAttachment, ended[0].Scope)
	s.False(a.IsApplied())
	s.True(b.IsApplied())
	got, err := damage(ctx, bus, testDamage{AttackerID: holder, Frame: strMelee(9), Total: 10})
	s.Require().NoError(err)
	s.Equal(13, got)
	s.Require().NoError(b.Remove(ctx, bus))
}
