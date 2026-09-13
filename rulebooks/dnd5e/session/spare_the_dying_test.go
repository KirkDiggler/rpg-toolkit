// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func (s *CastSuite) finalizedSpareTheDyingCleric() *character.Data {
	s.T().Helper()
	encoded, err := os.ReadFile("testdata/spare_the_dying_cleric.json")
	s.Require().NoError(err)
	var sheet character.Data
	s.Require().NoError(json.Unmarshal(encoded, &sheet))
	s.Contains(sheet.KnownCantrips, refs.Spells.SpareTheDying().String())
	s.Equal(2, sheet.Resources[resources.SpellSlotLevel1].Current)
	return &sheet
}

func (s *CastSuite) TestSpareTheDyingPublicCastPersistsAndReplays() {
	for _, stable := range []bool{false, true} {
		s.Run(fmt.Sprint(stable), func() {
			ctx := context.Background()
			// Form initiative while the patient is conscious, then load their
			// injured state so the test also exercises their existing turn slot.
			s.sceneWithAllies(s.finalizedSpareTheDyingCleric(), []*character.Data{armedFighter("patient")}, 4)
			patient := s.characters.byID["patient"]
			patient.HitPoints = 0
			patient.DeathSaveState = &saves.DeathSaveState{Successes: 1, Failures: 1, Stabilized: stable}
			rolls := s.dice.next
			row := s.castRow(spells.SpareTheDying)
			candidate, found := castCandidate(s.T(), row, "patient")
			s.Require().True(found)
			s.Require().True(candidate.Available)
			_, err := s.mgr.Cast(ctx, &session.CastInput{
				Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"patient"},
			})
			s.Require().NoError(err)
			s.Equal(rolls, s.dice.next)
			s.Zero(s.characters.byID["cleric"].ActionEconomy.ActionsRemaining)
			s.Equal(2, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			s.False(s.castRow(spells.SpareTheDying).Available)
			s.Zero(s.characters.byID["patient"].HitPoints)
			s.Equal(&saves.DeathSaveState{Stabilized: true}, s.characters.byID["patient"].DeathSaveState)
			beats := s.beats(session.EventCast, session.EventActivationResult)
			s.Require().Len(beats, 2)
			result := beats[1].Body.(session.ActivationResultBody)
			s.Nil(result.HealingApplied)
			s.Require().NotNil(result.Stabilized)
			before := session.LifeStateDying
			if stable {
				before = session.LifeStateStabilized
			}
			s.Equal(&session.StabilizedBody{
				Target: "patient", SourceRef: refs.Spells.SpareTheDying().String(), SourceName: "Spare the Dying",
				Before: before, After: session.LifeStateStabilized, HitPoints: 0,
				Progress: session.DeathSaveProgress{SuccessesNeeded: 3, FailuresRemaining: 3, Stabilized: true},
			}, result.Stabilized)

			s.reloadHealingScene()
			story, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "cleric", FromSeq: beats[0].Seq})
			s.Require().NoError(err)
			s.Equal(s.beats(session.EventCast, session.EventActivationResult, session.EventDowned), story)
			loaded, err := character.Load(ctx, s.characters.byID["patient"])
			s.Require().NoError(err)
			status, err := loaded.StatusView(&character.StatusViewInput{})
			s.Require().NoError(err)
			s.Equal(combat.LifeStateStabilized, status.View.LifeState)
			s.Zero(status.View.HitPoints.Current)
			s.Equal(&character.DeathSaveProgress{SuccessesNeeded: 3, FailuresRemaining: 3, Stabilized: true}, status.View.DeathSaves)
			_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{
				Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric"),
			})
			s.Require().NoError(err)
			turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "patient"})
			s.Require().NoError(err)
			s.NotEqual("patient", turn.Active, "stable patients do not hold up the turn clock")
			offers, err := s.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "patient"})
			s.Require().NoError(err)
			s.NotContains(declarationVerbs(offers.Declarations), session.VerbDeathSave)
			s.Equal(rolls, s.dice.next, "stabilization, replay and advancing past the stable patient roll no dice")
			s.Equal(&saves.DeathSaveState{Stabilized: true}, s.characters.byID["patient"].DeathSaveState)
		})
	}
}

func (s *CastSuite) TestSpareTheDyingPublicCastRevalidatesLifeState() {
	for _, dead := range []bool{false, true} {
		s.Run(fmt.Sprint(dead), func() {
			patient := armedFighter("patient")
			patient.HitPoints = 0
			patient.DeathSaveState = &saves.DeathSaveState{Failures: 1}
			s.sceneWithAllies(s.finalizedSpareTheDyingCleric(), []*character.Data{patient}, 4)
			row := s.castRow(spells.SpareTheDying)
			if dead {
				s.characters.byID["patient"].DeathSaveState = &saves.DeathSaveState{Failures: 3, Dead: true}
			} else {
				s.characters.byID["patient"].HitPoints = 1
				s.characters.byID["patient"].DeathSaveState = nil
			}
			before, err := copyOf(s.characters.byID["cleric"])
			s.Require().NoError(err)
			rolls := s.dice.next
			_, err = s.mgr.Cast(context.Background(), &session.CastInput{
				Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"patient"},
			})
			s.ErrorIs(err, session.ErrStaleDeclaration)
			s.Equal(before, s.characters.byID["cleric"])
			s.Equal(rolls, s.dice.next)
			s.Empty(s.beats(session.EventCast, session.EventActivationResult))
		})
	}
}
