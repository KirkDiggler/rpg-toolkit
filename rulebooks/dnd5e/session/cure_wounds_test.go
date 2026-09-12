// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func healingCleric() *character.Data {
	sheet := castingCleric()
	sheet.KnownSpells = []string{refs.Spells.CureWounds().String()}
	sheet.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{
		resources.SpellSlotLevel1: {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
	}
	return sheet
}

func (s *CastSuite) TestCureWoundsLifeBonusClampingAndStoryReload() {
	sheet := healingCleric()
	sheet.SubclassID = classes.LifeDomain
	patient := armedFighter("patient")
	patient.HitPoints, patient.MaxHitPoints = 16, 20
	s.sceneWithAllies(sheet, []*character.Data{patient}, 4, 5)
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.CureWounds).ID, Targets: []string{"patient"},
	})
	s.Require().NoError(err)
	beats := s.beats(session.EventCast, session.EventActivationResult)
	s.Require().Len(beats, 2)
	body, ok := beats[1].Body.(session.ActivationResultBody)
	s.Require().True(ok)
	heal := body.HealingApplied
	s.Require().NotNil(heal)
	s.Equal(11, heal.Requested)
	s.Equal(4, heal.Amount)
	s.Equal(16, heal.HPBefore)
	s.Equal(20, heal.HPAfter)
	s.Equal(refs.Spells.CureWounds().String(), heal.SourceRef)
	s.Require().NotNil(heal.Calculation)
	s.Require().Len(heal.Calculation.Components, 3)
	s.Equal([]int{5}, heal.Calculation.Components[0].Dice.FinalRolls)
	s.Equal(refs.Abilities.Wisdom().String(), heal.Calculation.Components[1].Source.Ref)
	s.Equal(3, *heal.Calculation.Components[1].Modifier)
	s.Equal(refs.Features.DiscipleOfLife().String(), heal.Calculation.Components[2].Source.Ref)
	s.Equal(3, *heal.Calculation.Components[2].Modifier)
	s.Equal(4, s.dice.next, "three initiative rolls and one healing die")
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
	story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "cleric", FromSeq: beats[0].Seq})
	s.Require().NoError(err)
	s.Equal(beats, story)
	s.Equal(20, s.characters.byID["patient"].HitPoints)
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
}

func (s *CastSuite) TestCureWoundsRestoresDyingAndStabilizedCharacters() {
	for _, stabilized := range []bool{false, true} {
		s.Run(fmt.Sprint(stabilized), func() {
			patient := armedFighter("patient")
			patient.HitPoints = 0
			patient.DeathSaveState = &saves.DeathSaveState{Successes: 1, Failures: 1, Stabilized: stabilized}
			s.sceneWithAllies(healingCleric(), []*character.Data{patient}, 4, 5)
			s.dice.rolls, s.dice.next = []int{5}, 0 // down characters do not roll initiative
			_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.CureWounds).ID, Targets: []string{"patient"}})
			s.Require().NoError(err)
			s.Equal(8, s.characters.byID["patient"].HitPoints)
			s.Empty(s.characters.byID["patient"].DeathSaveState)
		})
	}
}

func (s *CastSuite) TestCureWoundsUndeadIsPaidNoEffect() {
	s.scene(healingCleric(), 1)
	before := s.storedSkeleton()
	_, err := s.cast(spells.CureWounds)
	s.Require().NoError(err)
	s.Equal(before, s.storedSkeleton())
	s.Equal(2, s.dice.next, "no healing dice on an unaffected recipient")
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	beats := s.beats(session.EventActivationResult)
	s.Require().Len(beats, 1)
	body := beats[0].Body.(session.ActivationResultBody)
	s.Require().NotNil(body.HealingApplied)
	s.Zero(body.HealingApplied.Requested)
	s.Zero(body.HealingApplied.Amount)
	s.Equal("No effect on undead", body.HealingApplied.Calculation.Components[0].Source.Label)
}

