// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// trainTarget is the fighter the goblin boss's Multiattack is aimed at in the
// record-train scenes: holding Fog Cloud, a frail constitution so a flat roll
// fails the check, and the given hit points as current AND maximum (a sheet
// loads at its maximum, so a lowered current value alone would be restored).
type trainTarget struct {
	hp   int
	ally bool
}

// bossSwingsAt launches the tomb with the boss beside the fighter, holds Fog
// Cloud on the fighter, and ends the fighter's turn so the boss's driven
// Multiattack lands on the given dice. It returns the manager and the story
// length before the turn.
func (s *MonsterTurnTestSuite) bossSwingsAt(target trainTarget, roller interface {
	Roll(context.Context, int) (int, error)
}) (*session.Manager, *fakeCharacters, int) {
	ctx := context.Background()

	fighter := armedFighter("fighter")
	fighter.HitPoints, fighter.MaxHitPoints = target.hp, target.hp
	sheets := []*character.Data{fighter}
	sc := bossBesideFighter()
	if target.ally {
		sheets = append(sheets, armedFighter("ally"))
		sc.Party = append(sc.Party, seatAt("ally", 11, 5))
	}
	chars := newFakeCharacters(sheets...)
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: roller, TurnDriver: firstInReach{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: chars, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	launchScene(s.T(), mgr, sc)
	s.holdSpellAfterJoin(chars, "fighter")
	seated, err := chars.GetCharacter(ctx, "fighter")
	s.Require().NoError(err)
	seated.AbilityScores[abilities.CON] = 6
	s.Require().NoError(chars.SaveCharacter(ctx, seated))
	s.encounters.byID["sess"].SightAreas = append(s.encounters.byID["sess"].SightAreas, encounter.SightAreaData{
		ID: "cloud", SourceID: "fighter", Name: "Fog Cloud",
		Center: encounter.PositionData{X: 9, Y: 4}, RadiusFeet: 10,
	})
	return mgr, chars, len(s.storyBeats(mgr, "fighter"))
}

func (s *MonsterTurnTestSuite) endFightersTurn(mgr *session.Manager) error {
	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	return err
}

// storyFrom is the beat kinds of the fighter's story from an index, with the
// turn bookends and picks dropped.
func (s *MonsterTurnTestSuite) trainFrom(mgr *session.Manager, from int) []string {
	var train []string
	for _, beat := range s.storyBeats(mgr, "fighter")[from:] {
		switch beat {
		case "struck", "missed", "saved", "concentration_ended", "down", "ended":
			train = append(train, beat)
		}
	}
	return train
}

// eventsFrom is the fighter's story events from an index.
func (s *MonsterTurnTestSuite) eventsFrom(mgr *session.Manager, from int) []session.Event {
	events, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	return events[from:]
}

// struckAmounts is the damage of every struck beat of a run of events.
func (s *MonsterTurnTestSuite) struckAmounts(events []session.Event) []int {
	var amounts []int
	for _, event := range events {
		if body, ok := event.Body.(session.StruckBody); ok {
			amounts = append(amounts, body.Damage)
		}
	}
	return amounts
}

// TestAMultiattackAgainstAHealthyTargetTellsBothSwings is the first of the
// three scenes of rpg-toolkit#2002: the target holds Fog Cloud at 40 hit
// points, the first swing breaks the hold, and nothing falls.
func (s *MonsterTurnTestSuite) TestAMultiattackAgainstAHealthyTargetTellsBothSwings() {
	// Initiative twice; swing one attacks 15, damage 6 and the fighter's save
	// is a 1; swing two rolls at disadvantage, 15 and 15, damage 6.
	mgr, _, before := s.bossSwingsAt(trainTarget{hp: 40}, &sequenceDice{rolls: []int{10, 10, 15, 4, 1, 15, 15, 4, 10, 10}})
	s.Require().NoError(s.endFightersTurn(mgr))

	s.Equal([]string{"struck", "saved", "concentration_ended", "struck"}, s.trainFrom(mgr, before))
}

// TestAMultiattackWhoseSecondSwingFellsTheLastStandingTellsBothThenTheFall is
// the scene that failed with ErrClosed: the consult between the two swings
// answered from the end state of the whole output, told the fall ahead of the
// second blow and closed the encounter on it.
func (s *MonsterTurnTestSuite) TestAMultiattackWhoseSecondSwingFellsTheLastStandingTellsBothThenTheFall() {
	mgr, chars, before := s.bossSwingsAt(trainTarget{hp: 12}, &sequenceDice{rolls: []int{10, 10, 15, 4, 1, 15, 15, 4, 10, 10, 10, 10}})
	s.Require().NoError(s.endFightersTurn(mgr), "a felling second swing must not refuse on a closed encounter")

	s.Equal([]string{"struck", "saved", "concentration_ended", "struck", "down", "ended"}, s.trainFrom(mgr, before))

	status, err := mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.False(status.Open, "the party-defeated ending closed the encounter")

	sheet, err := chars.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	s.Zero(sheet.HitPoints, "the stored sheet reads zero")
	sum := 0
	for _, amount := range s.struckAmounts(s.eventsFrom(mgr, before)) {
		sum += amount
	}
	s.Equal(12, sum, "the story's struck amounts agree with the sheet")
}

// TestAMultiattackWhoseSecondSwingFellsATargetWithAnAllyTellsTheFallLast: the
// same blows with an ally standing out of reach. The fall is told, after the
// second swing, and the encounter goes on.
func (s *MonsterTurnTestSuite) TestAMultiattackWhoseSecondSwingFellsATargetWithAnAllyTellsTheFallLast() {
	mgr, _, before := s.bossSwingsAt(trainTarget{hp: 12, ally: true}, &sequenceDice{rolls: []int{1, 10, 10, 15, 4, 1, 15, 15, 4, 10, 10, 10}})
	s.Require().NoError(s.endFightersTurn(mgr))

	s.Equal([]string{"struck", "saved", "concentration_ended", "struck", "down"}, s.trainFrom(mgr, before))
}

// TestARetaliationThatFellsTheAttackerIsToldBeforeTheFall: the boss's first
// swing hits the fighter holding Wrath of the Storm, who takes it. The boss is
// on one hit point, so the retaliation fells it. The resumed landing tells the
// retaliation's beats and the swing that followed, and only then the boss's
// fall: a consult between the retaliation and the next unit would tell the
// fall in the middle of the train.
//
// THE SWING THAT FOLLOWS is resolution's: its sequence stops when the TARGET
// falls and does not ask whether the ATTACKER still stands, so a boss felled by
// the retaliation still makes its second swing. That is not this seam's to
// close, and this test pins only where the fall is told.
func (s *MonsterTurnTestSuite) TestARetaliationThatFellsTheAttackerIsToldBeforeTheFall() {
	ctx := context.Background()
	// Initiative twice; swing one attacks 15, damage 3 and the fighter's save
	// is a 1. On the answer, Wrath's save for the boss is a 4 and its two dice
	// 4 and 5; the second swing rolls 3 and 3 at disadvantage and misses.
	mgr := s.bossBreaksTheFightersAreaWith(true, &sequenceDice{rolls: []int{10, 10, 15, 3, 1, 4, 4, 5, 3, 3, 3, 3}})
	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == "goblin-boss-1" {
			s.sessions.byID["sess"].NPCs[i].HitPoints = 1
		}
	}
	before := len(s.storyBeats(mgr, "fighter"))

	react := currentDeclaration(s.T(), mgr, "sess", "fighter", session.VerbReact)
	_, err := mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "fighter", DeclarationID: react.ID, Answer: session.Take("thunder")})
	s.Require().NoError(err)

	var resumed []string
	for _, beat := range s.storyBeats(mgr, "fighter")[before:] {
		switch beat {
		case "activated", "saved", "activation-result", "struck", "missed", "down":
			resumed = append(resumed, beat)
		}
	}
	s.Require().NotEmpty(resumed)
	s.Equal("activated", resumed[0], "the retaliation is the first thing told")
	s.Equal("down", resumed[len(resumed)-1], "the boss's fall follows every beat the landing told")
	s.Equal(1, count(resumed, "down"))
}

