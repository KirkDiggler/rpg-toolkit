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
				h := world.Perception.Intel.Holdings["member|cleric"]["member|patient"]
				h.CurrentVia = nil
				world.Perception.Intel.Holdings["member|cleric"]["member|patient"] = h
			}
			s.configureBless(session.StaleTargetRefuse)
			row := s.castRow(spells.Bless)
			candidate, found := castCandidate(s.T(), row, "patient")
			s.Require().True(found)
			s.True(candidate.Available)
			out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"patient", "cleric", "skeleton"}})
			s.Require().NoError(err)
			s.Empty(out.MissedTargets)
			applied := 0
			for _, event := range s.beats(encounter.BeatActivationResult) {
				body := resultOf(s.T(), event)
				if body.Kind != encounter.ResultConditionApplied {
					continue
				}
				applied++
				s.Nil(body.Detail, "Bless has no per-instance word to carry")
				s.NotContains(string(event.Payload), `"detail"`, "an empty detail writes no key")
			}
			s.Positive(applied, "the cast must have applied Blessed at least once for this to prove anything")
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
	s.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), StaleTargetPolicy: policy,
		PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters})
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
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), StaleTargetPolicy: "typo",
		PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters})
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
				h := world.Perception.Intel.Holdings["member|cleric"]["member|skeleton"]
				h.CurrentVia = nil
				var err error
				h.Payload, err = encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationKnown, Position: spatial.Position{X: 3, Y: 2}})
				s.Require().NoError(err)
				world.Perception.Intel.Holdings["member|cleric"]["member|skeleton"] = h
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
				beats := s.beats(encounter.BeatCast, encounter.BeatActivationResult, encounter.BeatCastMissed)
				s.Require().Len(beats, 3)
				s.Equal(encounter.BeatCastMissed, beats[2].Beat)
				want := `{"beat":"cast_missed","actor":"cleric","target":"skeleton","spell":{"ref":"` +
					refs.Spells.Bless().String() + `","name":"Bless"}}`
				s.JSONEq(want, string(beats[2].Payload))
				s.reloadHealingScene()
				s.configureBless(policy)
				foundMiss := false
				for _, event := range beatsNamed(storyBeatsAfter(s.T(), s.mgr, "sess", "cleric", beats[0].Seq-1), encounter.BeatCastMissed) {
					s.JSONEq(want, string(event.Payload))
					foundMiss = true
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
