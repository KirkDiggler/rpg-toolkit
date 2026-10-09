// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"fmt"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func (s *CastSuite) tempestSheet() *character.Data {
	sheet := s.finalizedSpareTheDyingCleric()
	sheet.ActionEconomy = &character.ActionEconomyData{ActionsRemaining: 1, BonusActionsRemaining: 1, ReactionsRemaining: 1}
	sheet.KnownSpells = append(sheet.KnownSpells, refs.Spells.FogCloud().String())
	raw, err := json.Marshal(features.WrathOfTheStormData{Ref: refs.Features.WrathOfTheStorm(), ID: "wrath", Name: "Wrath of the Storm", CharacterID: sheet.ID})
	s.Require().NoError(err)
	sheet.Features = append(sheet.Features, raw)
	sheet.Resources[resources.WrathOfTheStorm] = character.RecoverableResourceData{Current: 3, Maximum: 3, ResetType: coreResources.ResetLongRest}
	return sheet
}

func (s *CastSuite) TestFogCloudPublicCastPersistsWithoutDamageOrDice() {
	s.scene(s.tempestSheet(), 6)
	row := s.castRow(spells.FogCloud)
	s.Require().True(row.Available)
	s.Equal(session.TargetCell, row.TargetKind)
	rolls := s.dice.next
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Cell: &spatial.Position{X: 4, Y: 1}})
	s.Require().NoError(err)
	s.Equal(rolls, s.dice.next)
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	s.Zero(s.characters.byID["cleric"].ActionEconomy.ActionsRemaining)
	areas, err := s.mgr.Areas(context.Background(), &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Require().Len(areas, 1)
	s.Equal(20, areas[0].RadiusFeet)
	s.NotContains(areas[0].ID, "cleric")
	results := s.beats(session.EventActivationResult)
	s.Require().NotEmpty(results)
	result := results[0].Body.(session.ActivationResultBody)
	s.Require().NotNil(result.ConditionApplied)
	s.Equal(refs.Conditions.InFog().String(), result.ConditionApplied.Ref)
	s.Less(s.beats(session.EventCast)[0].Seq, results[0].Seq, "cast precedes membership narration")
	s.reloadHealingScene()
	again, err := s.mgr.Areas(context.Background(), &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Equal(areas, again)
	raw, err := json.Marshal(s.characters.byID["cleric"].Conditions)
	s.Require().NoError(err)
	s.NotContains(string(raw), refs.Conditions.InFog().String(),
		"membership is the encounter's answer; no sheet records it (rpg-project#539 R7)")
}

func (s *CastSuite) TestWrathPublicAttackReactPersistsWithoutRepeatingHit() {
	for _, choice := range []string{"thunder", "lightning", "thunder-save", "hold"} {
		s.Run(choice, func() {
			// A real character weapon attack supplies the triggering hit. The cleric
			// answers after a complete session reload, just like a later HTTP request.
			fighter := armedFighter("aaron")
			saveRoll := 1
			if choice == "thunder-save" {
				saveRoll = 20
			}
			s.sceneWithAllies(fighter, []*character.Data{s.tempestSheet()}, 6, 15, 3, saveRoll, 4, 5)
			afford, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "aaron"})
			s.Require().NoError(err)
			var attack session.Declaration
			for _, row := range afford.Declarations {
				if row.Verb == session.VerbAttack && row.Available {
					attack = row
					break
				}
			}
			s.Require().NotEmpty(attack.ID)
			out, err := s.mgr.Attack(context.Background(), &session.AttackInput{Session: "sess", Attacker: "aaron", Target: "cleric", DeclarationID: attack.ID})
			s.Require().NoError(err)
			s.Require().True(out.Paused)
			attackerHP := s.characters.byID["aaron"].HitPoints
			beforeHP := s.characters.byID["cleric"].HitPoints
			beforeRolls := s.dice.next
			s.reloadHealingScene()
			afford, err = s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "cleric"})
			s.Require().NoError(err)
			var react session.Declaration
			for _, row := range afford.Declarations {
				if row.Verb == session.VerbReact {
					react = row
					break
				}
			}
			s.Require().Len(react.Options, 2)
			in := &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: react.ID, Choice: session.ReactStrike, Option: choice}
			if choice == "thunder-save" {
				in.Option = "thunder"
			}
			if choice == "hold" {
				in.Choice = session.ReactHold
				in.Option = ""
			}
			invalid := *in
			invalid.Choice = session.ReactStrike
			invalid.Option = "not-offered"
			_, err = s.mgr.React(context.Background(), &invalid)
			s.ErrorIs(err, session.ErrNotOffered)
			s.Equal(beforeRolls, s.dice.next)
			s.Equal(3, s.characters.byID["cleric"].Resources[resources.WrathOfTheStorm].Current)
			_, err = s.mgr.React(context.Background(), in)
			s.Require().NoError(err)
			damage := 9
			if choice == "thunder-save" {
				damage = 4
			}
			if choice == "hold" {
				damage = 0
			}
			s.Equal(attackerHP-damage, s.characters.byID["aaron"].HitPoints)
			s.Equal(2, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)

			s.Equal(beforeHP, s.characters.byID["cleric"].HitPoints, "original hit must never repeat")
			spent := 2
			if choice == "hold" {
				spent = 3
				s.Equal(beforeRolls, s.dice.next)
			}
			s.Equal(spent, s.characters.byID["cleric"].Resources[resources.WrathOfTheStorm].Current)
			_, err = s.mgr.React(context.Background(), in)
			s.ErrorIs(err, session.ErrNoWindow)
			s.Equal(spent, s.characters.byID["cleric"].Resources[resources.WrathOfTheStorm].Current, fmt.Sprint(choice))
		})
	}
}

