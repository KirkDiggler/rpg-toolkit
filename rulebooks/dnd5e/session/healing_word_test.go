// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func healingWordCleric() *character.Data {
	sheet := healingCleric()
	sheet.KnownSpells = append(sheet.KnownSpells, refs.Spells.HealingWord().String())
	return sheet
}

func (s *CastSuite) reloadHealingScene() {
	s.T().Helper()
	var err error
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
		PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
}

func (s *CastSuite) TestHealingWordRangedUntypedTargetAndStoryReload() {
	sheet := healingWordCleric()
	sheet.SubclassID = classes.LifeDomain
	s.scene(sheet, 4, 4)
	for i := range s.sessions.byID["sess"].NPCs {
		npc := &s.sessions.byID["sess"].NPCs[i]
		npc.Ref, npc.CreatureType, npc.HitPoints = nil, "", 10
	}
	row := s.castRow(spells.HealingWord)
	candidate, found := castCandidate(s.T(), row, "skeleton")
	s.Require().True(found)
	s.Require().True(candidate.Available)
	s.Require().Len(row.Cost, 2)
	s.Equal(session.CurrencyBonus, row.Cost[0].Currency)
	s.Equal(1, row.Cost[0].Needed)
	touch, found := castCandidate(s.T(), s.castRow(spells.CureWounds), "skeleton")
	s.Require().True(found)
	s.False(touch.Available, "the same patient is beyond touch")
	_, err := s.cast(spells.HealingWord)
	s.Require().NoError(err)
	beats := s.beats(session.EventCast, session.EventActivationResult)
	s.Require().Len(beats, 2)
	heal := beats[1].Body.(session.ActivationResultBody).HealingApplied
	s.Require().NotNil(heal)
	s.Equal(10, heal.Requested)
	s.Equal(3, heal.Amount)
	s.Equal(10, heal.HPBefore)
	s.Equal(13, heal.HPAfter)
	s.Equal(refs.Spells.HealingWord().String(), heal.SourceRef)
	s.Require().NotNil(heal.Calculation)
	s.Len(heal.Calculation.Components, 3)
	s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
	s.Equal(1, s.characters.byID["cleric"].ActionEconomy.ActionsRemaining)
	s.Zero(s.characters.byID["cleric"].ActionEconomy.BonusActionsRemaining)
	s.reloadHealingScene()
	beforeRolls := s.dice.next
	story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "cleric", FromSeq: beats[0].Seq})
	s.Require().NoError(err)
	s.Equal(beats, story)
	s.Equal(beforeRolls, s.dice.next)
}

func (s *StaleCombatEconomySuite) TestSpellHistoryEndsWithCombatBeforeRoundOneIsReused() {
	sheet := healingWordCleric()
	sheet.ID = "alice"
	armForSwinging(sheet)
	s.characters.byID["alice"] = sheet
	s.spawnAdjacentSkeleton("skel-1")
	row := currentDeclaration(s.T(), s.mgr, "sess", "alice", session.VerbCast)
	// Pick Healing Word explicitly; acquisition and spell sorting are separate.
	for _, d := range s.afford("alice").Declarations {
		if d.Spell != nil && d.Spell.Ref == refs.Spells.HealingWord().String() {
			row = d
		}
	}
	s.Require().True(row.Available)
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "alice", DeclarationID: row.ID, Targets: []string{"alice"}})
	s.Require().NoError(err)
	s.True(s.characters.byID["alice"].ActionEconomy.Spellcasting.BonusActionSpell)
	for i := range s.sessions.byID["sess"].NPCs {
		s.sessions.byID["sess"].NPCs[i].HitPoints = 1
	}
	s.killAdjacentSkeleton("skel-1")
	s.Nil(s.characters.byID["alice"].ActionEconomy, "defeat cleanup clears spell history too")
	s.spawnAdjacentSkeleton("skel-2")
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(1, turn.Round)
	for _, d := range s.afford("alice").Declarations {
		if d.Spell != nil && d.Spell.Ref == refs.Spells.CureWounds().String() {
			s.Require().True(d.Available, "the previous fight's bonus spell must not block this action spell")
			_, err = s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "alice", DeclarationID: d.ID, Targets: []string{"alice"}})
			s.Require().NoError(err)
			s.Zero(s.characters.byID["alice"].Resources[resources.SpellSlotLevel1].Current)
			return
		}
	}
	s.FailNow("Cure Wounds offer missing")
}

