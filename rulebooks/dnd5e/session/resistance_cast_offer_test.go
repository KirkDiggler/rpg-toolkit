// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// resistance_cast_offer_test.go is Resistance's done-when for Cast
// (docs/ideas/cleric/plan.md): a target's own saving throw, mid-cast, stops
// to ask them whether to spend a Resistance die, and the answer finishes
// the SAME cast. Every scene drives the whole stack — Cast poses, resolution
// freezes the cast and its contest, the ledger persists, Afford offers the
// row, React resumes — [guidance_check_offer_test.go]'s own reason: proving
// any one of them in isolation would pass with the rest wired wrong.

import (
	"context"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// holdResistance puts a Resistance die on the member's stored sheet —
// [CastSuite.holdInspiration]'s pattern for a different condition.
func (s *CastSuite) holdResistance(member, casterID string) {
	s.T().Helper()
	ctx := context.Background()
	stored, err := s.characters.GetCharacter(ctx, member)
	s.Require().NoError(err)
	condition, err := conditions.NewResistanceCondition(conditions.NewResistanceConditionInput{
		MemberID: member, SourceID: casterID, SourceRef: refs.Spells.Resistance(),
	})
	s.Require().NoError(err)
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, raw)
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))
}

// reactRow is every VerbReact declaration the member is currently offered,
// narrowed to the first row — this suite never opens two windows on the
// same member at once.
func (s *CastSuite) reactRow(member string) session.Declaration {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	for _, declaration := range out.Declarations {
		if declaration.Verb == session.VerbReact {
			return declaration
		}
	}
	return session.Declaration{}
}

// TestBaneOnAResistantTargetStopsAndAsks is the pose, end to end.
func (s *CastSuite) TestBaneOnAResistantTargetStopsAndAsks() {
	fighter := armedFighter("fighter")
	s.sceneWithAllies(castingBardWithSpells("bard", spells.Bane), []*character.Data{fighter}, 4, 10)
	s.holdResistance("fighter", "cleric-1")

	out, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Bane).ID,
		Targets: []string{"fighter"},
	})
	s.Require().NoError(err)

	s.True(out.Posed, "fighter holds a Resistance die against Bane's own save")
	s.Require().NotNil(out.Roll)
	s.Equal(10, *out.Roll)
	s.Require().NotNil(out.Total)
	s.Equal(10-1, *out.Total, "10 rolled, CHA -1, no proficiency")
	s.Equal(refs.Spells.Bane().String(), out.Spell.Ref)
	s.Empty(out.Saved, "no verdict yet")

	row := s.reactRow("fighter")
	s.Require().NotEmpty(row.ID)
	s.True(row.Available)
	s.Require().NotNil(row.Reaction)
	s.Equal(conditions.ResistanceName, row.Reaction.Name)
	s.Equal(refs.Conditions.Resistance().String(), row.Reaction.Ref)
	s.Equal(session.TargetNone, row.TargetKind)
	s.Equal(session.SlotNone, row.Slot, "answering costs no reaction")

	stored := s.characters.byID["bard"]
	s.Equal(1, stored.Resources[resources.SpellSlotLevel1].Current,
		"the price is paid before the door yields its first step")
}

// TestAnUnresistedBaneIsUnchanged is the regression that matters most: the
// save-offer chain now folds on every save-gated cast, and a target holding
// nothing must resolve in one call exactly as it always did.
func (s *CastSuite) TestAnUnresistedBaneIsUnchanged() {
	fighter := armedFighter("fighter")
	s.sceneWithAllies(castingBardWithSpells("bard", spells.Bane), []*character.Data{fighter}, 4, 10)

	out, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Bane).ID,
		Targets: []string{"fighter"},
	})

	s.Require().NoError(err)
	s.False(out.Posed, "nobody offered anything, so nothing was asked")
	s.Require().NotNil(out.Saved)
	s.Empty(s.reactRow("fighter").ID, "and no window is open")
}