func (s *CastSuite) TestWrathMonsterTurnReloadResumesWithoutSecondStrike() {
	s.scene(s.tempestSheet(), 1, 15, 2, 1, 4, 5)
	ctx := context.Background()
	newManager := func() {
		var err error
		s.mgr, err = session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
		s.Require().NoError(err)
	}
	newManager()
	_, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
	s.Require().NoError(err)
	persisted, err := s.encounters.GetEncounter(ctx, "world")
	s.Require().NoError(err)
	s.Require().NotNil(persisted.PausedTurn)
	s.True(persisted.PausedTurn.AfterStrike)
	hp := s.characters.byID["cleric"].HitPoints
	newManager()
	offered, err := s.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	var row session.Declaration
	for _, d := range offered.Declarations {
		if d.Verb == session.VerbReact {
			row = d
		}
	}
	s.Require().Len(row.Options, 2)
	_, err = s.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Choice: session.ReactStrike, Option: "thunder"})
	s.Require().NoError(err)
	s.Equal(hp, s.characters.byID["cleric"].HitPoints)
	persisted, err = s.encounters.GetEncounter(ctx, "world")
	s.Require().NoError(err)
	s.Nil(persisted.PausedTurn)
	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Equal("cleric", turn.Active)
}

func (s *CastSuite) TestWrathAfterSpellAttackRecordsOneCastAndReaction() {
	s.sceneWithAllies(castingBardWithSpells("aaron", spells.InflictWounds), []*character.Data{s.tempestSheet()}, 6, 15, 1, 1, 1, 1, 4, 5)
	row := s.castRow(spells.InflictWounds)
	out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "aaron", DeclarationID: row.ID, Targets: []string{"cleric"}})
	s.Require().NoError(err)
	s.True(out.Posed)
	hp := s.characters.byID["cleric"].HitPoints
	s.reloadHealingScene()
	offered, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	var react session.Declaration
	for _, d := range offered.Declarations {
		if d.Verb == session.VerbReact {
			react = d
		}
	}
	s.Require().Len(react.Options, 2)
	_, err = s.mgr.React(context.Background(), &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: react.ID, Choice: session.ReactStrike, Option: "lightning"})
	s.Require().NoError(err)
	s.Equal(hp, s.characters.byID["cleric"].HitPoints)
	s.Equal(1, s.characters.byID["aaron"].Resources[resources.SpellSlotLevel1].Current)
	s.Equal(2, s.characters.byID["cleric"].Resources[resources.WrathOfTheStorm].Current)
	casts := s.beats(session.EventCast)
	s.Len(casts, 1)
	s.Len(s.beats(session.EventActivated), 1)
}

