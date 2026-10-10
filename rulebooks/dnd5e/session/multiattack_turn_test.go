// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// firstInReach takes the first action it can reach an opposed player with,
// which for a goblin boss is its Multiattack — the stat block's own order,
// projected in that order, read in that order.
//
// WRITTEN HERE rather than taken off the shelf, for the reason every other
// driver fixture in this package is: this test is about what one Attack
// intent produces, not about which policy the shipped content holds.
type firstInReach struct{}

func (firstInReach) Act(view session.MonsterView) (session.TurnIntent, error) {
	for _, seen := range view.Seen {
		if seen.Kind != session.KindPlayer || !seen.Standing {
			continue
		}
		for _, action := range view.Actions {
			if seen.InReach[action.Ref] {
				return session.Attack{Target: seen.ID, Action: action.Ref}, nil
			}
		}
	}

	return session.Pass{}, nil
}

// swingBeat is the part of a struck/missed payload these scenes read. The
// whole payload is deliberately not pinned: this is about how many swings
// happened, what they were, and which die counted.
type swingBeat struct {
	Beat        string            `json:"beat"`
	Actor       string            `json:"actor"`
	Targets     []string          `json:"targets"`
	Attack      session.AttackRef `json:"attack"`
	Calculation struct {
		Components []struct {
			Dice *struct {
				OriginalRolls []int `json:"original_rolls"`
				KeptIndices   []int `json:"kept_indices"`
				Keep          *struct {
					Rule    string `json:"rule"`
					Imposed []struct {
						Name     string `json:"name"`
						Ref      string `json:"ref"`
						SourceID string `json:"source_id"`
					} `json:"imposed"`
				} `json:"keep"`
			} `json:"dice"`
		} `json:"components"`
	} `json:"calculation"`
}

// swingsIn reads every struck/missed payload out of a run of story entries,
// in order.
func (s *MonsterTurnTestSuite) swingsIn(mgr *session.Manager, member string, from int) []swingBeat {
	s.T().Helper()

	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	swings := make([]swingBeat, 0, 2)
	for _, entry := range entries[from:] {
		var beat swingBeat
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Beat == "struck" || beat.Beat == "missed" {
			swings = append(swings, beat)
		}
	}

	return swings
}

