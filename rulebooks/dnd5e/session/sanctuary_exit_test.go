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

// TestAWardOutlivesTheClericWhoLeft is the probe that found the stall, kept.
//
// The ward used to read its DC off the caster's sheet, so a cleric who Exited
// left a Sanctuary nobody could read: every swing at its holder refused, a
// driven monster turn is one of those swings, and EndTurn wedged the table
// under ErrBadCharacter. The ward now records the caster's spell save DC when
// it is cast (rpg-toolkit#1967/#1968), so the goblin-side save rolls at that
// recorded number whether or not the cleric is still in the room — which is
// also RAW: Sanctuary does not end when its caster walks away.
//
// Cast through the verb rather than seeded, so the DC on the ward is the one
// resolution wrote at impose, not one this test chose.
func (s *CastSuite) TestAWardOutlivesTheClericWhoLeft() {
	cleric := castingCleric()
	cleric.KnownSpells = []string{refs.Spells.Sanctuary().String()}
	cleric.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{
		resources.SpellSlotLevel1: {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
	}
	// The skeleton stands beside alice. Its one Wisdom save is the only die the
	// scene rolls after initiative: a 5 fails the cleric's DC 13 (8 + 2 + WIS 3).
	s.sceneWithAllies(cleric, []*character.Data{armedFighter("warded")}, 2, 5)
	ctx := context.Background()

	_, err := s.mgr.Cast(ctx, &session.CastInput{
		Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.Sanctuary).ID,
		Targets: []string{"warded"},
	})
	s.Require().NoError(err)

	_, err = s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err, "the cleric leaves the session")

	// From here the skeleton swings at the warded fighter.
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
	s.Require().NoError(err, "the skeleton's driven swing resolves against the ward; the table moves")

	// Read as the warded fighter: the cleric is gone and is delivered nothing.
	warded := ofKinds(eventsFor(s.stream.published, "warded"), session.EventWarded)
	s.Require().NotEmpty(warded, "the skeleton's swing met the ward")
	body, ok := warded[0].Body.(session.WardedBody)
	s.Require().True(ok)
	s.Equal("skeleton", body.Attacker)
	s.Equal("warded", body.Target)
	s.Equal("cleric", body.Source, "the ward still names the cleric who cast it")
	s.Equal(13, body.DC, "the save rolled at the DC the ward recorded when it was cast")
	s.Equal(5, body.Roll)

	turn, err = s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "warded"})
	s.Require().NoError(err)
	s.Equal("warded", turn.Active, "the fight proceeds: the round came back around")
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