func (s *CastSuite) TestFogCloudExpiresAndRestoresSightAfterReload() {
	s.scene(s.tempestSheet(), 6)
	ctx := context.Background()
	row := s.castRow(spells.FogCloud)
	_, err := s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Cell: &spatial.Position{X: 4, Y: 1}})
	s.Require().NoError(err)
	// Advance the persisted duration to its final turn, preserving the real
	// concentration owner and normal EndTurn expiry/removal path.
	sheet := s.characters.byID["cleric"]
	found := false
	for i, raw := range sheet.Conditions {
		var data map[string]any
		s.Require().NoError(json.Unmarshal(raw, &data))
		if data["ref"] == refs.Conditions.Concentrating().String() {
			data["turn_ends_left"] = 1
			data["skip_next_turn_end"] = false
			sheet.Conditions[i], err = json.Marshal(data)
			s.Require().NoError(err)
			found = true
		}
	}
	s.Require().True(found)
	s.reloadHealingScene()
	_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
	s.Require().NoError(err)
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Empty(areas)
	results := s.beats(session.EventActivationResult)
	var ended []session.Event
	for _, event := range results {
		removed := event.Body.(session.ActivationResultBody).ConditionRemoved
		if removed != nil && removed.Ref == refs.Conditions.InFog().String() {
			s.Equal("area ended", removed.Reason)
			ended = append(ended, event)
		}
	}
	s.Require().NotEmpty(ended, "expiry narrates membership removal")
	s.reloadHealingScene()
	story, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	var replay []session.Event
	for _, event := range story {
		if event.Kind == session.EventActivationResult {
			replay = append(replay, event)
		}
	}
	s.Equal(results, replay, "expiration narration survives reload")
	for _, raw := range s.characters.byID["cleric"].Conditions {
		s.NotContains(string(raw), refs.Conditions.Concentrating().String())
		s.NotContains(string(raw), refs.Conditions.InFog().String(), "expiry removes membership")
	}
}

func (s *CastSuite) TestFogMembershipFollowsPublicMovement() {
	s.scene(s.tempestSheet(), 20)
	ctx := context.Background()
	row := s.castRow(spells.FogCloud)
	_, err := s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Cell: &spatial.Position{X: 7, Y: 1}})
	s.Require().NoError(err)
	// Membership is the encounter's answer, told in the story at the step
	// that crosses the edge; no sheet ever records it (rpg-project#539 R7).
	noSheetRow := func() {
		raw, marshalErr := json.Marshal(s.characters.byID["cleric"].Conditions)
		s.Require().NoError(marshalErr)
		s.NotContains(string(raw), refs.Conditions.InFog().String())
	}
	told := func(applied bool) int {
		count := 0
		for _, event := range s.beats(session.EventActivationResult) {
			body := event.Body.(session.ActivationResultBody)
			if applied && body.ConditionApplied != nil && body.ConditionApplied.Target == "cleric" {
				count++
			}
			if !applied && body.ConditionRemoved != nil && body.ConditionRemoved.Target == "cleric" {
				count++
			}
		}
		return count
	}
	s.Zero(told(true), "outside the cloud, nobody is told they entered it")
	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "cleric", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "cleric"), Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}}})
	s.Require().NoError(err)
	noSheetRow()
	s.Equal(1, told(true), "walking in is told once")
	s.Zero(told(false))
	s.reloadHealingScene()
	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "cleric", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "cleric"), Path: []spatial.Position{{X: 3, Y: 1}, {X: 2, Y: 1}, {X: 1, Y: 1}}})
	s.Require().NoError(err)
	noSheetRow()
	s.Equal(1, told(false), "walking out is told once")
	results := s.beats(session.EventActivationResult)
	s.Require().Len(results, 2, "one entry and one exit, without duplicate reload narration")
	entered := results[0].Body.(session.ActivationResultBody).ConditionApplied
	left := results[1].Body.(session.ActivationResultBody).ConditionRemoved
	s.Require().NotNil(entered)
	s.Require().NotNil(left)
	s.Equal("cleric", entered.Target)
	s.Equal(entered.SourceID, left.SourceID)
	s.Equal("left area", left.Reason)
	s.reloadHealingScene()
	story, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	var replay []session.Event
	for _, event := range story {
		if event.Kind == session.EventActivationResult {
			replay = append(replay, event)
		}
	}
	s.Equal(results, replay)
}

