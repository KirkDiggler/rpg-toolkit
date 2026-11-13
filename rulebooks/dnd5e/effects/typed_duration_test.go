// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func (s *typedDefinitionsSuite) TestTypedDurationKeepsItsOwnCountdown() {
	for _, skip := range []bool{false, true} {
		compiled, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{{Key: "duration", Effect: dndfx.Duration{TurnEnds: 5, SkipFirst: skip}, OnTerminal: generic.EndApplication}}})
		s.Require().NoError(err)
		in := partInput("duration", dndfx.KindDuration, "")
		in.Declaration = compiled.Parts[0]
		bound, err := dndfx.BindPart(in)
		s.Require().NoError(err)
		assembly, err := generic.Assemble(generic.AssemblyInput{Key: "duration", Parts: []generic.PartInput{bound.Part}, Lifecycle: generic.AssemblyLifecycle{End: func(context.Context, events.EventBus, generic.AssemblyEnd) error { return nil }}})
		s.Require().NoError(err)
		ctx, bus := context.Background(), events.NewEventBus()
		s.Require().NoError(assembly.Apply(ctx, bus))
		s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: in.HolderID}))
		info, err := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: compiled.Parts[0], Snapshot: assembly.Snapshot().Parts["duration"]})
		s.Require().NoError(err)
		s.Require().NotNil(info.Duration)
		want := 4
		if skip {
			want = 5
		}
		s.Equal(want, info.Duration.Remaining)
		s.Require().NoError(assembly.Remove(ctx, bus))
	}
	out, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{{Key: "duration", Effect: dndfx.Duration{}, OnTerminal: generic.EndApplication}}})
	s.Error(err)
	s.Nil(out, "zero duration is not an implicit infinite lifetime")
}