func (s *CastSuite) TestCureWoundsUntypedMonsterSelectionHealingAndReload() {
	for _, tc := range []struct {
		name string
		ref  *core.Ref
	}{
		{name: "missing ref and type"},
		{name: "unknown custom ref", ref: &core.Ref{Module: "custom", Type: "monsters", ID: "patient"}},
	} {
		s.Run(tc.name, func() {
			s.scene(healingCleric(), 1, 5)
			// Reuse the scene's placement and stat bundle as an untyped custom NPC.
			for i := range s.sessions.byID["sess"].NPCs {
				npc := &s.sessions.byID["sess"].NPCs[i]
				if npc.ID == "skeleton" {
					npc.Ref = tc.ref
					npc.CreatureType = ""
					npc.HitPoints = 1
				}
			}
			row := s.castRow(spells.CureWounds)
			candidate, found := castCandidate(s.T(), row, "skeleton")
			s.Require().True(found, "missing classification must not hide the target")
			s.True(candidate.Available)
			_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"skeleton"}})
			s.Require().NoError(err)
			s.Equal(9, s.storedSkeleton())
			s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			s.False(s.castRow(spells.CureWounds).Available, "the action was spent")
			s.Equal(3, s.dice.next, "two initiative rolls and one healing die")
			beats := s.beats(session.EventCast, session.EventActivationResult)
			s.Require().Len(beats, 2)
			heal := beats[1].Body.(session.ActivationResultBody).HealingApplied
			s.Require().NotNil(heal)
			s.Equal(8, heal.Requested)
			s.Equal(8, heal.Amount)
			s.Equal(1, heal.HPBefore)
			s.Equal(9, heal.HPAfter)
			s.Equal(refs.Spells.CureWounds().String(), heal.SourceRef)
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
			s.mgr, err = session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: brokenDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
			s.Require().NoError(err)
			story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "cleric", FromSeq: beats[0].Seq})
			s.Require().NoError(err)
			s.Equal(beats, story, "reload replays the result without another roll")
			s.Equal(9, s.storedSkeleton())
			s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			for _, npc := range s.sessions.byID["sess"].NPCs {
				if npc.ID == "skeleton" {
					s.Equal(tc.ref, npc.Ref)
					s.Empty(npc.CreatureType, "healing does not invent classification")
				}
			}
		})
	}
}

func (s *CastSuite) TestCureWoundsInvalidRequestsLeaveDiceSlotsAndHPAlone() {
	for _, targets := range [][]string{nil, {"cleric", "skeleton"}, {"missing"}, {"skeleton"}} {
		s.Run(fmt.Sprint(targets), func() {
			sheet := healingCleric()
			sheet.HitPoints = 2
			s.scene(sheet, 4)
			before, err := copyOf(s.characters.byID["cleric"])
			s.Require().NoError(err)
			_, err = s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.CureWounds).ID, Targets: targets})
			s.Require().Error(err)
			s.Equal(before, s.characters.byID["cleric"])
			s.Equal(2, s.dice.next)
			s.Empty(s.beats(session.EventCast, session.EventActivationResult))
		})
	}
}

func (s *CastSuite) TestCureWoundsHealsSelfAndPaysOnce() {
	sheet := healingCleric()
	sheet.HitPoints = 2
	s.scene(sheet, 2, 5)
	row := s.castRow(spells.CureWounds)
	s.Require().True(row.Available)
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"cleric"},
	})
	s.Require().NoError(err)
	s.Equal(10, s.characters.byID["cleric"].HitPoints)
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	s.Empty(s.beats(session.EventSaved))
	s.Len(s.beats(session.EventActivationResult), 1)
	s.False(s.castRow(spells.CureWounds).Available, "one action has been spent")
	before := s.dice.next
	_, err = s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"cleric"}})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.Equal(before, s.dice.next)
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	rested, err := resolution.LongRest(context.Background(), &resolution.LongRestInput{Character: s.characters.byID["cleric"]})
	s.Require().NoError(err)
	s.Equal(2, rested.Character.Resources[resources.SpellSlotLevel1].Current)
}