// A concentration-ending hit must be recorded before refreshed perception can
// notice the final downed player and close the encounter.
func (s *CastSuite) TestLethalHitBreakingFogKeepsItsStory() {
	sheet := s.tempestSheet()
	sheet.HitPoints = 2
	pool := sheet.Resources[resources.WrathOfTheStorm]
	pool.Current = 0
	sheet.Resources[resources.WrathOfTheStorm] = pool
	s.scene(sheet, 1, 15, 4, 1)
	ctx := context.Background()
	row := s.castRow(spells.FogCloud)
	_, err := s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Cell: &spatial.Position{X: 20, Y: 1}})
	s.Require().NoError(err)
	s.mgr, err = session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
	s.Require().NoError(err)
	_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
	s.Require().NoError(err, "a lethal hit that removes fog must still record and save its outcome")
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Empty(areas, "the driven strike's broken concentration ends the cloud")
	s.Zero(s.characters.byID["cleric"].HitPoints)
	s.Require().Len(s.beats(session.EventStruck), 1)
	s.Require().Len(s.beats(session.EventDowned), 1)
	live := s.beats(session.EventStruck, session.EventDowned)
	s.Equal(session.EventStruck, live[0].Kind)
	s.Equal(session.EventDowned, live[1].Kind)
	s.Less(live[0].Seq, live[1].Seq)
	s.reloadHealingScene()
	story, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	var replay []session.Event
	for _, event := range story {
		if event.Kind == session.EventStruck || event.Kind == session.EventDowned {
			replay = append(replay, event)
		}
	}
	s.Equal(live, replay, "the damaging hit and resulting downed event must survive reload unchanged")
}

// TestAStrikeThatBreaksFogInsideAWalkEndsTheArea is rpg-project#539's session
// done-when: the cleric walks out of the skeleton's reach, the opportunity
// attack breaks the concentration holding Fog Cloud, and the area ends on the
// live encounter — told to everyone the cloud held, after the strike that
// ended it.
func (s *CastSuite) TestAStrikeThatBreaksFogInsideAWalkEndsTheArea() {
	// No Wrath of the Storm to spend, so the opportunity attack lands without
	// posing the cleric a reaction.
	cleric := s.tempestSheet()
	pool := cleric.Resources[resources.WrathOfTheStorm]
	pool.Current = 0
	cleric.Resources[resources.WrathOfTheStorm] = pool
	dana := s.finalizedSpareTheDyingCleric()
	dana.ID, dana.PlayerID, dana.Name = "dana", "player-dana", "Dana"
	// The skeleton stands at the cleric's back (cells -1 puts it on (0,1)),
	// dana beside her at (2,1). The cloud is centred four cells past dana so
	// it holds dana and neither the cleric nor the skeleton, who can still see
	// each other.
	s.sceneWithAllies(cleric, []*character.Data{dana}, -1, 15, 4, 1)
	ctx := context.Background()
	row := s.castRow(spells.FogCloud)
	_, err := s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Cell: &spatial.Position{X: 6, Y: 1}})
	s.Require().NoError(err)
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Require().NotEmpty(areas, "control: the cloud stands")
	s.stream.published = nil

	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "cleric", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "cleric"), Path: []spatial.Position{{X: 1, Y: 2}, {X: 1, Y: 3}}})
	s.Require().NoError(err)

	// Ended on the live encounter, and so on the saved one.
	areas, err = s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "dana"})
	s.Require().NoError(err)
	s.Empty(areas, "the broken concentration ends the cloud")
	for _, id := range []string{"cleric", "dana"} {
		raw, marshalErr := json.Marshal(s.characters.byID[id].Conditions)
		s.Require().NoError(marshalErr)
		s.NotContains(string(raw), refs.Conditions.InFog().String(), "no sheet ever records membership")
	}

	// Dana, the one the cloud held, is told it ended — after the strike, the
	// save and the break that caused it, and before the walk goes on.
	seqOf := func(kind session.EventKind, match func(session.Event) bool) uint64 {
		for _, event := range eventsFor(s.stream.published, "dana") {
			if event.Kind == kind && match(event) {
				return event.Seq
			}
		}
		s.FailNow("dana was never told " + string(kind))
		return 0
	}
	first := func(session.Event) bool { return true }
	struck := seqOf(session.EventStruck, first)
	broken := seqOf(session.EventConcentrationEnded, first)
	ended := seqOf(session.EventActivationResult, func(e session.Event) bool {
		removed := e.Body.(session.ActivationResultBody).ConditionRemoved
		return removed != nil && removed.Target == "dana" && removed.Reason == "area ended"
	})
	moved := seqOf(session.EventMoved, first)
	s.Less(struck, broken)
	s.Less(broken, ended, "the cause is told before the membership change")
	s.Less(ended, moved, "and the area ends inside the step, before the walk goes on")
}

