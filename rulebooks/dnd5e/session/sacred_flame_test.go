// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// Creation is covered in the character module. This sheet deliberately has
// exhausted slots and different Wisdom/Charisma modifiers to expose wrong costs
// or use of the bard's casting ability in the shared cast path.
func castingCleric() *character.Data {
	return &character.Data{
		ID: "cleric", PlayerID: "player-cleric", Name: "Cleric", Level: 1,
		ClassID: classes.Cleric, RaceID: "human",
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 10, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 16, abilities.CHA: 8,
		},
		HitPoints: 10, MaxHitPoints: 10, ProficiencyBonus: 2,
		KnownCantrips: []string{
			refs.Spells.SacredFlame().String(), refs.Spells.Guidance().String(), refs.Spells.Light().String(),
		},
	}
}

func (s *CastSuite) TestSacredFlameSaveDamageAndReload() {
	for _, tc := range []struct {
		name   string
		roll   int
		saved  bool
		damage int
	}{
		{name: "failed Dexterity save", roll: 10, damage: 6},
		{name: "save equals DC", roll: 11, saved: true},
	} {
		s.Run(tc.name, func() {
			rolls := []int{tc.roll}
			if !tc.saved {
				rolls = append(rolls, tc.damage)
			}
			s.scene(castingCleric(), 2, rolls...)
			offered := s.offeredCantrips()
			s.NotContains(offered, refs.Spells.Light().String(),
				"an unsupported known cantrip stays on the sheet, not the cast panel")
			s.Contains(offered, refs.Spells.Guidance().String(),
				"Guidance is supported content now (rpg-toolkit#1757) and reaches the panel")
			firstRow := s.castRow(spells.SacredFlame)
			s.Require().True(firstRow.Available, "exhausted slots do not block a cantrip")
			before := s.storedSkeleton()
			mark := lastSeq(storyBeats(s.T(), s.mgr, "sess", s.member))
			out, err := s.cast(spells.SacredFlame)
			s.Require().NoError(err)
			s.Require().NotNil(out.Saved)
			s.Equal(13, out.Saved.DC, "8 + proficiency 2 + Wisdom 3")
			s.Equal(tc.saved, out.Saved.Succeeded)
			told := storyBeatsAfter(s.T(), s.mgr, "sess", s.member, mark)
			events := beatsNamed(told, encounter.BeatCast, encounter.BeatSaved, encounter.BeatActivationResult)
			expectedBeats := []string{encounter.BeatCast, encounter.BeatSaved} // only a failed save adds damage
			if !tc.saved {
				expectedBeats = append(expectedBeats, encounter.BeatActivationResult)
			}
			s.Require().Equal(expectedBeats, beatNames(events))
			var cast flameCastBeat
			events[0].decode(s.T(), &cast)
			s.Equal("cleric", cast.Actor)
			s.Equal(refs.Spells.SacredFlame().String(), cast.Spell.Ref)
			s.Equal([]string{"skeleton"}, cast.Targets)
			var saved flameSavedBeat
			events[1].decode(s.T(), &saved)
			s.Equal(string(abilities.DEX), saved.Ability)
			s.Equal("skeleton", saved.Saver)
			s.Equal(tc.roll, saved.Roll)
			s.Equal(tc.roll+2, saved.Total)
			s.Equal(13, saved.DC)
			s.Equal(tc.saved, saved.Succeeded)
			s.Equal(refs.Spells.SacredFlame().String(), saved.Source.Ref)
			if !tc.saved {
				var result flameResultBeat
				events[2].decode(s.T(), &result)
				s.Equal(encounter.ResultDamageApplied, result.Result.Kind, "damage, not a condition")
				damage := result.Result
				s.Equal(string(session.DamageRadiant), damage.DamageType)
				s.Equal(refs.Spells.SacredFlame().String(), damage.Ref)
				s.Equal("skeleton", damage.Target)
				s.Equal(tc.damage, damage.Amount)
				s.Equal(before, damage.Before)
				s.Equal(before-tc.damage, damage.After)
				s.NotEmpty(damage.Calculation)
			}
			s.Equal(before-tc.damage, s.storedSkeleton())
			s.Equal(2+len(rolls), s.dice.next, "a successful save rolls no damage")

			// Rebuild all repositories from JSON, then make a new manager with no
			// usable randomness. Reads must come from the saved record.
			for id, data := range s.sessions.byID {
				s.sessions.byID[id], err = copyOf(data)
				s.Require().NoError(err)
			}
			for id, data := range s.encounters.byID {
				s.encounters.byID[id], err = copyOf(data)
				s.Require().NoError(err)
			}
			for id, data := range s.characters.byID {
				s.characters.byID[id], err = copyOf(data)
				s.Require().NoError(err)
			}
			s.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
				PresentationIDs: testPresentationIDs{}, Dice: brokenDice{}, TurnDriver: session.Pass{},
				Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
			})
			s.Require().NoError(err)
			s.Equal(told, storyBeatsAfter(s.T(), s.mgr, "sess", s.member, mark),
				"cast, save and damage survive JSON and manager recreation")
			s.Equal(before-tc.damage, s.storedSkeleton())
			s.Equal(castingCleric().KnownCantrips, s.characters.byID[s.member].KnownCantrips)
			row := s.castRow(spells.SacredFlame)
			s.False(row.Available)
			s.Require().NotNil(row.Why)
			s.Equal(session.ShortfallNoBudget, row.Why.Reason)
			s.Equal(session.CurrencyAction, row.Why.Currency)
			s.Equal(0, row.Why.Left)
			s.refuseSacredFlame(firstRow.ID, "skeleton")
		})
	}
}