func count(beats []string, kind string) int {
	n := 0
	for _, beat := range beats {
		if beat == kind {
			n++
		}
	}
	return n
}

// TestEachMultiattackSwingNamesItsSequence: both swings of the goblin boss's
// Multiattack carry the sequence's own ref and name, beside the scimitar each
// one swung.
func (s *MonsterTurnTestSuite) TestEachMultiattackSwingNamesItsSequence() {
	mgr, _, before := s.bossSwingsAt(trainTarget{hp: 40}, &sequenceDice{rolls: []int{10, 10, 15, 6, 1, 15, 15, 6, 10, 10}})
	s.Require().NoError(s.endFightersTurn(mgr))

	var swings []session.StruckBody
	for _, event := range s.eventsFrom(mgr, before) {
		if body, ok := event.Body.(session.StruckBody); ok {
			swings = append(swings, body)
		}
	}
	s.Require().Len(swings, 2)
	for _, swing := range swings {
		s.Equal(refs.Weapons.Scimitar().String(), swing.Attack.Ref, "the component that swung")
		s.Equal(&session.SequenceRef{
			Ref: refs.MonsterActions.GoblinBossMultiattack().String(), Name: "Multiattack",
		}, swing.Sequence, "and the sequence it swung inside")
	}
}

// TestAMultiattackMissNamesItsSequence: a missed swing inside a multiattack
// names it too, and the key rides the beat's own payload.
func (s *MonsterTurnTestSuite) TestAMultiattackMissNamesItsSequence() {
	// Both swings roll a 1.
	mgr, _, before := s.bossSwingsAt(trainTarget{hp: 40}, &sequenceDice{rolls: []int{10, 10, 1, 1, 1, 10, 10, 10}})
	s.Require().NoError(s.endFightersTurn(mgr))

	missed := 0
	for _, event := range s.eventsFrom(mgr, before) {
		if body, ok := event.Body.(session.MissedBody); ok {
			missed++
			s.Require().NotNil(body.Sequence)
			s.Equal("Multiattack", body.Sequence.Name)
			s.Contains(string(event.Payload), `"sequence"`)
		}
	}
	s.Equal(2, missed)
}