// fogOverTheSkeleton is aaron's fight with the tempest cleric beside him and
// the skeleton six cells off: aaron acts first, both turns pass, and the
// cleric casts Fog Cloud over the skeleton — holding it and neither player —
// then the turn comes back round to aaron. Unless wrath, the cleric holds no
// Wrath of the Storm, so a blow against her poses her nothing; with it, a hit
// on her stops after the damage to ask whether she strikes back.
func (s *CastSuite) fogOverTheSkeleton(wrath bool, rolls ...int) {
	cleric := s.tempestSheet()
	if !wrath {
		pool := cleric.Resources[resources.WrathOfTheStorm]
		pool.Current = 0
		cleric.Resources[resources.WrathOfTheStorm] = pool
	}
	fighter := armedFighter("aaron")
	s.sceneWithAllies(fighter, []*character.Data{cleric}, 6, rolls...)
	ctx := context.Background()
	endTurn := func(member string) {
		_, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: member, DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", member)})
		s.Require().NoError(err)
	}
	endTurn("aaron")
	offered, err := s.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	var fog session.Declaration
	for _, row := range offered.Declarations {
		if row.Spell != nil && row.Spell.Ref == refs.Spells.FogCloud().String() && row.Available {
			fog = row
		}
	}
	s.Require().NotEmpty(fog.ID, "the cleric is offered Fog Cloud on her turn")
	_, err = s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: fog.ID, Cell: &spatial.Position{X: 7, Y: 1}})
	s.Require().NoError(err)
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Require().NotEmpty(areas, "control: the cloud stands")
	endTurn("cleric")
	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "aaron"})
	s.Require().NoError(err)
	s.Require().Equal("aaron", turn.Active, "control: the turn is back with aaron")
	s.stream.published = nil
}

// assertTheCloudEndedAfterItsCause is the shared verdict: the area is gone
// from the live encounter, and the skeleton the cloud held is told it ended
// after the strike and the broken concentration that ended it.
func (s *CastSuite) assertTheCloudEndedAfterItsCause() {
	s.T().Helper()
	areas, err := s.mgr.Areas(context.Background(), &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Empty(areas, "the broken concentration ends the cloud")
	seqOf := func(kind session.EventKind, match func(session.Event) bool) uint64 {
		for _, event := range eventsFor(s.stream.published, "skeleton") {
			if event.Kind == kind && match(event) {
				return event.Seq
			}
		}
		s.FailNow("the skeleton was never told " + string(kind))
		return 0
	}
	first := func(session.Event) bool { return true }
	struck := seqOf(session.EventStruck, first)
	broken := seqOf(session.EventConcentrationEnded, first)
	ended := seqOf(session.EventActivationResult, func(e session.Event) bool {
		removed := e.Body.(session.ActivationResultBody).ConditionRemoved
		return removed != nil && removed.Target == "skeleton" && removed.Reason == "area ended"
	})
	s.Less(struck, broken)
	s.Less(broken, ended, "the cause is told before the membership change")
}

// TestAnAttackThatBreaksFogEndsTheArea is the player's Attack verb landing the
// area its blow closed: aaron strikes the concentrating cleric, her save
// fails, and the cloud ends for the skeleton inside it, after its cause.
func (s *CastSuite) TestAnAttackThatBreaksFogEndsTheArea() {
	s.fogOverTheSkeleton(false, 15, 4, 1)
	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{Session: "sess", Attacker: "aaron", Target: "cleric", DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "aaron")})
	s.Require().NoError(err)
	s.assertTheCloudEndedAfterItsCause()
}

