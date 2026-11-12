// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

func (s *assemblySuite) TestEndingOneAssemblyDoesNotGateSharedVocabularyUsers() {
	shared := rageVocabulary()
	first := counterPart("first", 1, effects.EndApplication)
	second := counterPart("second", 3, effects.EndPart)
	first.Compose.MachineVocabulary = shared
	second.Compose.MachineVocabulary = shared
	a, err := effects.Assemble(effects.AssemblyInput{Key: "a", Parts: []effects.PartInput{first},
		Lifecycle: effects.AssemblyLifecycle{End: func(context.Context, events.EventBus, effects.AssemblyEnd) error { return nil }}})
	s.Require().NoError(err)
	b, err := effects.Assemble(effects.AssemblyInput{Key: "b", Parts: []effects.PartInput{second}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(a.Apply(ctx, bus))
	s.Require().NoError(b.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.False(a.IsApplied())
	s.True(b.IsApplied())
	s.Equal(2, b.Snapshot().Parts["second"].Bag["remaining"])
	s.Require().NoError(b.Remove(ctx, bus))
}
