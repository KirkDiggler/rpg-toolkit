// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

func (s *assemblySuite) TestRefusesAmbiguousInstanceOrTerminalApplicationPart() {
	part := counterPart("part", 2, effects.EndPart)
	part.Compose.InstanceKey = "caller-owned-namespace"
	_, err := effects.Assemble(effects.AssemblyInput{Key: "assembly-owned-namespace", Parts: []effects.PartInput{part}})
	s.Error(err, "construction must not silently discard a second identity authority")

	part = counterPart("part", 2, effects.EndApplication)
	part.Compose.State = "ended"
	_, err = effects.Assemble(effects.AssemblyInput{Key: "scope", Parts: []effects.PartInput{part},
		Lifecycle: effects.AssemblyLifecycle{End: func(context.Context, events.EventBus, effects.AssemblyEnd) error { return nil }}})
	s.Error(err, "a fresh constructor must not admit already-ended required upkeep either")
}

func (s *assemblySuite) TestLocalCompletionNotifiesSavedState() {
	var changes []effects.AssemblyChange
	assembly, err := effects.Assemble(effects.AssemblyInput{Key: "owner", Parts: []effects.PartInput{counterPart("consumed", 1, effects.EndPart)},
		Lifecycle: effects.AssemblyLifecycle{Changed: func(_ context.Context, _ events.EventBus, change effects.AssemblyChange) error {
			changes = append(changes, change)
			return nil
		}}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Equal([]effects.AssemblyChange{{AssemblyKey: "owner", PartKey: "consumed"}}, changes)
	s.Equal("ended", assembly.Snapshot().Parts["consumed"].State)
	s.Require().NoError(assembly.Remove(ctx, bus))
}
