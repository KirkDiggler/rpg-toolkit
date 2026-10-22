// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package session_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func (s *CastSuite) TestGuidingBoltPublicCastAndStoryReplay() {
	for _, hit := range []bool{false, true} {
		name := "miss"
		roll := 1
		if hit {
			name = "hit"
			roll = 18
		}
		s.Run(name, func() {
			cleric := healingWordCleric()
			cleric.KnownSpells = append(cleric.KnownSpells, refs.Spells.GuidingBolt().String())
			s.scene(cleric, 4, roll, 1, 1, 1, 1)
			beforeHP := s.storedSkeleton()
			row := s.castRow(spells.GuidingBolt)
			s.Require().True(row.Available)
			before := lastSeq(storyBeats(s.T(), s.mgr, "sess", "cleric"))
			out, err := s.cast(spells.GuidingBolt)
			s.Require().NoError(err)
			s.False(out.Posed)
			s.Nil(out.Saved)
			told := storyBeatsAfter(s.T(), s.mgr, "sess", "cleric", before)
			events := beatsNamed(told, encounter.BeatCast, string(encounter.OutcomeStruck),
				string(encounter.OutcomeMissed), encounter.BeatActivationResult)
			sighted := beatsNamed(told, encounter.BeatSighted)
			if hit {
				// The light is a real change to the skeleton, so its one watcher is
				// told to look again — once, naming only the lit target (R19).
				s.Require().Len(sighted, 1, "one sight beat for the one change")
				var sight struct {
					Gained  []string `json:"gained"`
					Lost    []string `json:"lost"`
					Changed []string `json:"changed"`
				}
				sighted[0].decode(s.T(), &sight)
				s.Equal([]string{"skeleton"}, sight.Changed)
				s.Empty(sight.Gained)
				s.Empty(sight.Lost)
				s.Require().Equal([]string{encounter.BeatCast, string(encounter.OutcomeStruck), encounter.BeatActivationResult},
					beatNames(events))
				var struck struct {
					Attack struct {
						Ref        string `json:"ref"`
						DamageType string `json:"damage_type"`
					} `json:"attack"`
					Amount         int             `json:"amount"`
					PresentationID string          `json:"presentation_id"`
					Calculation    json.RawMessage `json:"calculation"`
				}
				events[1].decode(s.T(), &struck)
				s.Equal(refs.Spells.GuidingBolt().String(), struck.Attack.Ref)
				s.Equal(string(session.DamageRadiant), struck.Attack.DamageType)
				s.Equal(4, struck.Amount)
				s.NotEmpty(struck.PresentationID)
				s.NotEmpty(struck.Calculation)
				var result struct {
					Result struct {
						Kind   encounter.ActivationResultKind `json:"kind"`
						Target string                         `json:"target"`
						Ref    string                         `json:"ref"`
					} `json:"result"`
				}
				events[2].decode(s.T(), &result)
				s.Equal(encounter.ResultConditionApplied, result.Result.Kind)
				s.Equal("skeleton", result.Result.Target)
				s.Equal(refs.Conditions.GuidingBolt().String(), result.Result.Ref)
				s.Equal(beforeHP-4, s.storedSkeleton())
			} else {
				s.Empty(sighted, "a miss lights nothing, so nobody looks again")
				s.Require().Equal([]string{encounter.BeatCast, string(encounter.OutcomeMissed)}, beatNames(events))
				var missed struct {
					Attack struct {
						Ref        string `json:"ref"`
						DamageType string `json:"damage_type"`
					} `json:"attack"`
					PresentationID string `json:"presentation_id"`
				}
				events[1].decode(s.T(), &missed)
				s.Equal(refs.Spells.GuidingBolt().String(), missed.Attack.Ref)
				s.Equal(string(session.DamageRadiant), missed.Attack.DamageType)
				s.NotEmpty(missed.PresentationID)
				s.Equal(beforeHP, s.storedSkeleton())
			}
			s.Zero(s.characters.byID["cleric"].ActionEconomy.ActionsRemaining)
			s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			s.reloadHealingScene()
			beforeRolls := s.dice.next
			// Read back after a reload, the story holds the same beats and
			// replaying it rolls nothing.
			s.Equal(told, storyBeatsAfter(s.T(), s.mgr, "sess", "cleric", before))
			s.Equal(beforeRolls, s.dice.next)
		})
	}
}

func (s *CastSuite) TestGuidingBoltAttackOfferResumesWithoutPayingOrRollingAgain() {
	cleric := healingWordCleric()
	cleric.KnownSpells = append(cleric.KnownSpells, refs.Spells.GuidingBolt().String())
	s.scene(cleric, 4, 18, 1, 1, 1, 1)
	// Inspired after the launch: its first-admission long rest ends the die.
	inspired, err := conditions.NewInspiredCondition("cleric", "bard", conditions.InspiredDie).ToJSON()
	s.Require().NoError(err)
	stored := s.characters.byID["cleric"]
	stored.Conditions = append(stored.Conditions, inspired)
	out, err := s.cast(spells.GuidingBolt)
	s.Require().NoError(err)
	s.True(out.Posed)
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	s.Empty(beatsNamed(storyBeats(s.T(), s.mgr, "sess", "cleric"), string(encounter.OutcomeStruck)))
	s.reloadHealingScene()
	before := s.dice.next
	row := currentDeclaration(s.T(), s.mgr, "sess", "cleric", session.VerbReact)
	_, err = s.mgr.React(context.Background(), &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Answer: session.Decline()})
	s.Require().NoError(err)
	s.Equal(before+4, s.dice.next, "resume only rolls damage; no second attack roll")
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	beats := storyBeats(s.T(), s.mgr, "sess", "cleric")
	s.Len(beatsNamed(beats, string(encounter.OutcomeStruck)), 1)
	s.Len(beatsNamed(beats, encounter.BeatActivationResult), 1)
}