func (s *CastPauseSuite) TestPaidCastHistorySurvivesSavedPauseAndResume() {
	s.oneFighterScene()
	s.characters.byID["bard"].KnownSpells = append(s.characters.byID["bard"].KnownSpells, refs.Spells.HealingWord().String())
	out, err := s.whisper()
	s.Require().NoError(err)
	s.Require().True(out.Paused)
	paid, err := copyOf(s.characters.byID["bard"])
	s.Require().NoError(err)
	s.True(paid.ActionEconomy.Spellcasting.OtherSpell)
	s.NotEmpty(paid.ActionEconomy.Spellcasting.Turn)
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
		PresentationIDs: testPresentationIDs{}, Dice: whisperDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.react("fighter", session.ReactHold)
	s.Equal(6, s.fledCells())
	s.Equal(paid.ActionEconomy, s.characters.byID["bard"].ActionEconomy)
	s.Equal(paid.Resources, s.characters.byID["bard"].Resources, "resuming the walk must not pay for the cast again")
	afford, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "bard"})
	s.Require().NoError(err)
	for _, d := range afford.Declarations {
		if d.Spell != nil && d.Spell.Ref == refs.Spells.HealingWord().String() {
			s.False(d.Available)
			s.Require().NotNil(d.Why)
			s.Equal(session.ShortfallUnavailable, d.Why.Reason)
			return
		}
	}
	s.FailNow("Healing Word offer missing")
}

func (s *CastSuite) TestHealingWordAndActionSpellsUseTheSameGateAfterReload() {
	for _, first := range []spells.Spell{spells.HealingWord, spells.CureWounds} {
		s.Run(string(first), func() {
			s.scene(healingWordCleric(), 3, 4)
			second := spells.CureWounds
			if first == second {
				second = spells.HealingWord
			}
			stale := s.castRow(second).ID
			_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: s.castRow(first).ID, Targets: []string{"cleric"}})
			s.Require().NoError(err)
			s.reloadHealingScene()
			row := s.castRow(second)
			s.False(row.Available)
			s.Require().NotNil(row.Why)
			s.Equal(session.ShortfallUnavailable, row.Why.Reason)
			before, err := copyOf(s.characters.byID["cleric"])
			s.Require().NoError(err)
			rolls := s.dice.next
			_, err = s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: stale, Targets: []string{"cleric"}})
			s.ErrorIs(err, session.ErrStaleDeclaration)
			s.Equal(before, s.characters.byID["cleric"])
			s.Equal(rolls, s.dice.next)
			_, err = s.mgr.EndTurn(context.Background(), &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
			s.Require().NoError(err)
			s.True(s.castRow(second).Available, "the next active turn clears the restriction")
			s.dice.rolls = append(s.dice.rolls, 4)
			_, err = s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: s.castRow(second).ID, Targets: []string{"cleric"}})
			s.Require().NoError(err)
			s.NotEqual(before.ActionEconomy.Spellcasting.Turn, s.characters.byID["cleric"].ActionEconomy.Spellcasting.Turn)
			s.Zero(s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
		})
	}
}

func (s *CastSuite) TestHealingWordAllowsAnActionCantripInEitherOrder() {
	for _, first := range []spells.Spell{spells.HealingWord, spells.SacredFlame} {
		s.Run(string(first), func() {
			s.scene(healingWordCleric(), 3, 4, 4, 4)
			second := spells.SacredFlame
			if first == second {
				second = spells.HealingWord
			}
			for _, spell := range []spells.Spell{first, second} {
				row := s.castRow(spell)
				s.Require().True(row.Available, "%s: %+v", spell, row.Why)
				target := "skeleton"
				if spell == spells.HealingWord {
					target = "cleric"
				}
				_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{target}})
				s.Require().NoError(err)
				s.reloadHealingScene()
			}
			s.Equal(1, s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1].Current)
			s.False(s.castRow(spells.HealingWord).Available)
			s.False(s.castRow(spells.SacredFlame).Available)
		})
	}
}

