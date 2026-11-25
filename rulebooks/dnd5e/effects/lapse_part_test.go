// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// lapseAssembly assembles one application of a timed status on the holder
// "maintainer" and records the reasons it ended with.
func (s *partsSuite) lapseAssembly(application string, reasons *[]string) *fx.Assembly {
	input := partInput("lapse", dndfx.KindLapse, `{}`)
	input.ApplicationID = application
	assembly, err := fx.Assemble(fx.AssemblyInput{Key: application, Parts: []fx.PartInput{s.bind(input)},
		Lifecycle: fx.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end fx.AssemblyEnd) error {
			*reasons = append(*reasons, application+":"+end.Reason)
			return nil
		}}})
	s.Require().NoError(err)
	return assembly
}

// Rounds of world time pass only when someone acts or waits (a Wait act is its
// own story, rpg-project#1079); the test publishes the lapse by hand, as the
// party that keeps time would.
func (s *partsSuite) TestALapseEndsOnlyItsOwnApplication() {
	var reasons []string
	first, second := s.lapseAssembly("application-1", &reasons), s.lapseAssembly("application-2", &reasons)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(first.Apply(ctx, bus))
	s.Require().NoError(second.Apply(ctx, bus))

	s.Require().NoError(dndevents.StatusLapsedTopic.On(bus).Publish(ctx,
		dndevents.StatusLapsedEvent{SubjectID: "maintainer", ApplicationID: "application-1"}))

	s.Equal([]string{"application-1:rounds_passed"}, reasons)
	s.False(first.IsApplied())
	s.True(second.IsApplied())
}

func (s *partsSuite) TestALapseForAnotherHolderEndsNothing() {
	var reasons []string
	assembly := s.lapseAssembly("application-1", &reasons)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))

	s.Require().NoError(dndevents.StatusLapsedTopic.On(bus).Publish(ctx,
		dndevents.StatusLapsedEvent{SubjectID: "someone-else", ApplicationID: "application-1"}))

	s.Empty(reasons)
	s.True(assembly.IsApplied())
}

func (s *partsSuite) TestALapsePartTakesNoParams() {
	got, err := dndfx.BindPart(partInput("lapse", dndfx.KindLapse, `{"rounds":3}`))
	s.Error(err)
	s.Nil(got)
}
