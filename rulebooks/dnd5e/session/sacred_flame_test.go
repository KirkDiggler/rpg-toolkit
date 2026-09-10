// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
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
		HitPoints: 10, MaxHitPoints: 10, ArmorClass: 10, ProficiencyBonus: 2,
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
			rows := s.castRows()
			s.Require().Len(rows, 1, "unsupported known cantrips stay on the sheet, not the cast panel")
			s.Equal(refs.Spells.SacredFlame().String(), rows[0].Spell.Ref)
			s.Require().True(rows[0].Available, "exhausted slots do not block a cantrip")
			before := s.storedSkeleton()
			out, err := s.cast(spells.SacredFlame)
			s.Require().NoError(err)
			s.Require().NotNil(out.Saved)
			s.Equal(13, out.Saved.DC, "8 + proficiency 2 + Wisdom 3")
			s.Equal(tc.saved, out.Saved.Succeeded)
			events := s.beats(session.EventCast, session.EventSaved, session.EventActivationResult)
			expectedBeats := 2 // cast and save; only a failed save adds damage
			if !tc.saved {
				expectedBeats++
			}
			s.Require().Len(events, expectedBeats)
			s.Len(out.Seqs, expectedBeats)
			s.Equal(session.EventCast, events[0].Kind)
			cast, ok := events[0].Body.(session.CastBody)
			s.Require().True(ok)
			s.Equal("cleric", cast.Actor)
			s.Equal(refs.Spells.SacredFlame().String(), cast.Spell.Ref)
			s.Equal("skeleton", cast.Target)
			saved, ok := events[1].Body.(session.SavedBody)
			s.Require().True(ok)
			s.Equal(string(abilities.DEX), saved.Ability)
			s.Equal("skeleton", saved.Saver)
			s.Equal(tc.roll, saved.Roll)
			s.Equal(tc.roll+2, saved.Total)
			s.Equal(13, saved.DC)
			s.Equal(tc.saved, saved.Succeeded)
			s.Equal(refs.Spells.SacredFlame().String(), saved.Source.Ref)
			if !tc.saved {
				result, ok := events[2].Body.(session.ActivationResultBody)
				s.Require().True(ok)
				s.Nil(result.ConditionApplied)
				s.Require().NotNil(result.DamageApplied)
				damage := result.DamageApplied
				s.Equal(session.DamageRadiant, damage.DamageType)
				s.Equal(refs.Spells.SacredFlame().String(), damage.SourceRef)
				s.Equal("skeleton", damage.Target)
				s.Equal(tc.damage, damage.Amount)
				s.Equal(before, damage.HPBefore)
				s.Equal(before-tc.damage, damage.HPAfter)
				s.Require().NotNil(damage.Calculation)
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
			s.mgr, err = session.NewManager(&session.Config{
				PresentationIDs: testPresentationIDs{}, Dice: brokenDice{}, TurnDriver: session.Pass{},
				Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream,
			})
			s.Require().NoError(err)
			story, err := s.mgr.Story(context.Background(), &session.StoryInput{
				Session: "sess", Member: s.member, FromSeq: events[0].Seq,
			})
			s.Require().NoError(err)
			s.Equal(events, story, "cast, save and damage survive JSON and manager recreation")
			s.Equal(before-tc.damage, s.storedSkeleton())
			s.Equal(castingCleric().KnownCantrips, s.characters.byID[s.member].KnownCantrips)
			row := s.castRow(spells.SacredFlame)
			s.False(row.Available)
			s.Require().NotNil(row.Why)
			s.Equal(session.ShortfallNoBudget, row.Why.Reason)
			s.Equal(session.CurrencyAction, row.Why.Currency)
			s.Equal(0, row.Why.Left)
			s.refuseSacredFlame(rows[0].ID, "skeleton")
		})
	}
}

// Refusal must not roll, append events, or write any repository.
func (s *CastSuite) refuseSacredFlame(selector, target string) {
	s.T().Helper()
	before := []int{s.sessions.saves, s.encounters.saves, s.characters.saves, s.dice.next, len(s.stream.published)}
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: s.member, DeclarationID: selector, Target: target,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.Equal(before, []int{s.sessions.saves, s.encounters.saves, s.characters.saves, s.dice.next, len(s.stream.published)})
}

func (s *CastSuite) TestSacredFlameRejectsForgedAndUnownedOffers() {
	s.scene(castingCleric(), 2)
	row := s.castRow(spells.SacredFlame)
	s.refuseSacredFlame("forged", "skeleton")
	s.refuseSacredFlame(row.ID, "nobody")
	s.characters.byID[s.member].KnownCantrips = []string{refs.Spells.Guidance().String(), refs.Spells.Light().String()}
	s.Empty(s.castRows())
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