// TestAGoblinBossMultiattackIsTwoBeatsInOneTurn is the wave's acceptance at
// this seam, and every claim in it is one the single-swing path could not
// have made.
//
// ONE INTENT, TWO BEATS. The driver declares a single Attack naming the
// Multiattack; the composition's turn loop calls Striker.Strike once and
// spends the turn's attack — which is exactly what it did before this wave
// and why encounter needed no change. What changed is underneath: that one
// call now writes two Struck beats, because a beat is a roll and there were
// two rolls.
//
// EACH BEAT NAMES THE SCIMITAR. Not "Multiattack": the script is not what hit
// anybody, and the ref on a beat is what a client maps to a model, an icon
// and a damage type.
//
// THE SECOND DIE SHOWS BOTH FACES AND SAYS WHY. The keep record travels from
// the sequence's declared reason all the way onto the persisted beat, which is
// the whole point of declaring disadvantage as a reason rather than a flag.
func (s *MonsterTurnTestSuite) TestAGoblinBossMultiattackIsTwoBeatsInOneTurn() {
	ctx := context.Background()
	mgr := s.tombManager(firstInReach{}, testDice{})

	launched := launchScene(s.T(), mgr, bossBesideFighter())
	s.Require().NotEmpty(launched.Formed)

	before := len(s.storyBeats(mgr, "fighter"))
	out, err := mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err, "the boss's whole turn drives inside this one call")

	swings := s.swingsIn(mgr, "fighter", before)
	s.Require().Len(swings, 2, "one Attack intent, two swings, two beats")

	for i, swing := range swings {
		s.Equal("goblin-boss-1", swing.Actor, "swing %d", i)
		s.Equal([]string{"fighter"}, swing.Targets, "swing %d", i)
		s.Equal(refs.Weapons.Scimitar().String(), swing.Attack.Ref,
			"swing %d names the weapon that hit, never the script", i)
		s.Equal("Scimitar", swing.Attack.Name, "swing %d", i)
	}

	first := swings[0].Calculation.Components[0].Dice
	s.Require().NotNil(first)
	s.Len(first.OriginalRolls, 1, "the first swing of a multiattack is an ordinary attack")
	s.Nil(first.Keep, "no rule met on the first pool, and the zero value says so")

	second := swings[1].Calculation.Components[0].Dice
	s.Require().NotNil(second)
	s.Len(second.OriginalRolls, 2, "the second swing rolled a pair")
	s.Len(second.KeptIndices, 1, "and kept one of them")
	s.Require().NotNil(second.Keep, "the beat carries the record of WHY")
	s.Equal("disadvantage", second.Keep.Rule)
	s.Require().Len(second.Keep.Imposed, 1)
	s.Equal("second attack of a multiattack", second.Keep.Imposed[0].Name,
		"the content's own words reach the persisted beat")
	s.Equal(refs.MonsterActions.GoblinBossMultiattack().String(), second.Keep.Imposed[0].Ref,
		"the script is the rule that threw the die")
	s.Equal("goblin-boss-1", second.Keep.Imposed[0].SourceID, "and the boss is whose die it is")

	beats := s.storyBeats(mgr, "fighter")[before:]
	s.Equal("tick", beats[len(beats)-1], "the round wrapped")
	s.Equal("turn-ended", beats[len(beats)-2],
		"the boss's turn closed after one attack, exactly as a single swing's would")
	s.True(out.RoundWrapped)
}

// TestAMultiattackSpendsTheTurnsOneAttack is the encounter-side finding
// stated as a test rather than as a claim in a PR body.
//
// The composition's turn loop sets AttacksLeft to zero after ONE
// Striker.Strike call and refuses any later Attack intent in the same turn
// (clocks.go's Attack arm). A multiattack arrives as one such intent, so a
// driver that asks to attack over and over gets exactly one action's worth of
// swings — the boss's two — and then its turn ends. Nothing about that loop
// needed to learn what a sequence is.
func (s *MonsterTurnTestSuite) TestAMultiattackSpendsTheTurnsOneAttack() {
	ctx := context.Background()
	mgr := s.tombManager(firstInReach{}, testDice{})

	launchScene(s.T(), mgr, bossBesideFighter())

	// firstInReach asks to attack on every view it is given, so a turn loop
	// that granted a sequence a fresh attack each time would swing forever.
	before := len(s.storyBeats(mgr, "fighter"))
	_, err := mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)

	s.Len(s.swingsIn(mgr, "fighter", before), 2,
		"one action's worth of attacking, however many blows that action lands")
}

// TestASequenceStopsWhenTheTargetGoesDown is resolution's stop rule observed
// at the seam that writes the story: a target who drops to the first blow is
// not swung at again, so the second beat is never written.
//
// It matters HERE and not only in resolution because the beat is the visible
// consequence: a machine that swung on would record an attack against a
// creature the same turn had already felled, and the encounter would refuse
// that Record on a run its own noticeDown had just closed.
func (s *MonsterTurnTestSuite) TestASequenceStopsWhenTheTargetGoesDown() {
	ctx := context.Background()

	frail := armedFighter("fighter")
	// One scimitar blow is 1d6+2 and testDice rolls the six. MAX hit points
	// as well as current: a sheet loads at its maximum, so a lowered current
	// value alone would be quietly restored and this test would pass while
	// asserting nothing.
	frail.HitPoints, frail.MaxHitPoints = 6, 6

	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: firstInReach{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(frail), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	launchScene(s.T(), mgr, bossBesideFighter())

	before := len(s.storyBeats(mgr, "fighter"))
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err, "a felling blow must not leave the turn half-recorded")

	swings := s.swingsIn(mgr, "fighter", before)
	s.Require().Len(swings, 1, "the script stopped when there was nothing left to swing at")
	s.Equal("struck", swings[0].Beat)

	beats := s.storyBeats(mgr, "fighter")[before:]
	s.Contains(beats, "down", "the blow that stopped the script is the one that felled her")
	s.NotContains(beats[indexOf(beats, "down"):], "struck",
		"and nothing swung at her afterwards")
}

