// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// Treating intended recipients as already delivered would remove nonexistent
// attachments; losing pending state on reload would strand the unanswered entry.
func (s *lifetimeSuite) TestPendingRecipientsRoundTripAndFinalizeIndependently() {
	definition := lifetimeDefinition()
	lifetime, err := effects.NewLifetime(definition)
	s.Require().NoError(err)
	s.Empty(lifetime.Snapshot().LiveMembers)
	s.Equal([]string{"a", "b", "c"}, lifetime.Snapshot().PendingMembers)
	first, err := lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: "a", Delivered: true})
	s.Require().NoError(err)
	s.Equal(effects.FinalizeMemberOutput{Before: 0, After: 1}, first)
	raw, err := json.Marshal(lifetime.Snapshot())
	s.Require().NoError(err)
	var snapshot effects.LifetimeSnapshot
	s.Require().NoError(json.Unmarshal(raw, &snapshot))
	restored, err := effects.RestoreLifetime(effects.RestoreLifetimeInput{Definition: definition, Snapshot: &snapshot})
	s.Require().NoError(err)
	s.Equal([]string{"a"}, restored.Snapshot().LiveMembers)
	s.Equal([]string{"b", "c"}, restored.Snapshot().PendingMembers)
	absent, err := restored.FinalizeMember(effects.FinalizeMemberInput{Key: "b"})
	s.Require().NoError(err)
	s.Equal(effects.FinalizeMemberOutput{Before: 1, After: 1}, absent, "no effect is not a member loss")
	_, err = restored.FinalizeMember(effects.FinalizeMemberInput{Key: "c", Delivered: true})
	s.Require().NoError(err)
	s.Empty(restored.Snapshot().PendingMembers)
	plan, err := restored.RequestEnd(effects.EndRequest{Requestor: "duration", Reason: "expired"})
	s.Require().NoError(err)
	s.Equal([]string{"a", "c"}, plan.Members)
}

func (s *lifetimeSuite) TestFinalizationCannotReopenOrRecruit() {
	lifetime, err := effects.NewLifetime(lifetimeDefinition())
	s.Require().NoError(err)
	_, err = lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: "a", Delivered: true})
	s.Require().NoError(err)
	_, err = lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: "b", Delivered: false})
	s.Require().NoError(err)
	for _, key := range []string{"foreign", "a", "b"} {
		before := lifetime.Snapshot()
		_, err = lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: key, Delivered: true})
		s.Require().ErrorIs(err, effects.ErrInvalidLifetime)
		s.Equal(before, lifetime.Snapshot())
	}
	_, err = lifetime.MemberRemoved(effects.MemberRemovedInput{Key: "a"})
	s.Require().NoError(err)
	before := lifetime.Snapshot()
	_, err = lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: "a", Delivered: true})
	s.Require().ErrorIs(err, effects.ErrInvalidLifetime)
	s.Equal(before, lifetime.Snapshot(), "removing an effect does not make its entry pending again")
}

func (s *lifetimeSuite) TestEndFreezesPendingRecipients() {
	lifetime, err := effects.NewLifetime(lifetimeDefinition())
	s.Require().NoError(err)
	_, err = lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: "a", Delivered: true})
	s.Require().NoError(err)
	plan, err := lifetime.RequestEnd(effects.EndRequest{Requestor: "concentration", Reason: "damage"})
	s.Require().NoError(err)
	s.Equal([]string{"a"}, plan.Members, "pending entries are not active effects to remove")
	before := lifetime.Snapshot()
	_, err = lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: "b", Delivered: true})
	s.Require().ErrorIs(err, effects.ErrInvalidLifetime)
	s.Equal(before, lifetime.Snapshot())
	s.Require().NoError(lifetime.CompleteEnd())
	_, err = lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: "c", Delivered: false})
	s.Require().ErrorIs(err, effects.ErrInvalidLifetime)
}

func (s *lifetimeSuite) TestPendingSnapshotRejectsMissingOverlappingAndForeignKeys() {
	for _, snapshot := range []effects.LifetimeSnapshot{
		{Phase: effects.LifetimeLive, LiveMembers: []string{}},
		{Phase: effects.LifetimeLive, PendingMembers: []string{}},
		{Phase: effects.LifetimeLive, LiveMembers: []string{"a"}, PendingMembers: []string{"a"}},
		{Phase: effects.LifetimeLive, LiveMembers: []string{}, PendingMembers: []string{"a", "a"}},
		{Phase: effects.LifetimeLive, LiveMembers: []string{}, PendingMembers: []string{"foreign"}},
	} {
		_, err := effects.RestoreLifetime(effects.RestoreLifetimeInput{Definition: lifetimeDefinition(), Snapshot: &snapshot})
		s.Require().ErrorIs(err, effects.ErrInvalidLifetime)
	}
	snapshot := effects.LifetimeSnapshot{Phase: effects.LifetimeLive, LiveMembers: []string{"c"}, PendingMembers: []string{"b", "a"}}
	restored, err := effects.RestoreLifetime(effects.RestoreLifetimeInput{Definition: lifetimeDefinition(), Snapshot: &snapshot})
	s.Require().NoError(err)
	snapshot.PendingMembers[0] = "foreign"
	observed := restored.Snapshot()
	observed.PendingMembers[0] = "foreign"
	s.Equal([]string{"a", "b"}, restored.Snapshot().PendingMembers, "pending snapshots own their slices and use definition order")
	_, err = restored.FinalizeMember(effects.FinalizeMemberInput{Key: "a", Delivered: true})
	s.Require().NoError(err)
	plan, err := restored.RequestEnd(effects.EndRequest{Requestor: "duration", Reason: "expired"})
	s.Require().NoError(err)
	s.Equal([]string{"a", "c"}, plan.Members)
}

func (s *lifetimeSuite) TestPendingRemovalDoesNotFinalizeOrInventLoss() {
	lifetime, err := effects.NewLifetime(lifetimeDefinition())
	s.Require().NoError(err)
	removed, err := lifetime.MemberRemoved(effects.MemberRemovedInput{Key: "a"})
	s.Require().NoError(err)
	s.Equal(effects.MemberRemovedOutput{}, removed)
	s.Equal([]string{"a", "b", "c"}, lifetime.Snapshot().PendingMembers)
	_, err = lifetime.FinalizeMember(effects.FinalizeMemberInput{Key: "a", Delivered: true})
	s.Require().NoError(err)
	s.Equal([]string{"a"}, lifetime.Snapshot().LiveMembers)
}