// flameCastBeat is a cast beat's payload.
type flameCastBeat struct {
	Actor string `json:"actor"`
	Spell struct {
		Ref  string `json:"ref"`
		Name string `json:"name"`
	} `json:"spell"`
	Targets []string `json:"targets"`
}

// flameSavedBeat is a saved beat's payload.
type flameSavedBeat struct {
	Saver     string `json:"saver"`
	Ability   string `json:"ability"`
	Roll      int    `json:"roll"`
	Total     int    `json:"total"`
	DC        int    `json:"dc"`
	Succeeded bool   `json:"succeeded"`
	Source    struct {
		Ref string `json:"ref"`
	} `json:"source"`
}

// flameResultBeat is an activation_result beat's payload: the one result it
// carries.
type flameResultBeat struct {
	Actor  string `json:"actor"`
	Result struct {
		Kind        encounter.ActivationResultKind `json:"kind"`
		Target      string                         `json:"target"`
		Ref         string                         `json:"ref"`
		Name        string                         `json:"name"`
		Amount      int                            `json:"amount"`
		Requested   int                            `json:"requested"`
		Before      int                            `json:"before"`
		After       int                            `json:"after"`
		DamageType  string                         `json:"damage_type"`
		Calculation json.RawMessage                `json:"calculation"`
	} `json:"result"`
}

// offeredCantrips is every spell ref the cast panel is currently offering.
//
// The panel's CONTENTS rather than its length: which cantrips this build
// supports grows as content lands (Guidance arrived in rpg-toolkit#1757), and
// a count says nothing about the claim these tests actually make — that a
// cantrip the sheet knows but this build cannot cast never reaches the panel.
func (s *CastSuite) offeredCantrips() []string {
	s.T().Helper()
	out := make([]string, 0, 2)
	for _, row := range s.castRows() {
		if row.Spell != nil {
			out = append(out, row.Spell.Ref)
		}
	}
	return out
}

// Refusal must not roll, append beats, or write any repository.
func (s *CastSuite) refuseSacredFlame(selector, target string) {
	s.T().Helper()
	before := []int{s.sessions.saves, s.encounters.saves, s.characters.saves, s.dice.next, len(storyBeats(s.T(), s.mgr, "sess", s.member))}
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: s.member, DeclarationID: selector, Target: target,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.Equal(before, []int{s.sessions.saves, s.encounters.saves, s.characters.saves, s.dice.next, len(storyBeats(s.T(), s.mgr, "sess", s.member))})
}

func (s *CastSuite) TestSacredFlameRejectsForgedAndUnownedOffers() {
	s.scene(castingCleric(), 2)
	row := s.castRow(spells.SacredFlame)
	s.refuseSacredFlame("forged", "skeleton")
	s.refuseSacredFlame(row.ID, "nobody")
	s.characters.byID[s.member].KnownCantrips = []string{refs.Spells.Guidance().String(), refs.Spells.Light().String()}
	s.NotContains(s.offeredCantrips(), refs.Spells.SacredFlame().String(),
		"a cantrip the sheet no longer knows is off the panel, and its old row is a forgery")
	s.refuseSacredFlame(row.ID, "skeleton")
}

func (s *CastSuite) TestSacredFlameRangeBoundary() {
	for _, cells := range []int{12, 13} {
		s.scene(castingCleric(), cells)
		row := s.castRow(spells.SacredFlame)
		s.Equal(cells == 12, row.Available)
		candidate, found := castCandidate(s.T(), row, "skeleton")
		s.Require().True(found)
		s.Equal(cells == 12, candidate.Available)
		if cells == 13 {
			s.Require().NotNil(candidate.Why)
			s.Equal(session.ShortfallTargetOutOfReach, candidate.Why.Reason)
			s.refuseSacredFlame(row.ID, "skeleton")
		}
	}
}

// TestACastWhoseTargetLostItsSheetNamesTheSheet: a selector minted while the
// skeleton's stat block was held, echoed after the session stops holding it,
// is refused by the sheet (ErrNoSheet), not called stale — the cast's offer is
// blocked by an unreadable participant, and that participant is what refuses
// (rpg-project#538). Nothing is written and no die is thrown.
func (s *CastSuite) TestACastWhoseTargetLostItsSheetNamesTheSheet() {
	s.scene(castingCleric(), 2)
	row := s.castRow(spells.SacredFlame)
	s.Require().True(row.Available, "precondition: the cast is offered while the sheet is held")

	s.sessions.byID["sess"].NPCs = nil

	before := []int{s.sessions.saves, s.encounters.saves, s.characters.saves, s.dice.next, len(storyBeats(s.T(), s.mgr, "sess", s.member))}
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: s.member, DeclarationID: row.ID, Target: "skeleton",
	})
	s.Require().ErrorIs(err, session.ErrNoSheet, "the sheet refuses the cast, not the selector")
	s.NotErrorIs(err, session.ErrStaleDeclaration)
	s.Equal(before, []int{s.sessions.saves, s.encounters.saves, s.characters.saves, s.dice.next, len(storyBeats(s.T(), s.mgr, "sess", s.member))})
}