// bossBesideFighter is the tomb with the fighter on its corner cell and a
// goblin boss on the next cell over, adjacent so both scimitar swings reach.
// They see each other, so the fight forms at launch.
func bossBesideFighter() scene {
	sc := tombRoom(12, 6)
	sc.Party = []sceneSeat{seatAt("fighter", 0, 0)}
	sc.Monsters = []dungeonspec.MonsterPlacement{
		monsterAt("goblin-boss-1", refs.Monsters.GoblinBoss().String(), 1, 0),
	}
	return sc
}

// indexOf is the position of the first beat of a kind, or zero when there is
// none — enough for the one slice above, where the caller has already
// asserted the kind is present.
func indexOf(beats []string, kind string) int {
	for i, beat := range beats {
		if beat == kind {
			return i
		}
	}

	return 0
}

// holdSpellAfterJoin puts a concentration hold on a sheet that is already
// seated, and it has to happen AFTER the launch: the launch's first-admission
// long rest rewrites the sheet in the character store, and a hold authored
// before it is simply gone by the time anybody swings (the rest also refreshes
// hit points to maximum, which is the same trap one floor down).
//
// Only the hold, no child — a passed check strips nothing, so a child would
// be scenery.
func (s *MonsterTurnTestSuite) holdSpellAfterJoin(chars *fakeCharacters, id string) {
	s.T().Helper()

	seated, err := chars.GetCharacter(context.Background(), id)
	s.Require().NoError(err)

	hold := conditions.NewConcentratingCondition(
		id, refs.Spells.TrueStrike().String(), "True Strike", spells.TrueStrikeTurnEnds)
	blob, err := hold.ToJSON()
	s.Require().NoError(err)

	seated.Conditions = append(seated.Conditions, json.RawMessage(blob))
	s.Require().NoError(chars.SaveCharacter(context.Background(), seated))
}

// TestEachSwingsConcentrationCheckLandsBehindItsOwnBeat is Kirk's per-hit
// ruling where the player actually sees it: the story.
//
// A concentration check rides in behind the beat that caused it, so a
// multiattack that forced two checks reads struck, saved, struck, saved — one
// blow, the roll it forced, the next blow, the roll IT forced. The folded
// shape this replaces read struck, struck, saved, saved, which tells the
// player the defender rolled twice at the end of the action and hides which
// blow each roll answered.
func (s *MonsterTurnTestSuite) TestEachSwingsConcentrationCheckLandsBehindItsOwnBeat() {
	ctx := context.Background()

	chars := newFakeCharacters(armedFighter("fighter"))
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: firstInReach{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: chars, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	launchScene(s.T(), mgr, bossBesideFighter())
	s.holdSpellAfterJoin(chars, "fighter")

	before := len(s.storyBeats(mgr, "fighter"))
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)

	// Just the swings and the rolls they forced, in order — the picks and the
	// turn bookends are not what this is about.
	var train []string
	for _, beat := range s.storyBeats(mgr, "fighter")[before:] {
		if beat == "struck" || beat == "missed" || beat == "saved" {
			train = append(train, beat)
		}
	}

	s.Equal([]string{"struck", "saved", "struck", "saved"}, train,
		"each check rides in behind the blow that forced it, never folded onto the end")
}