// TestALoneSwingNamesNoSequence: a player's declared attack is not inside any
// sequence, so its struck body has a nil Sequence and its beat has no
// "sequence" key at all.
func (s *EffectRowsSuite) TestALoneSwingNamesNoSequence() {
	s.cave(s.fighter())
	declared := s.mainAttack(s.afford("alice"))
	_, err := s.mgr.Attack(s.ctx, &session.AttackInput{
		Session: erSession, Attacker: "alice", Target: erGoblin1, DeclarationID: declared.ID,
	})
	s.Require().NoError(err)

	struck := eventsOfKind(s.stream.published, "alice", session.EventStruck)
	s.Require().Len(struck, 1)
	body, ok := struck[0].Body.(session.StruckBody)
	s.Require().True(ok)
	s.Nil(body.Sequence)
	var payload map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(struck[0].Payload, &payload))
	s.NotContains(payload, "sequence")
}

// TestTwoReactionsThatFellTheMoverAreBothTold: a fighter on 12 hit points
// steps out of two skeletons' reach. Both opportunity attacks hit and the
// second fells her. The step is one landing, so the story reads both reaction
// swings and then the fall, never the fall between them.
func (s *MoverSeamSuite) TestTwoReactionsThatFellTheMoverAreBothTold() {
	ctx := context.Background()

	frail := armedFighter("fighter")
	frail.MaxHitPoints, frail.HitPoints = 12, 12
	s.characters = newFakeCharacters(frail, armedFighter("ally"))
	mgr := s.managerWith(session.Pass{})

	first, second := authoredOf(hexCell(3, 0)), authoredOf(spatial.Position{X: 2, Y: 1})
	sc := tombRoom(12, 6)
	sc.Party = []sceneSeat{{ID: "fighter", At: authoredOf(hexCell(2, 0))}}
	sc.Monsters = []dungeonspec.MonsterPlacement{
		monsterAt("skel-1", refs.Monsters.Skeleton().String(), int(first.X), int(first.Y)),
		monsterAt("skel-2", refs.Monsters.Skeleton().String(), int(second.X), int(second.Y)),
	}
	s.Require().NotEmpty(launchScene(s.T(), mgr, sc).Formed, "adjacent and in sight starts the fight")
	s.inCombat("fighter", 1)

	before := len(s.reactionStory(mgr))
	_, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(1, 0)},
		DeclarationID: currentMoveID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)

	var told []string
	for _, beat := range s.reactionStory(mgr)[before:] {
		switch beat {
		case "struck", "missed", "down":
			told = append(told, beat)
		}
	}
	s.Equal([]string{"struck", "struck", "down"}, told, "both reactions are told, then the fall")
}

// reactionStory is the fighter's whole story as beat kinds.
func (s *MoverSeamSuite) reactionStory(mgr *session.Manager) []string {
	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	beats := make([]string, len(entries))
	for i, entry := range entries {
		var body struct {
			Beat string `json:"beat"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		beats[i] = body.Beat
	}
	return beats
}