// TestSpendingFinishesTheCastWithTheDieOnIt is the walk's spend branch: the
// die joins fighter's save, and the cast finishes on the far side of it.
func (s *CastSuite) TestSpendingFinishesTheCastWithTheDieOnIt() {
	fighter := armedFighter("fighter")
	s.sceneWithAllies(castingBardWithSpells("bard", spells.Bane), []*character.Data{fighter}, 4, 10, 4)
	s.holdResistance("fighter", "cleric-1")

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Bane).ID,
		Targets: []string{"fighter"},
	})
	s.Require().NoError(err)

	row := s.reactRow("fighter")
	s.Require().NotEmpty(row.ID)
	_, err = s.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter", DeclarationID: row.ID, Choice: session.ReactStrike,
	})
	s.Require().NoError(err)

	saves := s.beats(session.EventSaved)
	s.Require().Len(saves, 1)
	saved, ok := saves[0].Body.(session.SavedBody)
	s.Require().True(ok)
	s.Equal("fighter", saved.Saver)
	s.Equal(9+4, saved.Total, "10 - 1 CHA plus the offered d4's own face")
	s.True(saved.Succeeded, "13 beats Bane's own DC of 13")

	s.Empty(s.reactRow("fighter").ID, "the window is closed")

	stored, err := s.characters.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	for _, raw := range stored.Conditions {
		s.NotContains(string(raw), refs.Conditions.Resistance().ID, "the die is spent when it is TAKEN")
	}
}

// TestAKeptOfferOnOneTargetLeavesTheNextTargetToRun is the two-`Request`-
// layer case: fighterA holds a die and poses, fighterA's answer resumes the
// SAME cast, and fighterB — the next target, also holding a die — gets to
// roll and pose in turn rather than being silently skipped.
func (s *CastSuite) TestAKeptOfferOnOneTargetLeavesTheNextTargetToRun() {
	fighterA := armedFighter("fighter-a")
	fighterB := armedFighter("fighter-b")
	s.sceneWithAllies(castingBardWithSpells("bard", spells.Bane),
		[]*character.Data{fighterA, fighterB}, 4, 10, 12, 3)
	s.holdResistance("fighter-a", "cleric-1")
	s.holdResistance("fighter-b", "cleric-1")

	out, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Bane).ID,
		Targets: []string{"fighter-a", "fighter-b"},
	})
	s.Require().NoError(err)
	s.True(out.Posed, "fighter-a holds the die")
	s.Empty(s.beats(session.EventSaved), "no verdict for anybody yet")

	rowA := s.reactRow("fighter-a")
	s.Require().NotEmpty(rowA.ID)
	_, err = s.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter-a", DeclarationID: rowA.ID, Choice: session.ReactHold,
	})
	s.Require().NoError(err)

	// fighter-a's window is answered, and it did NOT finish the cast:
	// fighter-b's own save just ran and it ALSO holds a die.
	s.Empty(s.reactRow("fighter-a").ID, "fighter-a's window is closed")
	rowB := s.reactRow("fighter-b")
	s.Require().NotEmpty(rowB.ID, "fighter-b's own save posed in turn")
	s.Empty(s.beats(session.EventSaved), "still no verdict — the cast has not finished")

	_, err = s.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter-b", DeclarationID: rowB.ID, Choice: session.ReactStrike,
	})
	s.Require().NoError(err)

	saves := s.beats(session.EventSaved)
	s.Require().Len(saves, 2, "both targets' saves are now on the story, in caller order")
	first, ok := saves[0].Body.(session.SavedBody)
	s.Require().True(ok)
	s.Equal("fighter-a", first.Saver)
	s.Equal(9, first.Total, "fighter-a kept the die")
	second, ok := saves[1].Body.(session.SavedBody)
	s.Require().True(ok)
	s.Equal("fighter-b", second.Saver)
	s.Equal(11+3, second.Total, "fighter-b spent it: 12 - 1 CHA plus the offered d4")

	s.Empty(s.reactRow("fighter-b").ID, "fighter-b's window is closed too")
}

// TestKeepingFinishesTheCastWithoutIt is the other branch: the save stays
// whatever it already was, and the die is still in hand afterwards.
func (s *CastSuite) TestKeepingFinishesTheCastWithoutIt() {
	fighter := armedFighter("fighter")
	s.sceneWithAllies(castingBardWithSpells("bard", spells.Bane), []*character.Data{fighter}, 4, 10)
	s.holdResistance("fighter", "cleric-1")

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Bane).ID,
		Targets: []string{"fighter"},
	})
	s.Require().NoError(err)

	row := s.reactRow("fighter")
	_, err = s.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter", DeclarationID: row.ID, Choice: session.ReactHold,
	})
	s.Require().NoError(err)

	saves := s.beats(session.EventSaved)
	s.Require().Len(saves, 1)
	saved, ok := saves[0].Body.(session.SavedBody)
	s.Require().True(ok)
	s.Equal(9, saved.Total, "no face joined it")
	s.False(saved.Succeeded, "9 misses Bane's own DC of 13")

	stored, err := s.characters.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	found := false
	for _, raw := range stored.Conditions {
		if strings.Contains(string(raw), refs.Conditions.Resistance().ID) {
			found = true
		}
	}
	s.True(found, "declining costs nothing")
}
