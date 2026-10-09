// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Window prose, one real window per kind. Post-hit is
// TestReactWindowKeepsOfferAndOptionProseAcrossReload; these are the other
// five: a pending attack (a reaction during a walk), a post-roll offer, a
// check offer on a door, a cast offer, and a check offer on a social verb.
//
// Each opens its window through the verbs, reads the REACT row, then copies
// every store — the next HTTP request — and reads it again. The offer's prose
// is read from its owner (the condition's constant, the feature's own
// Description), so a wording change in content fails nothing here; a dropped
// copy anywhere between the pose and the row does.

// windowRow is the one REACT row the audience is shown for ref.
func windowRow(t *testing.T, mgr *session.Manager, member, ref string) session.Declaration {
	t.Helper()
	for _, row := range affordRows(t, mgr, member) {
		if row.Verb == session.VerbReact && row.Reaction != nil && row.Reaction.Ref == ref {
			return row
		}
	}
	t.Fatalf("%s was shown no REACT row for %s", member, ref)
	return session.Declaration{}
}

// reloadStores copies every store and builds a fresh manager over the copies,
// which is what a later request does. Afford rolls nothing, so the dice and
// driver are inert.
func reloadStores(t *testing.T, sessions *fakeSessions, encounters *fakeEncounters, characters *fakeCharacters) *session.Manager {
	t.Helper()
	var err error
	for id, data := range sessions.byID {
		sessions.byID[id], err = copyOf(data)
		require.NoError(t, err)
	}
	for id, data := range encounters.byID {
		encounters.byID[id], err = copyOf(data)
		require.NoError(t, err)
	}
	for id, data := range characters.byID {
		characters.byID[id], err = copyOf(data)
		require.NoError(t, err)
	}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: characters, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)
	return mgr
}

// requireWindowProse asserts the row carries exactly the owner's offer prose
// and no facts, and that each option carries prose when the window has
// options.
func requireWindowProse(t *testing.T, row session.Declaration, offer string, wantOptions int) {
	t.Helper()
	require.NotEmpty(t, offer, "precondition: the owner authored offer prose")
	require.Equal(t, &session.ActionInformation{Description: offer}, row.Information)
	require.Len(t, row.Options, wantOptions)
	seen := map[string]bool{}
	for _, option := range row.Options {
		require.NotEmpty(t, option.Description, "the offering owner describes %s", option.ID)
		require.False(t, seen[option.Description], "each option has its own prose")
		seen[option.Description] = true
	}
}

// wrathDescription is the feature's own prose, read off the sheet's feature.
func wrathDescription(t *testing.T, sheet *character.Data) string {
	t.Helper()
	for _, raw := range sheet.Features {
		feature, err := features.LoadJSON(raw)
		if err == nil && feature.Ref().String() == refs.Features.WrathOfTheStorm().String() {
			return feature.Description()
		}
	}
	t.Fatalf("the sheet holds no Wrath of the Storm")
	return ""
}

// TestPendingAttackWindowCarriesProseAcrossReload: a skeleton's swing during
// the cleric's walk asks about Wrath of the Storm on a pending-attack window.
func TestPendingAttackWindowCarriesProseAcrossReload(t *testing.T) {
	s := &CastSuite{}
	s.SetT(t)
	cleric := s.tempestSheet()
	s.sceneWithSecondSkeleton(cleric, 1, 1, 15, 3, 3, 3, 3, 3, 3, 3, 3)
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "cleric",
		DeclarationID: currentMoveID(t, s.mgr, "sess", "cleric"), Path: []spatial.Position{{X: 0, Y: 1}}})
	require.NoError(t, err)
	require.Equal(t, session.MovementPaused, out.Status, "precondition: the walk stopped to ask")

	ref := refs.Features.WrathOfTheStorm().String()
	before := windowRow(t, s.mgr, "cleric", ref)
	requireWindowProse(t, before, wrathDescription(t, cleric), 2)

	s.reloadHealingScene()
	after := windowRow(t, s.mgr, "cleric", ref)
	require.Equal(t, before.ID, after.ID)
	require.Equal(t, before.Information, after.Information)
	require.Equal(t, before.Options, after.Options)
}