// TestASwingThatBreaksConcentrationEndsItsAreaInTheSequence: the goblin boss's
// Multiattack lands a blow that breaks the fighter's concentration on a spell
// holding a runtime area, and the finished sequence lands the area it closed.
func (s *MonsterTurnTestSuite) TestASwingThatBreaksConcentrationEndsItsAreaInTheSequence() {
	ctx := context.Background()
	mgr := s.bossBreaksTheFightersArea(false)
	s.Contains(s.storyBeats(mgr, "fighter"), string(encounter.BeatConcentrationEnded), "control: a swing broke the concentration")
	areas, err := mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Empty(areas, "the sequence lands the area its swing closed")
}

// TestAPostHitInsideAMultiattackTellsTheSettledSwingAtThePause is the one
// pause envelope's headline proof. The goblin boss's first swing hits the
// fighter holding Fog Cloud, who fails her concentration save, and the hit
// stops the sequence to ask whether she strikes back with Wrath of the Storm.
// Everything that settled before the pause is told AT the pause — the struck
// beat, the failed save and the concentration ending, in that order — and the
// cloud closes then, behind its cause; the stored window holds the pause and
// nothing about what was told. Declining tells the second swing and nothing
// from the first again.
func (s *MonsterTurnTestSuite) TestAPostHitInsideAMultiattackTellsTheSettledSwingAtThePause() {
	ctx := context.Background()
	// Initiative twice; swing one attacks 15, damage 3, and the fighter's
	// concentration save is a 1; on the resume swing two rolls at
	// disadvantage, 15 and 15, damage 3.
	mgr := s.bossBreaksTheFightersAreaWith(true, &sequenceDice{rolls: []int{10, 10, 15, 3, 1, 15, 15, 3, 20, 20}})
	persisted, err := s.encounters.GetEncounter(ctx, "sess")
	s.Require().NoError(err)
	s.Require().NotNil(persisted.Pause, "control: the sequence paused on the fighter's window")

	atPause := swingTrain(s.storyBeats(mgr, "fighter"))
	s.Equal([]string{"struck", "saved", "concentration_ended"}, atPause,
		"the settled swing, its failed save and the break are told at the pause")
	areas, err := mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Empty(areas, "the cloud closes at the pause, behind the swing that closed it")
	windows := s.sessions.byID["sess"].Windows.Windows
	s.Require().Len(windows, 1, "control: one window stands")
	var stored map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(windows[0].Payload, &stored))
	keys := make([]string, 0, len(stored))
	for key := range stored {
		keys = append(keys, key)
	}
	s.ElementsMatch([]string{"version", "kind", "audience", "pause", "story"}, keys,
		"the window holds the pause and its story, never what was told")

	react := currentDeclaration(s.T(), mgr, "sess", "fighter", session.VerbReact)
	_, err = mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "fighter", DeclarationID: react.ID, Answer: session.Decline()})
	s.Require().NoError(err)
	s.Equal(append(atPause, "struck"), swingTrain(s.storyBeats(mgr, "fighter")),
		"the resume tells the second swing and nothing from the first again")
}

// swingTrain is the swings, saves and concentration endings of a story, in
// order — the beats the envelope's telling is about.
func swingTrain(beats []string) []string {
	var train []string
	for _, beat := range beats {
		switch beat {
		case "struck", "missed", "saved", "concentration_ended":
			train = append(train, beat)
		}
	}
	return train
}

