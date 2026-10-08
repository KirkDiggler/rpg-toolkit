// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// TestAWardEndsWithTheClericWhoLeft: in this build Sanctuary is the cleric's
// owned concentration, and a member who leaves a run takes their holds with
// them (rpg-project#542, the #1983 gate ruling): the cleric's Exit ends the
// hold, the ward comes off the fighter it protected, the removal is told on
// the exit beat, and the skeleton's next swing meets no ward — the table
// moves without a sheet nobody can read.
//
// It replaces the probe that pinned the opposite, when a ward outlived its
// cleric by recording the caster's DC (rpg-toolkit#1967/#1968): no hold
// outlives its caster's departure now, so no ward needs to.
func (s *CastSuite) TestAWardEndsWithTheClericWhoLeft() {
	cleric := castingCleric()
	cleric.KnownSpells = []string{refs.Spells.Sanctuary().String()}
	cleric.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{
		resources.SpellSlotLevel1: {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
	}
	// Initiative, then the skeleton's attack roll: a natural 1, so the swing
	// misses and asks for no damage die.
	s.sceneWithAllies(cleric, []*character.Data{armedFighter("warded")}, 2, 1)
	ctx := context.Background()
	_, err := s.mgr.Cast(ctx, &session.CastInput{
		Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.Sanctuary).ID,
		Targets: []string{"warded"},
	})
	s.Require().NoError(err)
	s.stream.published = nil

	_, err = s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err, "the cleric leaves the session")

	exited := ofKinds(eventsFor(s.stream.published, "warded"), session.EventExited)
	s.Require().Len(exited, 1)
	body, ok := exited[0].Body.(session.ExitedBody)
	s.Require().True(ok)
	var offWarded []string
	for _, removed := range body.Ended {
		if removed.Target == "warded" {
			offWarded = append(offWarded, removed.Ref)
		}
	}
	s.NotEmpty(offWarded, "the ward came off the fighter, told on the exit beat")
	stored, err := s.characters.GetCharacter(ctx, "warded")
	s.Require().NoError(err)
	for _, raw := range stored.Conditions {
		for _, ref := range offWarded {
			s.NotContains(string(raw), `"`+ref+`"`, "the stored sheet no longer carries the ward")
		}
	}

	// From here the skeleton swings at the fighter, who is no longer warded.
	s.mgr, err = session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: swingsAt{target: "warded"},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "warded"})
	s.Require().NoError(err)
	s.Require().Equal("warded", turn.Active, "precondition: the warded fighter acts before the skeleton")
	s.stream.published = nil
	_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "warded", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "warded"),
	})
	s.Require().NoError(err, "the skeleton's driven swing resolves; the table moves")
	s.Empty(ofKinds(eventsFor(s.stream.published, "warded"), session.EventWarded), "no ward stood in its way")
	s.NotEmpty(ofKinds(eventsFor(s.stream.published, "warded"), session.EventMissed), "the swing was rolled")
}

// swingsAt declares its first action against one named member, or passes when
// it was told about no action. Named rather than "whoever it sees" because the
// driver's view can still carry a sighting of the member who just left.
type swingsAt struct{ target string }

func (d swingsAt) Act(view session.MonsterView) (session.TurnIntent, error) {
	if len(view.Actions) == 0 {
		return session.Pass{}, nil
	}
	return session.Attack{Target: d.target, Action: view.Actions[0].Ref}, nil
}