// TestPostRollWindowCarriesProseAcrossReload: a swing by an inspired bard
// pauses on the attack roll and offers the Bardic Inspiration die.
func TestPostRollWindowCarriesProseAcrossReload(t *testing.T) {
	s := &CastSuite{}
	s.SetT(t)
	bard := castingBard("bard", spells.TrueStrike, spells.ViciousMockery)
	armForSwinging(bard)
	s.scene(bard, 1, 12)
	s.holdInspiration("bard")
	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "bard", Target: "skeleton",
		DeclarationID: currentAttackID(t, s.mgr, "sess", "bard"),
	})
	require.NoError(t, err)

	ref := refs.Conditions.Inspired().String()
	before := windowRow(t, s.mgr, "bard", ref)
	requireWindowProse(t, before, conditions.InspiredOfferDescription, 0)

	s.reloadHealingScene()
	after := windowRow(t, s.mgr, "bard", ref)
	require.Equal(t, before.ID, after.ID)
	require.Equal(t, before.Information, after.Information)
}

// TestDoorCheckOfferWindowCarriesProseAcrossReload: a guided unlock pauses
// on the check and offers the Guidance die.
func TestDoorCheckOfferWindowCarriesProseAcrossReload(t *testing.T) {
	s := &GuidedUnlockSuite{}
	s.SetT(t)
	s.SetupTest()
	mgr := s.sceneOverStores()
	s.guide()
	require.True(t, s.tryLock(mgr).Paused, "precondition: the unlock stopped to ask")

	ref := refs.Conditions.Guided().String()
	before := windowRow(t, mgr, "alice", ref)
	requireWindowProse(t, before, conditions.GuidedOfferDescription, 0)

	mgr = reloadStores(t, s.sessions, s.encounters, s.characters)
	after := windowRow(t, mgr, "alice", ref)
	require.Equal(t, before.ID, after.ID)
	require.Equal(t, before.Information, after.Information)
}

// TestCastOfferWindowCarriesProseAcrossReload: a spell attack that hits the
// tempest cleric asks about Wrath of the Storm on a cast-offer window, with
// both damage choices described.
func TestCastOfferWindowCarriesProseAcrossReload(t *testing.T) {
	s := &CastSuite{}
	s.SetT(t)
	cleric := s.tempestSheet()
	s.sceneWithAllies(castingBardWithSpells("aaron", spells.InflictWounds), []*character.Data{cleric}, 6, 15, 1, 1, 1, 1, 4, 5)
	out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "aaron",
		DeclarationID: s.castRow(spells.InflictWounds).ID, Targets: []string{"cleric"}})
	require.NoError(t, err)
	require.True(t, out.Posed, "precondition: the cast stopped to ask")

	ref := refs.Features.WrathOfTheStorm().String()
	before := windowRow(t, s.mgr, "cleric", ref)
	requireWindowProse(t, before, wrathDescription(t, cleric), 2)

	s.reloadHealingScene()
	after := windowRow(t, s.mgr, "cleric", ref)
	require.Equal(t, before.ID, after.ID)
	require.Equal(t, before.Information, after.Information)
	require.Equal(t, before.Options, after.Options)
}

// TestSocialCheckOfferWindowCarriesProseAcrossReload: a guided threat pauses
// on the check and offers the Guidance die.
func TestSocialCheckOfferWindowCarriesProseAcrossReload(t *testing.T) {
	s := &IntimidateSuite{}
	s.SetT(t)
	s.SetupTest()
	mgr := s.aYard([]int{6, 4})
	s.guide()
	out, err := s.threaten(mgr)
	require.NoError(t, err)
	require.True(t, out.Paused, "precondition: the threat stopped to ask")

	ref := refs.Conditions.Guided().String()
	before := windowRow(t, mgr, "alice", ref)
	requireWindowProse(t, before, conditions.GuidedOfferDescription, 0)

	mgr = reloadStores(t, s.sessions, s.encounters, s.characters)
	after := windowRow(t, mgr, "alice", ref)
	require.Equal(t, before.ID, after.ID)
	require.Equal(t, before.Information, after.Information)
}