// TestAResumedStrikeThatBreaksFogEndsTheArea is the same blow finished through
// a window: aaron holds a Bardic Inspiration die, so the swing stops after the
// d20 to ask, and the strike that breaks the cleric's concentration lands only
// when he answers. The resume lands the area its strike closed.
func (s *CastSuite) TestAResumedStrikeThatBreaksFogEndsTheArea() {
	s.fogOverTheSkeleton(false, 15, 4, 1)
	s.holdInspiration("aaron")
	ctx := context.Background()
	out, err := s.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "aaron", Target: "cleric", DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "aaron")})
	s.Require().NoError(err)
	s.Require().True(out.Paused, "control: the swing stops to ask about the die")
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Require().NotEmpty(areas, "control: nothing has landed while the table waits")

	react := currentDeclaration(s.T(), s.mgr, "sess", "aaron", session.VerbReact)
	_, err = s.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "aaron", DeclarationID: react.ID, Choice: session.ReactHold})
	s.Require().NoError(err)
	s.assertTheCloudEndedAfterItsCause()
}

// TestAHitThatBreaksFogEndsTheAreaBeforeItsWindow: aaron's blow breaks the
// cleric's concentration and stops to ask whether she strikes back. The area
// the blow closed lands before that window opens, so the table waits over a
// world where the cloud is already gone.
func (s *CastSuite) TestAHitThatBreaksFogEndsTheAreaBeforeItsWindow() {
	s.fogOverTheSkeleton(true, 15, 4, 1)
	out, err := s.mgr.Attack(context.Background(), &session.AttackInput{Session: "sess", Attacker: "aaron", Target: "cleric", DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "aaron")})
	s.Require().NoError(err)
	s.Require().True(out.Paused, "control: the cleric is asked whether she strikes back")
	s.assertTheCloudEndedAfterItsCause()
}

// TestAResumedHitThatBreaksFogEndsTheAreaBeforeItsNextWindow: the same blow
// reached through aaron's own post-roll window first. Answering it lands the
// strike, which breaks the cleric's concentration and poses her window; the
// area lands before that window opens.
func (s *CastSuite) TestAResumedHitThatBreaksFogEndsTheAreaBeforeItsNextWindow() {
	s.fogOverTheSkeleton(true, 15, 4, 1)
	s.holdInspiration("aaron")
	ctx := context.Background()
	out, err := s.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "aaron", Target: "cleric", DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "aaron")})
	s.Require().NoError(err)
	s.Require().True(out.Paused, "control: the swing stops to ask about the die")
	react := currentDeclaration(s.T(), s.mgr, "sess", "aaron", session.VerbReact)
	_, err = s.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "aaron", DeclarationID: react.ID, Choice: session.ReactHold})
	s.Require().NoError(err)
	offered, err := s.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	asked := false
	for _, row := range offered.Declarations {
		asked = asked || row.Verb == session.VerbReact
	}
	s.Require().True(asked, "control: the landed hit poses the cleric her window")
	s.assertTheCloudEndedAfterItsCause()
}