func (s *CastSuite) TestSpellTurnIdentityDistinguishesActiveMembersInOneRound() {
	other := healingWordCleric()
	other.ID = "patient"
	s.sceneWithAllies(healingWordCleric(), []*character.Data{other}, 4, 4, 4)
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.HealingWord).ID, Targets: []string{"cleric"}})
	s.Require().NoError(err)
	first := s.characters.byID["cleric"].ActionEconomy.Spellcasting.Turn
	_, err = s.mgr.EndTurn(context.Background(), &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "cleric")})
	s.Require().NoError(err)
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "patient"})
	s.Require().NoError(err)
	s.Require().Equal("patient", turn.Active)
	s.Equal(1, turn.Round)
	s.member = "patient"
	_, err = s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "patient", DeclarationID: s.castRow(spells.HealingWord).ID, Targets: []string{"patient"}})
	s.Require().NoError(err)
	s.NotEqual(first, s.characters.byID["patient"].ActionEconomy.Spellcasting.Turn)
}

func (s *CastSuite) TestHealingWordRestoresDyingAndStabilizedPatients() {
	for _, stabilized := range []bool{false, true} {
		s.Run(fmt.Sprint(stabilized), func() {
			patient := armedFighter("patient")
			patient.HitPoints = 0
			patient.DeathSaveState = &saves.DeathSaveState{Successes: 1, Failures: 1, Stabilized: stabilized}
			s.sceneWithAllies(healingWordCleric(), []*character.Data{patient}, 4)
			s.dice.rolls, s.dice.next = []int{4}, 0
			_, err := s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: s.castRow(spells.HealingWord).ID, Targets: []string{"patient"}})
			s.Require().NoError(err)
			s.Equal(7, s.characters.byID["patient"].HitPoints)
			s.Empty(s.characters.byID["patient"].DeathSaveState)
		})
	}
}

func (s *CastSuite) TestHealingWordRevalidatesSightAndTargetsBeforePayment() {
	for _, invalid := range []string{"stale sight", "wall", "range", "dead", "missing", "slots"} {
		s.Run(invalid, func() {
			cells := 3
			if invalid == "range" {
				cells = 15
			}
			s.scene(healingWordCleric(), cells)
			id := s.castRow(spells.HealingWord).ID
			target := "skeleton"
			switch invalid {
			case "stale sight":
				// Persisted testimony still names channels: EncounterData's
				// field is perception.Data, whose own Intel is the store
				// underneath. perception.Holding collapsed CurrentVia into a
				// bool for READERS; the persistence shape kept the list.
				for _, world := range s.encounters.byID {
					holding := world.Perception.Intel.Holdings["cleric"]["skeleton"]
					holding.CurrentVia = nil
					world.Perception.Intel.Holdings["cleric"]["skeleton"] = holding
				}
			case "wall":
				for _, world := range s.encounters.byID {
					world.Field.Walls = append(world.Field.Walls, encounter.BoundaryData{
						From: encounter.PositionData{X: 2, Y: 1}, To: encounter.PositionData{X: 3, Y: 1}, BlocksMovement: true, BlocksLineOfSight: true,
					})
				}
			case "dead":
				s.sessions.byID["sess"].NPCs[0].HitPoints = 0
			case "missing":
				target = "missing"
			case "slots":
				pool := s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1]
				pool.Current = 0
				s.characters.byID["cleric"].Resources[resources.SpellSlotLevel1] = pool
			}
			before, err := copyOf(s.characters.byID["cleric"])
			s.Require().NoError(err)
			rolls := s.dice.next
			_, err = s.mgr.Cast(context.Background(), &session.CastInput{Session: "sess", Member: "cleric", DeclarationID: id, Targets: []string{target}})
			s.ErrorIs(err, session.ErrStaleDeclaration)
			s.Equal(before, s.characters.byID["cleric"])
			s.Equal(rolls, s.dice.next)
			s.Empty(s.beats(session.EventCast, session.EventActivationResult))
		})
	}
}
