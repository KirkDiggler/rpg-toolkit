// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func blessCleric() *character.Data {
	ch := healingCleric()
	ch.KnownSpells = append(ch.KnownSpells, refs.Spells.Bless().String())
	return ch
}

func (s *CastSuite) TestBlessKnownDyingAndStabilizedRecipientsPersistWithConcentration() {
	for _, stable := range []bool{false, true} {
		s.Run(map[bool]string{false: "dying", true: "stabilized"}[stable], func() {
			patient := armedFighter("patient")
			s.sceneWithAllies(blessCleric(), []*character.Data{patient}, 4)
			s.characters.byID["patient"].HitPoints = 0
			s.characters.byID["patient"].DeathSaveState = &saves.DeathSaveState{Stabilized: stable}
			for _, world := range s.encounters.byID {
				h := world.Perception.Intel.Holdings["cleric"]["patient"]
				h.CurrentVia = nil
				world.Perception.Intel.Holdings["cleric"]["patient"] = h
			}
			s.configureBless(session.StaleTargetRefuse)
			row := s.castRow(spells.Bless)
			candidate, found := castCandidate(s.T(), row, "patient")
			s.Require().True(found)
			s.True(candidate.Available)
			out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"patient", "cleric", "skeleton"}})
			s.Require().NoError(err)
			s.Empty(out.MissedTargets)
			for _, member := range []string{"patient", "cleric"} {
				raw, err := json.Marshal(s.characters.byID[member].Conditions)
				s.Require().NoError(err)
				s.Contains(string(raw), "blessed")
				if member == "cleric" {
					s.Contains(string(raw), "concentrating")
				}
			}
			s.Equal(0, s.characters.byID["patient"].HitPoints)
			s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
		})
	}
}

func (s *CastSuite) configureBless(policy session.StaleTargetPolicy) {
	var err error
	s.mgr, err = session.NewManager(&session.Config{StaleTargetPolicy: policy,
		PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
	s.Require().NoError(err)
}

func (s *CastSuite) TestBlessMissingPolicyExplainsBothOfferAndCast() {
	s.scene(blessCleric(), 3)
	row := s.castRow(spells.Bless)
	s.False(row.Available)
	s.Require().NotNil(row.Why)
	s.Equal("Known-creature casting requires a stale-target policy", row.Why.Text)
	s.True(s.castRow(spells.CureWounds).Available)
	before := s.dice.next
	out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"cleric"}})
	s.ErrorIs(err, session.ErrIncompleteConfig)
	s.ErrorContains(err, row.Why.Text)
	s.Nil(out)
	s.Equal(before, s.dice.next)
	s.Equal(2, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
}

func (s *CastSuite) TestBlessInvalidPolicyFailsConfiguration() {
	s.scene(blessCleric(), 3)
	mgr, err := session.NewManager(&session.Config{StaleTargetPolicy: "typo",
		PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
	s.Nil(mgr)
	s.ErrorIs(err, session.ErrIncompleteConfig)
	s.ErrorContains(err, "StaleTargetPolicy")
}

func (s *CastSuite) TestBlessPoliciesMixedCastAndStoryReload() {
	for _, policy := range []session.StaleTargetPolicy{session.StaleTargetRefuse, session.StaleTargetAttempt} {
		s.Run(string(policy), func() {
			s.scene(blessCleric(), 3)
			s.configureBless(policy)
			row := s.castRow(spells.Bless)
			for _, world := range s.encounters.byID {
				h := world.Perception.Intel.Holdings["cleric"]["skeleton"]
				h.CurrentVia = nil
				var err error
				h.Payload, err = encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationKnown, Position: spatial.Position{X: 3, Y: 2}})
				s.Require().NoError(err)
				world.Perception.Intel.Holdings["cleric"]["skeleton"] = h
			}
			candidate, found := castCandidate(s.T(), s.castRow(spells.Bless), "skeleton")
			s.Require().True(found)
			s.Equal(policy == session.StaleTargetAttempt, candidate.Available)
			before := s.dice.next
			out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"cleric", "skeleton"}})
			if policy == session.StaleTargetRefuse {
				s.Error(err)
				s.Nil(out)
				s.Equal(2, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			} else {
				s.Require().NoError(err)
				s.Equal([]string{"skeleton"}, out.MissedTargets)
				s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
				s.Zero(s.characters.byID["cleric"].ActionEconomy.ActionsRemaining)
				beats := s.beats(session.EventCast, session.EventActivationResult, session.EventCastMissed)
				s.Require().Len(beats, 3)
				s.Equal(session.EventCastMissed, beats[2].Kind)
				want := session.CastMissedBody{Actor: "cleric", Target: "skeleton", Spell: session.SpellRef{Ref: refs.Spells.Bless().String(), Name: "Bless"}}
				s.Equal(want, beats[2].Body)
				s.reloadHealingScene()
				s.configureBless(policy)
				story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "cleric", FromSeq: beats[0].Seq})
				s.Require().NoError(err)
				foundMiss := false
				for _, event := range story {
					if event.Kind == session.EventCastMissed {
						s.Equal(want, event.Body)
						foundMiss = true
					}
				}
				s.True(foundMiss)
			}
			s.Equal(before, s.dice.next)
		})
	}
}

func (s *CastSuite) TestBlessPolicyChangeInvalidatesOnlyKnownCastSelector() {
	s.scene(blessCleric(), 3)
	s.configureBless(session.StaleTargetRefuse)
	old := s.castRow(spells.Bless).ID
	healing := s.castRow(spells.CureWounds).ID
	s.configureBless(session.StaleTargetAttempt)
	s.NotEqual(old, s.castRow(spells.Bless).ID)
	s.Equal(healing, s.castRow(spells.CureWounds).ID)
	out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: old, Targets: []string{"cleric"}})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.Nil(out)
}
