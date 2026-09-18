// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package session_test

import (
	"context"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
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
			out, err := s.cast(spells.GuidingBolt)
			s.Require().NoError(err)
			s.False(out.Posed)
			s.Nil(out.Saved)
			events := s.beats(session.EventCast, session.EventStruck, session.EventMissed, session.EventActivationResult)
			if hit {
				s.Require().Len(events, 3)
				struck := events[1].Body.(session.StruckBody)
				s.Equal(refs.Spells.GuidingBolt().String(), struck.Attack.Ref)
				s.Equal(session.DamageRadiant, struck.Attack.DamageType)
				s.Equal(4, struck.Damage)
				s.NotEmpty(struck.PresentationID)
				s.Require().NotNil(struck.Calculation)
				condition := events[2].Body.(session.ActivationResultBody).ConditionApplied
				s.Require().NotNil(condition)
				s.Equal("skeleton", condition.Target)
				s.Equal(refs.Conditions.GuidingBolt().String(), condition.Ref)
				s.Equal(beforeHP-4, s.storedSkeleton())
			} else {
				s.Require().Len(events, 2)
				missed := events[1].Body.(session.MissedBody)
				s.Equal(refs.Spells.GuidingBolt().String(), missed.Attack.Ref)
				s.Equal(session.DamageRadiant, missed.Attack.DamageType)
				s.NotEmpty(missed.PresentationID)
				s.Equal(beforeHP, s.storedSkeleton())
			}
			s.Zero(s.characters.byID["cleric"].ActionEconomy.ActionsRemaining)
			s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			s.reloadHealingScene()
			beforeRolls := s.dice.next
			story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "cleric", FromSeq: events[0].Seq})
			s.Require().NoError(err)
			s.Equal(events, story)
			s.Equal(beforeRolls, s.dice.next)
		})
	}
}

func (s *CastSuite) TestGuidingBoltAttackOfferResumesWithoutPayingOrRollingAgain() {
	cleric := healingWordCleric()
	cleric.KnownSpells = append(cleric.KnownSpells, refs.Spells.GuidingBolt().String())
	inspired, err := conditions.NewInspiredCondition("cleric", "bard", conditions.InspiredDie).ToJSON()
	s.Require().NoError(err)
	cleric.Conditions = append(cleric.Conditions, inspired)
	s.scene(cleric, 4, 18, 1, 1, 1, 1)
	out, err := s.cast(spells.GuidingBolt)
	s.Require().NoError(err)
	s.True(out.Posed)
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	s.Empty(s.beats(session.EventStruck))
	s.reloadHealingScene()
	before := s.dice.next
	row := currentDeclaration(s.T(), s.mgr, "sess", "cleric", session.VerbReact)
	_, err = s.mgr.React(context.Background(), &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Choice: session.ReactHold})
	s.Require().NoError(err)
	s.Equal(before+4, s.dice.next, "resume only rolls damage; no second attack roll")
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	s.Len(s.beats(session.EventStruck), 1)
	s.Len(s.beats(session.EventActivationResult), 1)
}