// TestARepausedSequenceLandsWhatItHasTold: the fighter holds back her Wrath,
// so the sequence resumes, its second swing hits, breaks her concentration and
// pauses again. Everything that settled before the second pause — that swing
// and its break — is told at it, and the area closes then: the area and the
// story agree at every pause.
func (s *MonsterTurnTestSuite) TestARepausedSequenceLandsWhatItHasTold() {
	ctx := context.Background()
	// Initiative twice; swing one attack 15, damage 3, a d20 of 20 before the
	// pause; on the resume a 15, then attack 15, damage 3 and a save of 1.
	mgr := s.bossBreaksTheFightersAreaWith(true, &sequenceDice{rolls: []int{10, 10, 15, 3, 20, 15, 15, 3, 1}})
	areas, err := mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Require().NotEmpty(areas, "control: nothing broke before the first pause")

	react := currentDeclaration(s.T(), mgr, "sess", "fighter", session.VerbReact)
	_, err = mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "fighter", DeclarationID: react.ID, Answer: session.Decline()})
	s.Require().NoError(err)
	s.Require().NotEmpty(s.sessions.byID["sess"].Windows.Windows, "control: the sequence paused again")
	s.Require().Contains(s.storyBeats(mgr, "fighter"), string(encounter.BeatConcentrationEnded),
		"control: the resume told the break")
	areas, err = mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Empty(areas, "a told break lands its area at once, even while the sequence waits")
}

// bossBreaksTheFightersArea is the goblin boss's driven Multiattack against a
// fighter concentrating on a spell that holds a runtime area. The area is
// placed on the stored world directly: what is under test is the sequence
// landing the area its swing closed, not how the area came to stand.
func (s *MonsterTurnTestSuite) bossBreaksTheFightersArea(wrath bool) *session.Manager {
	return s.bossBreaksTheFightersAreaWith(wrath, testDice{})
}

// bossBreaksTheFightersAreaWith is bossBreaksTheFightersArea on the given dice.
func (s *MonsterTurnTestSuite) bossBreaksTheFightersAreaWith(wrath bool, roller interface {
	Roll(context.Context, int) (int, error)
}) *session.Manager {
	ctx := context.Background()

	fighter := armedFighter("fighter")
	if wrath {
		raw, err := json.Marshal(features.WrathOfTheStormData{Ref: refs.Features.WrathOfTheStorm(), ID: "wrath", Name: "Wrath of the Storm", CharacterID: fighter.ID})
		s.Require().NoError(err)
		fighter.Features = append(fighter.Features, raw)
		if fighter.Resources == nil {
			fighter.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{}
		}
		fighter.Resources[resources.WrathOfTheStorm] = character.RecoverableResourceData{Current: 3, Maximum: 3, ResetType: coreResources.ResetLongRest}
	}
	chars := newFakeCharacters(fighter)
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: roller, TurnDriver: firstInReach{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: chars, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	launchScene(s.T(), mgr, bossBesideFighter())
	s.holdSpellAfterJoin(chars, "fighter")
	// A frail constitution, so testDice's flat 10 fails the check: CON 6 is -2.
	seated, err := chars.GetCharacter(ctx, "fighter")
	s.Require().NoError(err)
	seated.AbilityScores[abilities.CON] = 6
	s.Require().NoError(chars.SaveCharacter(ctx, seated))
	s.encounters.byID["sess"].SightAreas = append(s.encounters.byID["sess"].SightAreas, encounter.SightAreaData{
		ID: "cloud", SourceID: "fighter", Name: "Fog Cloud",
		Center: encounter.PositionData{X: 9, Y: 4}, RadiusFeet: 10,
	})
	areas, err := mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Require().NotEmpty(areas, "control: the fighter's area stands")

	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)
	return mgr
}

