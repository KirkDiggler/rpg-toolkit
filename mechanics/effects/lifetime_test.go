// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

type lifetimeSuite struct{ suite.Suite }

func TestLifetimeSuite(t *testing.T) { suite.Run(t, new(lifetimeSuite)) }

func lifetimeDefinition() effects.LifetimeInput {
	return effects.LifetimeInput{
		ID: "application-1", Members: []string{"a", "b", "c"},
		Enders: []string{"duration", "concentration"},
	}
}

func (s *lifetimeSuite) deliverAll(lifetime *effects.Lifetime) {
	for _, key := range lifetime.Snapshot().PendingMembers {
		_, err := lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: key, Delivered: true})
		s.Require().NoError(err)
	}
}

// Removing idempotence or ordering must produce a second plan or wrong members.
func (s *lifetimeSuite) TestEndIsAuthorizedOrderedAndIdempotent() {
	lifetime, err := effects.NewLifetime(lifetimeDefinition())
	s.Require().NoError(err)
	s.deliverAll(lifetime)
	removed, err := lifetime.MemberRemoved(effects.MemberRemovedInput{Key: "a"})
	s.Require().NoError(err)
	s.Equal(effects.MemberRemovedOutput{Removed: true, Before: 3, After: 2}, removed)
	plan, err := lifetime.RequestEnd(effects.EndRequest{Requestor: "duration", Reason: "expired"})
	s.Require().NoError(err)
	s.True(plan.Accepted)
	s.Equal("expired", plan.Reason)
	s.Equal([]string{"b", "c"}, plan.Members)
	s.Equal(effects.LifetimeEnding, lifetime.Snapshot().Phase)

	again, err := lifetime.RequestEnd(effects.EndRequest{Requestor: "concentration", Reason: "damage"})
	s.Require().NoError(err)
	s.False(again.Accepted)
	s.Equal("expired", again.Reason)
	s.Empty(again.Members, "a re-entrant request must not obtain another removal plan")
	s.Require().NoError(lifetime.CompleteEnd())
	s.Require().NoError(lifetime.CompleteEnd())
	s.Equal(effects.LifetimeEnded, lifetime.Snapshot().Phase)
	again, err = lifetime.RequestEnd(effects.EndRequest{Requestor: "duration", Reason: "later"})
	s.Require().NoError(err)
	s.False(again.Accepted)
	s.Equal("expired", again.Reason)
	s.Empty(again.Members)
}

// A retained input, snapshot, or plan slice would let a caller change removal custody.
func (s *lifetimeSuite) TestSnapshotDoesNotAliasCallerOrRuntime() {
	definition := lifetimeDefinition()
	lifetime, err := effects.NewLifetime(definition)
	s.Require().NoError(err)
	s.deliverAll(lifetime)
	definition.Members[0] = "foreign"
	definition.Enders[0] = "intruder"
	snapshot := lifetime.Snapshot()
	snapshot.LiveMembers[1] = "foreign"
	plan, err := lifetime.RequestEnd(effects.EndRequest{Requestor: "duration", Reason: "expired"})
	s.Require().NoError(err)
	s.Equal([]string{"a", "b", "c"}, plan.Members)
	plan.Members[2] = "foreign"
	s.Equal([]string{"a", "b", "c"}, lifetime.Snapshot().LiveMembers)
}

