package session_test

import (
	"context"
	"encoding/json"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func (s *CastSuite) flareSheet() *character.Data {
	sheet := s.finalizedSpareTheDyingCleric()
	sheet.ActionEconomy = &character.ActionEconomyData{ActionsRemaining: 1, BonusActionsRemaining: 1, ReactionsRemaining: 1}
	raw, err := json.Marshal(features.WardingFlareData{Ref: refs.Features.WardingFlare(), ID: "warding_flare", Name: "Warding Flare", CharacterID: sheet.ID})
	s.Require().NoError(err)
	sheet.Features = append(sheet.Features, raw)
	sheet.Resources[resources.WardingFlare] = character.RecoverableResourceData{Current: 3, Maximum: 3, ResetType: coreResources.ResetLongRest}
	return sheet
}
func (s *CastSuite) flareReaction() session.Declaration {
	offered, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "cleric"})
	s.Require().NoError(err)
	for _, row := range offered.Declarations {
		if row.Verb == session.VerbReact {
			return row
		}
	}
	s.FailNow("missing Warding Flare reaction")
	return session.Declaration{}
}
func (s *CastSuite) TestFlareMonsterTurnReloadResumesWithoutRepeatingAttack() {
	for _, spend := range []bool{true, false} {
		s.Run(map[bool]string{true: "use", false: "decline"}[spend], func() {
			sheet := s.flareSheet()
			sheet.ActionEconomy = nil
			s.scene(sheet, 1, 15, 2, 1, 4, 5)
			ctx := context.Background()
			newManager := func() {
				var err error
				s.mgr, err = session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
				s.Require().NoError(err)
			}
			newManager()
			rolls := s.dice.next
			hp := s.characters.byID["cleric"].HitPoints
			_, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
			s.Require().NoError(err)
			s.Equal(rolls, s.dice.next, "no attack roll before answering")
			s.Equal(hp, s.characters.byID["cleric"].HitPoints)
			s.Empty(s.beats(session.EventStruck, session.EventMissed))
			newManager()
			row := s.flareReaction()
			s.Require().Len(row.Options, 1)
			s.Equal("Warding Flare", row.Reaction.Name)
			in := &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Choice: session.ReactHold}
			if spend {
				in.Choice = session.ReactStrike
				in.Option = "use"
			}
			_, err = s.mgr.React(ctx, in)
			s.Require().NoError(err)
			expected := 3
			if spend {
				expected = 2
			}
			s.Equal(expected, s.characters.byID["cleric"].Resources[resources.WardingFlare].Current)
			s.Equal(2, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			live := s.beats(session.EventStruck, session.EventMissed)
			s.Require().Len(live, 1)
			_, err = s.mgr.React(ctx, in)
			s.ErrorIs(err, session.ErrNoWindow)
			turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "cleric"})
			s.Require().NoError(err)
			s.Equal("cleric", turn.Active)
			newManager()
			story, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "cleric"})
			s.Require().NoError(err)
			var replay []session.Event
			for _, e := range story {
				if e.Kind == session.EventStruck || e.Kind == session.EventMissed {
					replay = append(replay, e)
				}
			}
			s.Equal(live, replay)
		})
	}
}
func (s *CastSuite) TestFlareBeforeSpellAttackUsesSharedCastContinuation() {
	s.sceneWithAllies(castingBardWithSpells("aaron", spells.InflictWounds), []*character.Data{s.flareSheet()}, 6, 15, 1, 1, 1, 1)
	rolls := s.dice.next
	row := s.castRow(spells.InflictWounds)
	out, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "aaron", DeclarationID: row.ID, Targets: []string{"cleric"}})
	s.Require().NoError(err)
	s.True(out.Posed)
	s.Equal(rolls, s.dice.next)
	s.reloadHealingScene()
	react := s.flareReaction()
	_, err = s.mgr.React(context.Background(), &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: react.ID, Choice: session.ReactStrike, Option: "use"})
	s.Require().NoError(err)
	s.Equal(1, s.characters.byID["aaron"].Resources[resources.SpellSlotLevel1].Current)
	s.Equal(2, s.characters.byID["cleric"].Resources[resources.WardingFlare].Current)
	s.Len(s.beats(session.EventCast), 1)
}

func (s *CastSuite) TestFlareDuringOpportunityAttackResumesWithoutRepeatingStep() {
	s.scene(s.flareSheet(), 1, 15, 2, 1)
	ctx := context.Background()
	before := s.dice.next
	movement := s.characters.byID["cleric"].ActionEconomy.MovementRemaining
	_, err := s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "cleric", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "cleric"), Path: []spatial.Position{{X: 0, Y: 1}}})
	s.Require().NoError(err)
	s.Equal(before, s.dice.next)
	s.Equal(spatial.Position{X: 1, Y: 1}, s.cellOf("cleric"), "reaction pauses before leaving the cell")
	s.Equal(movement-5, s.characters.byID["cleric"].ActionEconomy.MovementRemaining)
	s.reloadHealingScene()
	row := s.flareReaction()
	_, err = s.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Choice: session.ReactStrike, Option: "use"})
	s.Require().NoError(err)
	s.Len(s.beats(session.EventStruck, session.EventMissed), 1)
	s.Equal(2, s.characters.byID["cleric"].Resources[resources.WardingFlare].Current)
	s.Equal(spatial.Position{X: 0, Y: 1}, s.cellOf("cleric"))
	s.Equal(movement-5, s.characters.byID["cleric"].ActionEconomy.MovementRemaining, "resuming does not pay movement twice")
}

func (s *CastSuite) TestFlareLethalResumeCommitsDefeatAndClosesWindow() {
	for _, spend := range []bool{false, true} {
		s.Run(map[bool]string{false: "decline", true: "use"}[spend], func() {
			sheet := s.flareSheet()
			sheet.HitPoints = 1
			rolls := []int{20, 6, 6}
			if spend {
				rolls = []int{20, 20, 6, 6}
			}
			s.scene(sheet, 1, rolls...)
			ctx := context.Background()
			var err error
			s.mgr, err = session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
			s.Require().NoError(err)
			_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
			s.Require().NoError(err)
			row := s.flareReaction()
			in := &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Choice: session.ReactHold}
			if spend {
				in.Choice = session.ReactStrike
				in.Option = "use"
			}
			_, err = s.mgr.React(ctx, in)
			s.Require().NoError(err, "a finishing attack must commit rather than resume a closed encounter")
			s.Zero(s.characters.byID["cleric"].HitPoints)
			s.Require().NotNil(s.encounters.byID["world"].Outcome)
			s.Nil(s.encounters.byID["world"].PausedTurn)
			s.Len(s.beats(session.EventStruck), 1)
			expected := 3
			if spend {
				expected = 2
			}
			s.Equal(expected, s.characters.byID["cleric"].Resources[resources.WardingFlare].Current)
			s.reloadHealingScene()
			_, err = s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "cleric"})
			s.Require().NoError(err, "closed world reloads")
		})
	}
}