// TestARetaliationThatBreaksFogEndsTheArea: the concentrating member is the
// ATTACKER. Aaron, a cleric holding Fog Cloud over the skeleton, swings at the
// tempest cleric; she answers with Wrath of the Storm, its thunder breaks his
// concentration, and the area lands when her answer resolves.
func (s *CastSuite) TestARetaliationThatBreaksFogEndsTheArea() {
	aaron := s.finalizedSpareTheDyingCleric()
	aaron.ID, aaron.PlayerID, aaron.Name = "aaron", "player-aaron", "Aaron"
	aaron.ActionEconomy = &character.ActionEconomyData{ActionsRemaining: 1, BonusActionsRemaining: 1, ReactionsRemaining: 1}
	aaron.KnownSpells = append(aaron.KnownSpells, refs.Spells.FogCloud().String())
	armForSwinging(aaron)
	// Aaron's swing hits for 3; the tempest cleric's thunder: aaron's save 1,
	// damage 4 and 5; then aaron's concentration save 1.
	s.sceneWithAllies(aaron, []*character.Data{s.tempestSheet()}, 6, 15, 3, 1, 4, 5, 1)
	ctx := context.Background()
	fog := s.castRow(spells.FogCloud)
	_, err := s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "aaron", DeclarationID: fog.ID, Cell: &spatial.Position{X: 7, Y: 1}})
	s.Require().NoError(err)
	for _, member := range []string{"aaron", "cleric"} {
		_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: member, DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", member)})
		s.Require().NoError(err)
	}
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "aaron"})
	s.Require().NoError(err)
	s.Require().NotEmpty(areas, "control: aaron's cloud stands")

	out, err := s.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "aaron", Target: "cleric", DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "aaron")})
	s.Require().NoError(err)
	s.Require().True(out.Paused, "control: the cleric is asked whether she strikes back")
	areas, err = s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "aaron"})
	s.Require().NoError(err)
	s.Require().NotEmpty(areas, "control: aaron's blow ended nothing of his own")
	s.stream.published = nil

	react := currentDeclaration(s.T(), s.mgr, "sess", "cleric", session.VerbReact)
	_, err = s.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: react.ID, Choice: session.ReactStrike, Option: "thunder"})
	s.Require().NoError(err)

	areas, err = s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "aaron"})
	s.Require().NoError(err)
	s.Empty(areas, "the thunder's broken concentration ends aaron's cloud")
	var broken, ended uint64
	for _, event := range eventsFor(s.stream.published, "skeleton") {
		switch event.Kind {
		case session.EventConcentrationEnded:
			broken = event.Seq
		case session.EventActivationResult:
			if removed := event.Body.(session.ActivationResultBody).ConditionRemoved; removed != nil &&
				removed.Target == "skeleton" && removed.Reason == "area ended" {
				ended = event.Seq
			}
		}
	}
	s.Require().NotZero(broken, "the skeleton is told the concentration broke")
	s.Require().NotZero(ended, "and that the cloud it stood in ended")
	s.Less(broken, ended, "the cause is told before the membership change")
}

// TestADrivenHitThatBreaksFogEndsTheAreaBeforeItsWindow: the skeleton's own
// turn lands a hit that breaks the cleric's concentration and stops to ask
// whether she strikes back. The driven strike lands the area its hit closed
// before that window opens.
func (s *CastSuite) TestADrivenHitThatBreaksFogEndsTheAreaBeforeItsWindow() {
	s.scene(s.tempestSheet(), 1, 15, 2, 1)
	ctx := context.Background()
	row := s.castRow(spells.FogCloud)
	_, err := s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Cell: &spatial.Position{X: 20, Y: 1}})
	s.Require().NoError(err)
	s.mgr, err = session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
	s.Require().NoError(err)
	_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
	s.Require().NoError(err)
	persisted, err := s.encounters.GetEncounter(ctx, "world")
	s.Require().NoError(err)
	s.Require().NotNil(persisted.PausedTurn, "control: the hit stops to ask the cleric")
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Empty(areas, "the driven hit's broken concentration ends the cloud before the window")
}

