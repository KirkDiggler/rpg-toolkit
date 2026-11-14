// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type roundDurationSuite struct{ suite.Suite }

func TestRoundDurationSuite(t *testing.T) { suite.Run(t, new(roundDurationSuite)) }

type roundRig struct {
	bus         events.EventBus
	assembly    *generic.Assembly
	definition  generic.AssemblyInput
	declaration dndfx.PartDeclaration
	ends        []generic.AssemblyEnd
}

func (s *roundDurationSuite) rig(rounds int) *roundRig {
	compiled, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{{Key: "clock", Effect: dndfx.RoundDuration{Rounds: rounds, ExpiredReason: "duration_expired", ResetReason: "clock_reset"}, OnTerminal: generic.EndApplication}}})
	s.Require().NoError(err)
	in := partInput("clock", dndfx.KindRoundDuration, "")
	in.Declaration = compiled.Parts[0]
	part, err := dndfx.BindPart(in)
	s.Require().NoError(err)
	r := &roundRig{bus: events.NewEventBus(), declaration: compiled.Parts[0]}
	r.definition = generic.AssemblyInput{Key: "clock-instance", Parts: []generic.PartInput{part.Part}, Lifecycle: generic.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end generic.AssemblyEnd) error {
		r.ends = append(r.ends, end)
		return nil
	}}}
	r.assembly, err = generic.Assemble(r.definition)
	s.Require().NoError(err)
	s.Require().NoError(r.assembly.Apply(context.Background(), r.bus))
	s.T().Cleanup(func() { s.NoError(r.assembly.Remove(context.Background(), r.bus)) })
	return r
}
func (s *roundDurationSuite) end(r *roundRig, round int) {
	s.Require().NoError(dndevents.TurnEndTopic.On(r.bus).Publish(context.Background(), dndevents.TurnEndEvent{SubjectID: "maintainer", Round: round}))
}
func (s *roundDurationSuite) info(r *roundRig) *dndfx.RoundDurationInfo {
	info, err := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: r.declaration, Snapshot: r.assembly.Snapshot().Parts["clock"]})
	s.Require().NoError(err)
	s.Require().NotNil(info.RoundDuration)
	return info.RoundDuration
}
func (s *roundDurationSuite) TestSparseRoundsExpireWithoutCountingEvents() {
	r := s.rig(10)
	for _, round := range []int{1, 2, 3, 4, 8, 9} {
		s.end(r, round)
		s.Empty(r.ends)
	}
	s.end(r, 10)
	s.Require().Len(r.ends, 1)
	s.Equal("duration_expired", r.ends[0].Reason)
}
func (s *roundDurationSuite) TestRelativeAnchorAndDerivedProgress() {
	r := s.rig(10)
	for _, tc := range []struct{ round, progress int }{{4, 1}, {5, 2}, {6, 3}, {9, 6}} {
		s.end(r, tc.round)
		s.Empty(r.ends)
		info := s.info(r)
		s.True(info.Anchored)
		s.Equal(4, info.RoundActivated)
		s.Equal(tc.progress, info.TurnsActive)
		s.Equal(10, info.TotalRounds)
	}
	s.end(r, 12)
	s.Empty(r.ends)
	s.end(r, 13)
	s.Require().Len(r.ends, 1)
	s.Equal("duration_expired", r.ends[0].Reason)
}
func (s *roundDurationSuite) TestRoundlessNegativeLateAnchorAndReset() {
	r := s.rig(10)
	s.end(r, -5)
	s.end(r, 0)
	info := s.info(r)
	s.False(info.Anchored)
	s.Zero(info.RoundActivated)
	s.Zero(info.TurnsActive)
	s.end(r, 6)
	info = s.info(r)
	s.Equal(6, info.RoundActivated)
	s.Equal(1, info.TurnsActive)
	s.end(r, 0)
	s.Empty(r.ends)
	s.Equal(6, s.info(r).RoundActivated)
	s.Equal(1, s.info(r).TurnsActive)
	s.end(r, 1)
	s.Require().Len(r.ends, 1)
	s.Equal("clock_reset", r.ends[0].Reason)
}
func (s *roundDurationSuite) TestOneRoundExpiresOnFirstPositiveEnd() {
	r := s.rig(1)
	s.end(r, 0)
	s.Empty(r.ends)
	s.end(r, 4)
	s.Require().Len(r.ends, 1)
	s.Equal("duration_expired", r.ends[0].Reason)
}
func (s *roundDurationSuite) TestRestorePreservesAnchorAndRefusesInvalidState() {
	r := s.rig(10)
	s.end(r, 4)
	s.end(r, 9)
	snapshot := r.assembly.Snapshot()
	s.Require().NoError(r.assembly.Remove(context.Background(), r.bus))
	restored, err := generic.RestoreAssembly(generic.RestoreAssemblyInput{Definition: r.definition, Snapshot: &snapshot})
	s.Require().NoError(err)
	r.assembly = restored
	s.Require().NoError(restored.Apply(context.Background(), r.bus))
	s.Equal(4, s.info(r).RoundActivated)
	s.Equal(6, s.info(r).TurnsActive)
	for _, change := range []func(*generic.Snapshot){
		func(s *generic.Snapshot) { s.Bag = nil },
		func(s *generic.Snapshot) { s.State = "unknown" },
		func(s *generic.Snapshot) { s.Bag[machine.KeyRoundActivated] = -1 },
		func(s *generic.Snapshot) { s.Bag[machine.KeyTurnsActive] = 10 },
		func(s *generic.Snapshot) { s.Bag[machine.KeySawTurnEnd] = false },
		func(s *generic.Snapshot) { s.Bag[machine.KeyTurnsActive] = "bad" },
	} {
		state := r.assembly.Snapshot().Parts["clock"]
		change(&state)
		info, err := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: r.declaration, Snapshot: state})
		s.Error(err)
		s.Nil(info)
	}
	invalidEnded := r.assembly.Snapshot().Parts["clock"]
	invalidEnded.State = "ended"
	invalidEnded.Bag[machine.KeyTurnsActive] = 11
	badInfo, badErr := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: r.declaration, Snapshot: invalidEnded})
	s.Error(badErr, "terminal progress cannot exceed the authored maximum either")
	s.Nil(badInfo)
	s.end(r, 13)
	s.Require().Len(r.ends, 1)
	ended := r.assembly.Snapshot()
	revived, err := generic.RestoreAssembly(generic.RestoreAssemblyInput{Definition: r.definition, Snapshot: &ended})
	s.Error(err)
	s.Nil(revived)
}
func (s *roundDurationSuite) TestInvalidDeclarationRefuses() {
	for _, effect := range []dndfx.RoundDuration{{}, {Rounds: -1, ExpiredReason: "expired", ResetReason: "reset"}, {Rounds: 10, ExpiredReason: "", ResetReason: "reset"}, {Rounds: 10, ExpiredReason: "expired", ResetReason: " "}} {
		out, err := dndfx.Compile(&dndfx.CompileInput{Definition: effect})
		s.Error(err)
		s.Nil(out)
	}
	out, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{{Key: "clock", Effect: dndfx.RoundDuration{Rounds: 10, ExpiredReason: "expired", ResetReason: "reset"}, OnTerminal: generic.EndPart}}})
	s.Error(err)
	s.Nil(out)
}
