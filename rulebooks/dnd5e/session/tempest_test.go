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
	s.Empty(s.beats(session.EventActivationResult))
	s.reloadHealingScene()
	again, err := s.mgr.Areas(context.Background(), &session.ViewInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	s.Equal(areas, again)
	raw, err := json.Marshal(s.characters.byID["cleric"].Conditions)
	s.Require().NoError(err)
	s.Contains(string(raw), refs.Conditions.InFog().String(), "membership persists through reload")
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
		s.mgr, err = session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
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
	assertFog := func(want bool) {
		raw, marshalErr := json.Marshal(s.characters.byID["cleric"].Conditions)
		s.Require().NoError(marshalErr)
		if want {
			s.Contains(string(raw), refs.Conditions.InFog().String())
		} else {
			s.NotContains(string(raw), refs.Conditions.InFog().String())
		}
	}
	assertFog(false)
	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "cleric", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "cleric"), Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}}})
	s.Require().NoError(err)
	assertFog(true)
	s.reloadHealingScene()
	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "cleric", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "cleric"), Path: []spatial.Position{{X: 3, Y: 1}, {X: 2, Y: 1}, {X: 1, Y: 1}}})
	s.Require().NoError(err)
	assertFog(false)
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
	s.mgr, err = session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
	s.Require().NoError(err)
	_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
	s.Require().NoError(err, "a lethal hit that removes fog must still record and save its outcome")
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
