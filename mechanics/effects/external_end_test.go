// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

func (s *assemblySuite) TestExternalOwnerCanFreezeBeforeOrderedRemoval() {
	assembly, err := effects.Assemble(effects.AssemblyInput{Key: "recipient", Parts: []effects.PartInput{counterPart("payload", 3, effects.EndPart)}})
	s.Require().NoError(err)
	s.Error(assembly.BeginEnd(), "a detached assembly is not an active end transport")
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(assembly.BeginEnd())
	s.Require().NoError(assembly.BeginEnd())
	outputs := assembly.CurrentOutputs()
	s.Require().Len(outputs, 1)
	s.False(outputs[0].Output.CanReact())
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Equal(3, assembly.Snapshot().Parts["payload"].Bag["remaining"])
	during, err := damage(ctx, bus, testDamage{AttackerID: holder, Frame: strMelee(9), Total: 10})
	s.Require().NoError(err)
	s.Equal(13, during)
	s.Require().NoError(assembly.Remove(ctx, bus))
	s.Empty(assembly.CurrentOutputs())
	s.Error(assembly.Apply(ctx, bus))
}
