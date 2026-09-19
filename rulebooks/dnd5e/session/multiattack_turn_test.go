// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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

	_, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "goblin-boss-1", Ref: refs.Monsters.GoblinBoss().String(),
		Position: spatial.Position{X: 1, Y: 0}, // adjacent, so both scimitar swings reach
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed)

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

	_, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "goblin-boss-1", Ref: refs.Monsters.GoblinBoss().String(),
		Position: spatial.Position{X: 1, Y: 0},
	})
	s.Require().NoError(err)

	// firstInReach asks to attack on every view it is given, so a turn loop
	// that granted a sequence a fresh attack each time would swing forever.
	before := len(s.storyBeats(mgr, "fighter"))
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
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

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: firstInReach{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(frail), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "goblin-boss-1", Ref: refs.Monsters.GoblinBoss().String(),
		Position: spatial.Position{X: 1, Y: 0},
	})
	s.Require().NoError(err)

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
// seated, and it has to happen AFTER Join: Join rebuilds the sheet from the
// character store and a condition authored before it is simply gone by the
// time anybody swings (it also refreshes hit points to maximum, which is the
// same trap one floor down).
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
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: firstInReach{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: chars, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)
	s.holdSpellAfterJoin(chars, "fighter")

	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "goblin-boss-1", Ref: refs.Monsters.GoblinBoss().String(),
		Position: spatial.Position{X: 1, Y: 0},
	})
	s.Require().NoError(err)

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