// TestASequenceThatPausesOnItsSecondSwingTellsBoth: the goblin boss's first
// swing misses and its second hits a fighter holding Wrath of the Storm, which
// stops the sequence to ask. Both swings settled before the pause, so both are
// told at it; declining resumes a sequence with nothing left to swing and
// tells neither again.
func (s *MonsterTurnTestSuite) TestASequenceThatPausesOnItsSecondSwingTellsBoth() {
	ctx := context.Background()
	// Initiative twice; swing one attacks with a 1 and misses; swing two
	// rolls at disadvantage, 15 and 15, damage 3, and the fighter's
	// concentration save is a 20.
	mgr := s.bossBreaksTheFightersAreaWith(true, &sequenceDice{rolls: []int{10, 10, 1, 15, 15, 3, 20, 20, 20, 20}})
	persisted, err := s.encounters.GetEncounter(ctx, "sess")
	s.Require().NoError(err)
	s.Require().NotNil(persisted.Pause, "control: the sequence paused on the fighter's window")

	atPause := swingTrain(s.storyBeats(mgr, "fighter"))
	s.Equal([]string{"missed", "struck", "saved"}, atPause, "both settled swings are told at the pause")

	react := currentDeclaration(s.T(), mgr, "sess", "fighter", session.VerbReact)
	_, err = mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "fighter", DeclarationID: react.ID, Answer: session.Decline()})
	s.Require().NoError(err)
	s.Equal(atPause, swingTrain(s.storyBeats(mgr, "fighter")), "the resume tells neither swing again")
}

// TestACompletedSequenceTellsOnlyWhatSettledAfterItsPause: the goblin boss's
// first swing hits, breaks the fighter's concentration and stops to ask about
// Wrath of the Storm, so the swing, the break and the closed area all land at
// the pause. The fighter holds back, the second swing misses and the sequence
// completes; the resume tells only that miss.
func (s *MonsterTurnTestSuite) TestACompletedSequenceTellsOnlyWhatSettledAfterItsPause() {
	ctx := context.Background()
	// Initiative twice; swing one attacks 15, damage 3, and the fighter's
	// concentration save is a 1; on the resume swing two rolls at
	// disadvantage, 1 and 1, and misses.
	mgr := s.bossBreaksTheFightersAreaWith(true, &sequenceDice{rolls: []int{10, 10, 15, 3, 1, 1, 1, 20, 20, 20}})
	areas, err := mgr.Areas(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Require().Empty(areas, "control: the area closed at the pause")
	s.Require().NotEmpty(s.sessions.byID["sess"].Windows.Windows, "control: the sequence paused")
	atPause := swingTrain(s.storyBeats(mgr, "fighter"))

	react := currentDeclaration(s.T(), mgr, "sess", "fighter", session.VerbReact)
	_, err = mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "fighter", DeclarationID: react.ID, Answer: session.Decline()})
	s.Require().NoError(err)

	s.Require().Empty(s.sessions.byID["sess"].Windows.Windows, "control: the sequence completed rather than pausing again")
	s.Equal(append(atPause, "missed"), swingTrain(s.storyBeats(mgr, "fighter")),
		"the resume tells the second swing alone")
}

// TestATurnBoundaryTellsConcentrationItEnded: the fighter holds a
// concentration that lapses at the end of her own turn. Ending the turn
// crosses that boundary, and the boundary has no causing unit to ride behind,
// so the concentration ending is told through the encounter's own verb rather
// than dropped (ruling E6).
func (s *MonsterTurnTestSuite) TestATurnBoundaryTellsConcentrationItEnded() {
	ctx := context.Background()
	chars := newFakeCharacters(armedFighter("fighter"))
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: firstInReach{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: chars, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	launchScene(s.T(), mgr, bossBesideFighter())

	seated, err := chars.GetCharacter(ctx, "fighter")
	s.Require().NoError(err)
	hold := conditions.NewConcentratingCondition("fighter", refs.Spells.TrueStrike().String(), "True Strike", 1)
	blob, err := hold.ToJSON()
	s.Require().NoError(err)
	seated.Conditions = append(seated.Conditions, json.RawMessage(blob))
	s.Require().NoError(chars.SaveCharacter(ctx, seated))

	before := len(s.storyBeats(mgr, "fighter"))
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err, "a boundary that ends concentration lands")

	after := s.storyBeats(mgr, "fighter")[before:]
	s.Contains(after, string(encounter.BeatConcentrationEnded), "the boundary tells the concentration it ended")
	s.NotContains(swingTrain(after), "saved", "and nothing is left to be tested by the blows that follow")
}