// Restoring from defaults or accepting unknown member keys would recreate or misroute effects.
func (s *lifetimeSuite) TestRestoreRejectsMissingDuplicateForeignOrNonLiveState() {
	for _, tc := range []struct {
		name     string
		snapshot *effects.LifetimeSnapshot
	}{
		{name: "missing"},
		{name: "missing phase", snapshot: &effects.LifetimeSnapshot{LiveMembers: []string{"a"}}},
		{name: "duplicate", snapshot: &effects.LifetimeSnapshot{Phase: effects.LifetimeLive, PendingMembers: []string{}, LiveMembers: []string{"a", "a"}}},
		{name: "foreign", snapshot: &effects.LifetimeSnapshot{Phase: effects.LifetimeLive, PendingMembers: []string{}, LiveMembers: []string{"other"}}},
		{name: "ending", snapshot: &effects.LifetimeSnapshot{Phase: effects.LifetimeEnding, LiveMembers: []string{"a"}}},
		{name: "ended", snapshot: &effects.LifetimeSnapshot{Phase: effects.LifetimeEnded}},
	} {
		s.Run(tc.name, func() {
			got, err := effects.RestoreLifetime(effects.RestoreLifetimeInput{Definition: lifetimeDefinition(), Snapshot: tc.snapshot})
			s.Error(err)
			s.Nil(got)
		})
	}
	snapshot := effects.LifetimeSnapshot{Phase: effects.LifetimeLive, PendingMembers: []string{}, LiveMembers: []string{"c", "b"}}
	lifetime, err := effects.RestoreLifetime(effects.RestoreLifetimeInput{Definition: lifetimeDefinition(), Snapshot: &snapshot})
	s.Require().NoError(err)
	snapshot.LiveMembers[0] = "foreign"
	plan, err := lifetime.RequestEnd(effects.EndRequest{Requestor: "duration", Reason: "expired"})
	s.Require().NoError(err)
	s.Equal([]string{"b", "c"}, plan.Members, "declared binding order, not caller snapshot order")
}

// Refusals must leave a valid future end intact, rather than partly spending membership.
func (s *lifetimeSuite) TestMembershipAndCompletionRefusals() {
	lifetime, err := effects.NewLifetime(lifetimeDefinition())
	s.Require().NoError(err)
	s.deliverAll(lifetime)
	s.Error(lifetime.CompleteEnd())
	_, err = lifetime.MemberRemoved(effects.MemberRemovedInput{Key: "foreign"})
	s.Error(err)
	for _, request := range []effects.EndRequest{
		{Requestor: "intruder", Reason: "wrong"},
		{Reason: "missing requester"},
		{Requestor: "duration"},
		{Requestor: "duration", Reason: "   "},
	} {
		out, refusal := lifetime.RequestEnd(request)
		s.Error(refusal)
		s.False(out.Accepted)
		s.Empty(out.Members)
	}
	s.Equal(effects.LifetimeLive, lifetime.Snapshot().Phase)
	removed, err := lifetime.MemberRemoved(effects.MemberRemovedInput{Key: "b"})
	s.Require().NoError(err)
	s.True(removed.Removed)
	again, err := lifetime.MemberRemoved(effects.MemberRemovedInput{Key: "b"})
	s.Require().NoError(err)
	s.Equal(effects.MemberRemovedOutput{Before: 2, After: 2}, again)
	plan, err := lifetime.RequestEnd(effects.EndRequest{Requestor: "duration", Reason: "expired"})
	s.Require().NoError(err)
	s.Equal([]string{"a", "c"}, plan.Members)
	_, err = lifetime.MemberRemoved(effects.MemberRemovedInput{Key: "a"})
	s.Error(err)
	s.Equal([]string{"a", "c"}, lifetime.Snapshot().LiveMembers)
}

func (s *lifetimeSuite) TestInvalidDefinitionsAreRefused() {
	for _, definition := range []effects.LifetimeInput{
		{},
		{ID: " "},
		{ID: "scope", Members: []string{""}},
		{ID: "scope", Members: []string{"a", "a"}},
		{ID: "scope", Enders: []string{" "}},
		{ID: "scope", Enders: []string{"end", "end"}},
	} {
		got, err := effects.NewLifetime(definition)
		s.Error(err)
		s.Nil(got)
	}
}

// Empty creation/restoration must not silently revive members or manufacture a loss.
func (s *lifetimeSuite) TestEmptyLifetimeAndEmptyRestoreRemainLive() {
	lifetime, err := effects.NewLifetime(effects.LifetimeInput{ID: "area", Enders: []string{"duration"}})
	s.Require().NoError(err)
	s.Equal(effects.LifetimeLive, lifetime.Snapshot().Phase)
	s.Empty(lifetime.Snapshot().LiveMembers)
	empty := effects.LifetimeSnapshot{Phase: effects.LifetimeLive, PendingMembers: []string{}, LiveMembers: []string{}}
	restored, err := effects.RestoreLifetime(effects.RestoreLifetimeInput{Definition: lifetimeDefinition(), Snapshot: &empty})
	s.Require().NoError(err)
	plan, err := restored.RequestEnd(effects.EndRequest{Requestor: "duration", Reason: "expired"})
	s.Require().NoError(err)
	s.True(plan.Accepted)
	s.Empty(plan.Members)
}