func (s *CastSuite) TestCureWoundsFullHPStillPaysAndReportsZeroApplied() {
	s.scene(healingCleric(), 2, 5)
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.CureWounds).ID, Targets: []string{"cleric"}})
	s.Require().NoError(err)
	beat := s.beats(session.EventActivationResult)[0].Body.(session.ActivationResultBody)
	s.Equal(8, beat.HealingApplied.Requested)
	s.Zero(beat.HealingApplied.Amount)
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
}

func (s *CastSuite) TestCureWoundsRejectsExhaustedSlotsRemovedAccessAndDeadTargets() {
	for _, why := range []string{"slots", "access", "dead", "modifier changed"} {
		s.Run(why, func() {
			patient := armedFighter("patient")
			s.sceneWithAllies(healingCleric(), []*character.Data{patient}, 4)
			id := s.castRow(spells.CureWounds).ID
			sheet := s.characters.byID["cleric"]
			switch why {
			case "slots":
				pool := sheet.Resources[resources.SpellSlotLevel1]
				pool.Current = 0
				sheet.Resources[resources.SpellSlotLevel1] = pool
			case "access":
				sheet.KnownSpells = nil
			case "dead":
				s.characters.byID["patient"].HitPoints = 0
				s.characters.byID["patient"].DeathSaveState = &saves.DeathSaveState{Dead: true}
			case "modifier changed":
				sheet.AbilityScores[abilities.WIS] = 18
			}
			before, err := copyOf(sheet)
			s.Require().NoError(err)
			_, err = s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: id, Targets: []string{"patient"}})
			s.ErrorIs(err, session.ErrStaleDeclaration)
			s.Equal(before, s.characters.byID["cleric"])
			s.Equal(3, s.dice.next)
		})
	}
}

func (s *CastSuite) TestCureWoundsCanTouchKnownCreatureInDarkness() {
	patient := armedFighter("patient")
	patient.HitPoints = 1
	s.sceneWithAllies(healingCleric(), []*character.Data{patient}, 4, 5)
	for _, world := range s.encounters.byID {
		for i := range world.Field.Regions {
			dark := 0.0
			world.Field.Regions[i].Lighting = &encounter.LightingData{Intensity: &dark}
		}
	}
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.CureWounds).ID, Targets: []string{"patient"}})
	s.Require().NoError(err)
	s.Equal(9, s.characters.byID["patient"].HitPoints)
}

func (s *CastSuite) TestCureWoundsFailureBoundaries() {
	for _, failure := range []string{"roller", "encounter save", "delivery"} {
		s.Run(failure, func() {
			sheet := healingCleric()
			sheet.HitPoints = 2
			s.scene(sheet, 2, 5)
			id := s.castRow(spells.CureWounds).ID
			config := &session.Config{PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream}
			fault := errors.New("injected failure")
			switch failure {
			case "roller":
				config.Dice = brokenDice{}
			case "encounter save":
				config.Encounters = &failingEncounters{fakeEncounters: s.encounters, saveErr: fault}
			case "delivery":
				config.Events = &failingStream{err: fault}
			}
			var err error
			s.mgr, err = session.NewManager(config)
			s.Require().NoError(err)
			out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: id, Targets: []string{"cleric"}})
			if failure == "roller" {
				s.Error(err)
				s.Nil(out)
				s.Equal(2, s.characters.byID["cleric"].HitPoints)
				s.Equal(2, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
				return
			}
			s.Equal(10, s.characters.byID["cleric"].HitPoints)
			s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			if failure == "encounter save" {
				s.ErrorIs(err, session.ErrSaveFailed)
				s.Nil(out)
				var saveErr *session.SaveError
				s.Require().ErrorAs(err, &saveErr)
				s.Contains(saveErr.Report.Written, "character:cleric")
				s.Contains(saveErr.Report.Failed, "encounter:world")
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(out)
				s.True(out.Delivery.Failed)
				story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "cleric", FromSeq: out.Seqs[0] - 1})
				s.Require().NoError(err)
				s.Len(story, 2)
			}
		})
	}
}