// flareFogScene is the warding cleric holding Fog Cloud, on a world where the
// skeleton's own turn swings at her: the swing stops BEFORE its roll to ask
// whether she flares. With wrath she also holds Wrath of the Storm, so a hit
// that lands after her answer stops again to ask whether she strikes back.
func (s *CastSuite) flareFogScene(wrath bool) {
	sheet := s.flareSheet()
	sheet.KnownSpells = append(sheet.KnownSpells, refs.Spells.FogCloud().String())
	if wrath {
		raw, err := json.Marshal(features.WrathOfTheStormData{Ref: refs.Features.WrathOfTheStorm(), ID: "wrath", Name: "Wrath of the Storm", CharacterID: sheet.ID})
		s.Require().NoError(err)
		sheet.Features = append(sheet.Features, raw)
		sheet.Resources[resources.WrathOfTheStorm] = character.RecoverableResourceData{Current: 3, Maximum: 3, ResetType: coreResources.ResetLongRest}
	}
	// The skeleton's attack 15 and damage 2, then the cleric's concentration
	// save 1.
	s.scene(sheet, 1, 15, 2, 1)
	ctx := context.Background()
	row := s.castRow(spells.FogCloud)
	_, err := s.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Cell: &spatial.Position{X: 20, Y: 1}})
	s.Require().NoError(err)
	s.mgr, err = session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
	s.Require().NoError(err)
	_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
	s.Require().NoError(err)
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Require().NotEmpty(areas, "control: nothing has landed before the roll")
	row = s.flareReaction()
	_, err = s.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Choice: session.ReactHold})
	s.Require().NoError(err)
}

// TestAResumedDrivenHitThatBreaksFogEndsTheArea: the cleric declines to
// flare, the resumed swing lands and breaks her concentration, and the
// pending-attack resume lands the area it closed.
func (s *CastSuite) TestAResumedDrivenHitThatBreaksFogEndsTheArea() {
	s.flareFogScene(false)
	areas, err := s.mgr.Areas(context.Background(), &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Empty(areas, "the resumed hit's broken concentration ends the cloud")
}

// TestAResumedDrivenHitThatBreaksFogEndsTheAreaBeforeItsWindow: the same
// resumed swing, against a cleric who also holds Wrath of the Storm. The hit
// poses her next window, and the area lands before it opens.
func (s *CastSuite) TestAResumedDrivenHitThatBreaksFogEndsTheAreaBeforeItsWindow() {
	s.flareFogScene(true)
	ctx := context.Background()
	offered, err := s.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	asked := false
	for _, row := range offered.Declarations {
		asked = asked || (row.Verb == session.VerbReact && row.Reaction != nil && row.Reaction.Name == "Wrath of the Storm")
	}
	s.Require().True(asked, "control: the landed hit asks whether she strikes back")
	areas, err := s.mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Empty(areas, "the area lands before the next window opens")
}

// TestAWalkThatPausesOnItsSecondReactionTellsTheFirst: a step that provokes
// two skeletons, where the first swing misses and the second hits and asks
// the walking cleric about Wrath of the Storm. The walk pauses on the second
// reaction, and the first — already resolved — is told before the window
// opens, by the movement landing's record step.
func (s *CastSuite) TestAWalkThatPausesOnItsSecondReactionTellsTheFirst() {
	s.scene(s.tempestSheet(), 1, 1, 15, 3, 3, 3, 3, 3, 3, 3, 3)
	ctx := context.Background()
	_, err := s.mgr.Spawn(ctx, &session.SpawnInput{Session: "sess", ID: "skeleton2", Ref: refs.Monsters.Skeleton().String(), Position: spatial.Position{X: 2, Y: 0}})
	s.Require().NoError(err)

	out, err := s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "cleric", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "cleric"), Path: []spatial.Position{{X: 0, Y: 1}}})
	s.Require().NoError(err)

	s.Equal(session.MovementPaused, out.Status, "the second reaction asks the walker")
	told := s.beats(session.EventStruck, session.EventMissed)
	s.Require().Len(told, 1, "the first reaction is told before the window")
	s.Equal(session.EventMissed, told[0].Kind)
	offered, err := s.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	asked := false
	for _, d := range offered.Declarations {
		if d.Verb == session.VerbReact && d.Reaction != nil && d.Reaction.Ref == refs.Features.WrathOfTheStorm().String() {
			asked = true
		}
	}
	s.True(asked, "and the cleric is asked about Wrath of the Storm")
}
