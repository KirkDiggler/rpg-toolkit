// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_script_test.go is THE QA CASTLE'S SCRIPT (rpg-project#639): the
// castle is the standing QA script, and each piece of it the content stages is
// one test here, so a later edit that breaks a step fails by the step's name.
//
//  1. Start outside, neutral, with guards at the gate door.
//  2. The stranger beside the start, out of the gate's sight, gives the
//     feast, on a line whose success and failure both teach it.
//  3. With the fact, the troupe talks its way in; without it, the line is not
//     offered.
//  4. Without it, sneak at the front door: a voice draws the posted guard off
//     the crossing, and the sneaker crosses on the patrol's blind beat
//     (rpg-project#836, R4 A).
//  5. Sneaking in plain sight is refused (R11, rpg-project#768).
//  6. Fight the gate guards: nobody comes.
//  7. Open the gate: the far room answers.
//
// THE START IS OUTSIDE (the operator's layout, rpg-project#654): open ground
// round the gatehouse, whose one way in is a doorway in its south wall. The
// gate guards and the captain keep the gatehouse. A step that only happens
// in front of them opens the hero inside it, on [gatehouseSeat], where the
// party started before the fold: the walk in from the outside start is its
// own claim, made once by [kingScene.walkIn] in the gatehouse's one-way-in
// test and walked again by the stranger's troupe route. Walking it in every
// step would cost the package a third of its race budget for no new claim.
//
// THE HOST IS THE CASTLE'S, with every creature watching a 120-degree arc,
// the arc the session seeds every monster with: a cone is what the script
// shows, so the script is asked of cones.

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The script's stranger and the faction the file places him in.
const (
	kingPedlar = encounter.MemberID("pedlar")
	townsfolk  = encounter.FactionID("townsfolk")
)

// kingReserve2 is the second of the far room's two guards.
const kingReserve2 = encounter.MemberID("reserve-2")

// The script's cells, dungeon-absolute.
var (
	// beneathTheGate is the gatehouse cell under the gate door: the cell a
	// party stands on to open it, and the one a sneak must reach.
	beneathTheGate = castleCell(0, 7)

	// inTheGatehouse is the gatehouse floor two cells in from its south
	// doorway, in front of the gate guards and the captain: where a party
	// that walks in from the start stands.
	inTheGatehouse = castleCell(0, 11)

	// gatehouseSeat is inTheGatehouse as an offset cell, for
	// [encounter.MemberInput.Position]: where a step that acts in front of
	// the gate opens the hero.
	gatehouseSeat = spatial.Position{X: 0, Y: 11}

	// theCorner is the cell behind the corner wall, in the gatehouse's
	// north-west: out of the gate's sight, so a party that walked in civil
	// declares sneaking there unseen. The gatehouse is otherwise one open
	// room, and a declaration a watcher sees is refused (sneaking R10 as
	// amended by R11, rpg-project#768).
	theCorner = castleCell(-3, 7)

	// besideTheCorner is the floor just south of the corner, where a party
	// waits in the open for the patrol to turn.
	besideTheCorner = castleCell(-3, 8)

	// theDash is the way from the corner to the cell under the gate, one
	// adjacent cell at a time (Step checks no adjacency; the walk does): out
	// below the corner wall's end, then along the gate wall.
	theDash = []spatial.Position{castleCell(-3, 8), castleCell(-2, 8), castleCell(-2, 7), castleCell(-1, 7), castleCell(0, 7)}

	// pastTheGate is the courtyard cell under the gate, past the shut door: a
	// walk toward it ends beneath the gate.
	pastTheGate = castleCell(0, 6)

	// besideTheCrossing is the gatehouse cell west of the one beneath the
	// gate, along the gate wall: where a party stands beside the posted
	// guard (rpg-project#836).
	besideTheCrossing = castleCell(-1, 7)
)

// The gate patrol's posts as the file writes them: each guard stands at a
// door post watching the cell under the gate, then at a beat post watching
// the incomers.
var (
	westDoorPost = castleCell(-1, 8)
	eastDoorPost = castleCell(2, 8)
	westBeatPost = castleCell(0, 10)
	eastBeatPost = castleCell(1, 10)
)

// The script host's passive Perception for the gate's watchers: one each, so
// the DC a sneak roll was asked at names the watcher it was rolled against.
const (
	gate1Passive   = 11
	gate2Passive   = 12
	captainPassive = 13
)

// scriptStealth is a sneaking hero's Stealth for the script: +7 and a 15 on
// every roll, so every roll asked is won.
func scriptStealth() *partyStealth {
	stealth := &partyStealth{
		modifier: map[encounter.MemberID]int{hero: 7},
		d20s:     map[encounter.MemberID][]int{hero: make([]int, 200)},
	}
	for i := range stealth.d20s[hero] {
		stealth.d20s[hero][i] = 15
	}

	return stealth
}

// openTheScript opens the castle with the hero at a cell (offset [col, row])
// and every creature the file compiles watching a 120-degree arc.
func openTheScript(t *testing.T, at spatial.Position, stealth *partyStealth) *kingScene {
	t.Helper()
	arcs := map[encounter.MemberID]int{}
	for _, m := range loadContent(t, kingFixture).Monsters {
		arcs[encounter.MemberID(m.MemberID)] = 120
	}
	host := castleHost{
		at:      at,
		arcs:    arcs,
		passive: map[encounter.MemberID]int{kingGate1: gate1Passive, kingGate2: gate2Passive, kingCaptain: captainPassive},
	}
	if stealth != nil {
		host.resolver = stealth
	}

	return openTheCastleAs(t, host)
}

// stepTo walks the hero toward a free cell and onto it.
func (s *kingScene) stepTo(to spatial.Position) {
	s.t.Helper()
	s.walkTo(to)
	if s.at(hero) != to {
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: true})
		require.NoError(s.t, err, "the hero steps onto %v", to)
	}
	require.Equal(s.t, to, s.at(hero))
}

// declareBehindTheCorner steps the hero to [theCorner] and declares it
// sneaking there, stepping out beside the corner and back while a patrol
// that looks round the corner refuses it: a refused declaration changes
// nothing, so it is simply made again.
func (s *kingScene) declareBehindTheCorner() {
	s.t.Helper()
	s.stepTo(besideTheCorner)
	_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: theCorner, EndWalk: true})
	require.NoError(s.t, err)
	for i := 0; i < 40; i++ {
		_, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: hero, Sneaking: true})
		if err == nil {
			return
		}
		require.ErrorIs(s.t, err, encounter.ErrSeen)
		for _, to := range []spatial.Position{besideTheCorner, theCorner} {
			round := s.enc.ToData().Clock.HighWater
			_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: true})
			require.NoError(s.t, err)
			s.reloadOnRound(round)
		}
	}
	require.FailNow(s.t, "the hero found the corner unwatched")
}

// pace walks the hero out and back between two adjacent cells, a cell per
// step, until a condition holds or the steps run out, and reports whether it
// held.
func (s *kingScene) pace(a, b spatial.Position, steps int, until func() bool) bool {
	s.t.Helper()
	require.Equal(s.t, 1.0, s.enc.Distance(a, b), "pace walks between adjacent cells")
	for i := 0; i < steps; i++ {
		if until() {
			return true
		}
		to := a
		if s.at(hero) == a {
			to = b
		}
		round := s.enc.ToData().Clock.HighWater
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: true})
		require.NoError(s.t, err)
		s.reloadOnRound(round)
	}

	return until()
}

// room is the wall partition a cell lies in: the room its walls close
// (rpg-project#725).
func (s *kingScene) room(cell spatial.Position) encounter.PartitionID {
	s.t.Helper()
	id, ok := s.enc.PartitionAt(cell)
	require.True(s.t, ok, "%v is floor", cell)

	return id
}

// labelOf is the name of the file's authored region that holds a cell — the
// studio's name for the room, which the region carries itself since
// rpg-project#725 — or "" when the cell is in the implicit region, the open
// ground in no room of the castle. The walls make the rooms, and the barracks
// and the armoury are open to the courtyard, so the authored name is what
// tells them apart.
func labelOf(t *testing.T, cell spatial.Position) string {
	t.Helper()
	raw, err := os.ReadFile(kingFixture)
	require.NoError(t, err)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)
	gp := decoded.Spec.Room.Gameplay
	for _, region := range gp.Regions {
		for _, c := range dungeonspec.ExpandRows(region.Cells, "cells", func(p, m string) { t.Fatalf("%s: %s", p, m) }) {
			if float64(c.Q) != cell.X || float64(c.R) != cell.Y {
				continue
			}
			if region.ID == gp.ImplicitRegionID {
				return ""
			}
			return region.Name
		}
	}

	return ""
}

// scriptBeat is the fields of a story beat the script reads.
type scriptBeat struct {
	Beat     string `json:"beat"`
	Creature string `json:"creature"`
	Option   string `json:"option"`
	Say      string `json:"say"`
	Fact     string `json:"fact"`
	Watcher  string `json:"watcher"`
	DC       int    `json:"dc"`
	ID       string `json:"id"`
}

// beats is a member's story, decoded, from a sequence on.
func (s *kingScene) beats(audience encounter.MemberID) []scriptBeat {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: audience})
	require.NoError(s.t, err)
	out := make([]scriptBeat, 0, len(story))
	for _, entry := range story {
		var b scriptBeat
		require.NoError(s.t, json.Unmarshal(entry.Payload, &b))
		out = append(out, b)
	}

	return out
}

// beatsOf is the beats of one kind in a member's story.
func (s *kingScene) beatsOf(audience encounter.MemberID, kind string) []scriptBeat {
	s.t.Helper()
	var out []scriptBeat
	for _, b := range s.beats(audience) {
		if b.Beat == kind {
			out = append(out, b)
		}
	}

	return out
}

// offers is whether a creature offers the hero a line now.
func (s *kingScene) offers(target encounter.MemberID, option string) bool {
	s.t.Helper()
	options, err := s.enc.OptionsFor(hero, target)
	require.NoError(s.t, err)
	for _, o := range options {
		if o.ID == option {
			return true
		}
	}

	return false
}

// heardTheStranger walks the hero to the stranger beside the start and has
// it ask what the bustle is, the check beaten or not.
func (s *kingScene) heardTheStranger(beaten bool) {
	s.t.Helper()
	s.walkTo(s.at(kingPedlar))
	require.Equal(s.t, 1.0, s.enc.Distance(s.at(hero), s.at(kingPedlar)), "the hero stands beside the stranger")
	require.Equal(s.t, "", labelOf(s.t.(*testing.T), s.at(hero)), "outside, in no room of the castle")
	s.said(kingPedlar, "tonight", "persuasion", beaten)
}

// walkIn walks the hero, civil and not sneaking, from wherever it stands
// outside through the gatehouse's south doorway to the gatehouse floor in
// front of the gate, and checks it arrived there with nobody fighting.
func (s *kingScene) walkIn() {
	s.t.Helper()
	s.stepTo(inTheGatehouse)
	require.Equal(s.t, "Gatehouse", labelOf(s.t.(*testing.T), s.at(hero)), "the hero walked into the gatehouse")
	require.Equal(s.t, encounter.ClockWorld, clockKind(s.t.(*testing.T), s.enc, hero), "the civil garrison lets the party walk in")
}

// TestMadKingScriptStartsOutsideNeutral is piece 1: the party starts
// outside the castle, in no room of it, with the garrison and the stranger
// civil; the gate guards and the captain keep the gatehouse between the start
// and the gate door, and a gate guard keeps a post within two cells of it.
func TestMadKingScriptStartsOutsideNeutral(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, partyStart(t), nil)

	start := s.at(hero)
	require.Equal(t, "", labelOf(t, start), "the party starts outside, in no room of the castle")
	outside := s.room(start)
	gatehouse := s.room(inTheGatehouse)
	require.Equal(t, "Gatehouse", labelOf(t, inTheGatehouse), "precondition: the gatehouse")
	require.NotEqual(t, outside, gatehouse, "the gatehouse is a room of its own")
	require.NotEqual(t, outside, s.room(pastTheGate), "the gate door is between the start and the castle")
	require.NotEqual(t, gatehouse, s.room(pastTheGate), "and between the gatehouse and the courtyard")
	require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty))
	require.Equal(t, encounter.StanceNeutral, s.stance(townsfolk, encounter.FactionParty))
	for _, id := range []encounter.MemberID{kingGate1, kingGate2, kingCaptain} {
		require.Equal(t, gatehouse, s.room(s.at(id)), "%s stands in the gatehouse, before the gate", id)
	}
	require.Equal(t, outside, s.room(s.at(kingPedlar)), "the stranger is outside with the party")
	// The QA castle is a checklist, not a search (rpg-project#654): the
	// stranger stands where the party sees him at once.
	require.LessOrEqual(t, s.enc.Distance(start, s.at(kingPedlar)), 3.0, "the stranger stands beside the start")
	require.True(t, s.seenNow(hero)[s.at(kingPedlar)], "the party sees the stranger from the start")

	beside := spatial.Position{X: start.X - 1, Y: start.Y}
	var post spatial.Position
	dwellsAtTheGate := func() bool {
		for _, id := range []encounter.MemberID{kingGate1, kingGate2} {
			if d := dwellOf(t, s.enc, id); d != nil {
				cell := spatial.Position{X: d.Cell.X, Y: d.Cell.Y}
				if s.at(id) == cell && s.enc.Distance(cell, beneathTheGate) <= 2 {
					post = cell
					return true
				}
			}
		}
		return false
	}
	require.True(t, s.pace(start, beside, 24, dwellsAtTheGate), "a gate guard stands a post within two cells of the gate door")
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "the civil garrison forms no fight")
	t.Logf("a gate guard dwells at %v", post)
}

// TestMadKingScriptTheStrangerGivesTheFeast is piece 2: beside the start, the
// stranger answers "What's all the bustle?" with the feast on a won check and
// on a lost one, and the hero knows `feast-tonight` either way. The captain,
// who cannot see past the gatehouse's walls, does not.
func TestMadKingScriptTheStrangerGivesTheFeast(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		beaten bool
		line   string
	}{
		{"won", true, "Feast for the whole court tonight — they’ll want players."},
		{"lost", false, "Feast, tonight. Now leave me be."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := openTheScript(t, partyStart(t), nil)
			require.False(t, s.learners("feast-tonight")[hero], "precondition: the hero does not know")

			s.heardTheStranger(tc.beaten)

			var answered []scriptBeat
			for _, b := range s.beatsOf(hero, encounter.BeatAnswered) {
				if b.Creature == string(kingPedlar) && b.Option == "tonight" {
					answered = append(answered, b)
				}
			}
			require.Len(t, answered, 1, "the stranger answered once")
			require.Equal(t, tc.line, answered[0].Say)
			require.Equal(t, "feast-tonight", answered[0].Fact)
			require.True(t, s.learners("feast-tonight")[hero], "the hero knows")
			require.False(t, s.learners("feast-tonight")[kingCaptain], "the captain did not witness the line")
		})
	}
}

// TestMadKingScriptTheTroupeNeedsTheFeast is piece 3: the captain offers the
// troupe's line only to a hero who knows the feast; after the stranger he
// does, and a troupe that passes is let through with the garrison civil, and
// opens the gate on a far room that comes civil.
func TestMadKingScriptTheTroupeNeedsTheFeast(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, partyStart(t), nil)

	require.False(t, s.offers(kingCaptain, "troupe"), "before the stranger, no troupe")
	_, err := s.speak(kingCaptain, "troupe", "performance", true)
	require.ErrorIs(t, err, encounter.ErrNoOption)

	s.heardTheStranger(false)
	require.True(t, s.offers(kingCaptain, "troupe"), "after the stranger, the troupe")

	s.walkTo(s.at(kingCaptain))
	s.said(kingCaptain, "troupe", "performance", true)
	var lines []string
	for _, b := range s.beatsOf(hero, encounter.BeatAnswered) {
		if b.Creature == string(kingCaptain) && b.Option == "troupe" {
			lines = append(lines, b.Say)
		}
	}
	require.Equal(t, []string{"Through to the hall, then. Don’t dawdle."}, lines)
	require.Empty(t, s.learners("party-recognized"), "a troupe that passes writes nothing")
	require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty))
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "nobody fights")

	// Let through, the troupe opens the gate, and the far room comes civil
	// (piece 7's other half; its test is the fight's).
	s.stepTo(beneathTheGate)
	s.open(kingGateDoor)
	for _, id := range []encounter.MemberID{kingReserve, kingReserve2} {
		require.Contains(t, memberIDsOf(t, s.enc), id, "%s answers the gate", id)
		require.False(t, s.hostile(hero, id), "%s comes civil", id)
	}
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "nobody fights")
}

// TestMadKingScriptASneakWindowAtTheDoor is piece 4, re-aimed by
// rpg-project#836 (R4 A): the gate holds one body and one eye. gate-1 is
// posted on the crossing, the cell beneath the gate, and holds it through
// any number of rounds: a sneaker who dashes for it is stopped by the body,
// seen or not. gate-2 walks the loop, and two rounds of every four stands at
// her beat post with her back to the gate. The sneak is two moves: a voice
// draws gate-1 off the crossing for one round (his post's `heard` row), and
// the sneaker crosses in that round while gate-2 is on her blind beat.
func TestMadKingScriptASneakWindowAtTheDoor(t *testing.T) {
	t.Parallel()

	t.Run("posted, gate-1 holds the crossing every round, and the dash stops at him", func(t *testing.T) {
		t.Parallel()
		s := openTheSneak(t)
		s.declareBehindTheCorner()
		s.stepTo(besideTheCorner)

		// gate2Watches is one round's reading: whether gate-2's zone holds the
		// cell under the gate.
		gate2Watches := func() bool {
			for _, z := range zonesOf(t, s.enc, hero) {
				if z.Watcher == kingGate2 {
					return cellSet(z.Cells)[beneathTheGate]
				}
			}
			return false
		}
		watched, blind, longestBlind := 0, 0, 0
		beside := castleCell(-4, 8)
		for round := 0; round < 12; round++ {
			require.Equal(t, beneathTheGate, s.at(kingGate1), "round %d: gate-1 stands the crossing", round)
			if gate2Watches() {
				watched++
				blind = 0
			} else {
				blind++
				longestBlind = max(longestBlind, blind)
			}
			start := s.enc.ToData().Clock.HighWater
			require.True(t, s.pace(besideTheCorner, beside, 12, func() bool { return s.enc.ToData().Clock.HighWater > start }),
				"a round passes")
		}
		require.Positive(t, watched, "some rounds gate-2 watches the cell under the gate")
		require.GreaterOrEqual(t, longestBlind, 2, "and two rounds running she does not: her blind beat")

		// The dash, posted: every step but the last is taken, and the last
		// is stopped by the body on the crossing.
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: theCorner, EndWalk: true})
		require.NoError(t, err)
		for _, cell := range theDash[:len(theDash)-1] {
			_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell})
			require.NoError(t, err)
		}
		_, err = s.enc.Step(&encounter.StepInput{Member: hero, To: beneathTheGate, EndWalk: true})
		require.ErrorIs(t, err, encounter.ErrBodyInTheWay, "the posted body stops the sneaker as it stops anyone")
		require.Equal(t, theDash[len(theDash)-2], s.at(hero), "the walk stopped one cell short of him")
		require.Equal(t, beneathTheGate, s.at(kingGate1), "and he still holds the crossing")
	})

	t.Run("drawn off by a voice, the sneaker crosses on gate-2's blind beat", func(t *testing.T) {
		t.Parallel()
		s := openTheSneak(t)
		s.declareBehindTheCorner()
		s.stepTo(besideTheCorner)
		// gate-2 has just come to her beat post: she stands there this round
		// and the next, with her back to the gate.
		justArrived := func() bool {
			d := dwellOf(t, s.enc, kingGate2)
			return d != nil && s.at(kingGate2) == eastBeatPost && d.Rounds > 0
		}
		beside := castleCell(-4, 8)
		require.True(t, s.pace(besideTheCorner, beside, 60, justArrived), "gate-2 comes to her beat post")
		require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: gate-1 holds the crossing")

		// The noise: the companion outside asks the stranger about the
		// bustle, and the stranger's answer is a voice the post hears. He
		// answers it on the world's next round, and the sneaker goes then.
		s.voiceOutside()
		start := s.enc.ToData().Clock.HighWater
		require.True(t, s.pace(besideTheCorner, beside, 12, func() bool { return s.enc.ToData().Clock.HighWater > start }),
			"a round passes")
		require.NotEqual(t, beneathTheGate, s.at(kingGate1), "the voice drew gate-1 off the crossing")
		require.Equal(t, eastBeatPost, s.at(kingGate2), "precondition: gate-2 still stands her blind beat")

		for i, cell := range theDash {
			if s.at(hero) == cell {
				continue // the pace may have left the hero on the dash's first cell
			}
			_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: i == len(theDash)-1})
			require.NoError(t, err, "the dash steps onto %v", cell)
			require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "no fight formed")
		}
		require.Equal(t, beneathTheGate, s.at(hero), "the sneaker stands beneath the gate")
		require.True(t, s.enc.Sneaking(hero), "still sneaking")
		for _, z := range zonesOf(t, s.enc, hero) {
			if z.Watcher == kingGate2 {
				require.False(t, cellSet(z.Cells)[beneathTheGate], "gate-2 does not watch the cell under the gate")
			}
		}
	})
}

// openTheSneak opens [openTheScript]'s castle with a companion outside beside
// the stranger, whose word to him makes the noise the sneak needs.
func openTheSneak(t *testing.T) *kingScene {
	t.Helper()
	arcs := map[encounter.MemberID]int{}
	for _, m := range loadContent(t, kingFixture).Monsters {
		arcs[encounter.MemberID(m.MemberID)] = 120
	}

	return openTheCastleAs(t, castleHost{
		at: gatehouseSeat, arcs: arcs, resolver: scriptStealth(),
		passive: map[encounter.MemberID]int{kingGate1: gate1Passive, kingGate2: gate2Passive, kingCaptain: captainPassive},
		players: []encounter.MemberInput{{ID: sneakCompanion, Kind: encounter.KindPlayer, Position: besideTheStranger}},
	})
}

// sneakCompanion is the sneak's second party member, outside beside the
// stranger, and besideTheStranger the cell it stands on (offset).
var (
	sneakCompanion    = encounter.MemberID("companion")
	besideTheStranger = spatial.Position{X: 6, Y: 14}
)

// voiceOutside has the companion ask the stranger about the bustle: the
// stranger answers aloud, a voice the gate post hears, and the world round
// the asking takes is the round the post answers it in.
func (s *kingScene) voiceOutside() {
	s.t.Helper()
	_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: sneakCompanion, Target: kingPedlar, OptionID: "tonight", Skill: "persuasion",
		Beaten: true, DC: 15, Total: 20, Roller: rollsOne{},
	})
	require.NoError(s.t, err, "the stranger answers the companion")
}

// TestMadKingScriptSneakingInPlainSightIsRefused is piece 5: declared in
// the gatehouse, in the gate's sight, sneaking is refused with ErrSeen
// naming the gate's watchers (sneaking R10 as amended by R11,
// rpg-project#768: "if anyone can see you you cannot go into sneaking"): the
// mode does not change, no beat is written, nothing rolls and nobody turns.
func TestMadKingScriptSneakingInPlainSightIsRefused(t *testing.T) {
	t.Parallel()
	stealth := scriptStealth()
	s := openTheScript(t, gatehouseSeat, stealth)
	before := s.enc.ToData().Log.NextSeq

	out, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: hero, Sneaking: true})
	require.ErrorIs(t, err, encounter.ErrSeen)
	require.Nil(t, out)
	for _, watcher := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
		require.ErrorContains(t, err, string(watcher), "the refusal names %s", watcher)
	}

	require.False(t, s.enc.Sneaking(hero), "the mode did not change")
	require.Empty(t, zonesOf(t, s.enc, hero), "not sneaking: no red floor")
	require.Equal(t, before, s.enc.ToData().Log.NextSeq, "no beat")
	require.Empty(t, stealth.asked, "nothing rolled")
	require.Nil(t, s.enc.ToData().SneakVerdicts, "no verdict held")
	require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty), "no stance changed")
	require.Equal(t, encounter.StanceNeutral, s.stance(townsfolk, encounter.FactionParty))
}

// TestMadKingScriptFightBringsNobody is piece 6: the hero spears a gate guard,
// the fight forms at the gate and the gate shouts the alarm; over three rounds
// nobody arrives, the courtyard stays empty, and no creature beyond the gate
// leaves its room.
func TestMadKingScriptFightBringsNobody(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, gatehouseSeat, nil)
	gatehouse := labelOf(t, s.at(hero))
	rooms := map[encounter.MemberID]string{}
	for _, id := range memberIDsOf(t, s.enc) {
		if room := labelOf(t, s.at(encounter.MemberID(id))); room != gatehouse {
			rooms[encounter.MemberID(id)] = room
		}
	}

	s.struck(kingGate1, false)
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "the fight forms at the gate")
	require.True(t, s.learners("alarm-raised")[hero], "the gate shouted the alarm and the hero heard it")

	for i := 0; i < 3; i++ {
		s.endTurn()
	}

	require.Empty(t, s.beatsOf(hero, encounter.BeatArrived), "nobody arrived")
	require.NotContains(t, memberIDsOf(t, s.enc), kingReserve)
	require.NotContains(t, memberIDsOf(t, s.enc), kingReserve2)
	require.Equal(t, "Courtyard", labelOf(t, pastTheGate), "precondition: past the gate is the courtyard")
	for _, id := range memberIDsOf(t, s.enc) {
		require.NotEqual(t, "Courtyard", labelOf(t, s.at(encounter.MemberID(id))), "%s is not in the courtyard", id)
	}
	for id, room := range rooms {
		require.Equal(t, room, labelOf(t, s.at(id)), "%s stayed in the %s", id, room)
	}
}

// TestMadKingScriptOpeningTheGateBringsTheFarRoom is piece 7: the gate door
// teaches `gate-opened`, and the far room's two guards arrive on it in the
// barracks as the garrison's and walk to the gate. After the fight they come
// hostile. (On the troupe route they come civil: piece 3's test.)
func TestMadKingScriptOpeningTheGateBringsTheFarRoom(t *testing.T) {
	t.Parallel()

	// answered opens the gate from beneath it and checks the far room
	// arrived in the barracks, in the garrison, and walked toward the gate
	// within three world rounds.
	answered := func(t *testing.T, s *kingScene) {
		t.Helper()
		require.NotContains(t, memberIDsOf(t, s.enc), kingReserve, "precondition: the far room waits")
		s.stepTo(beneathTheGate)
		s.open(kingGateDoor)
		require.True(t, s.learners("gate-opened")[hero], "the hero who opened the gate knows it")

		arrived := map[string]bool{}
		for _, b := range s.beatsOf(hero, encounter.BeatArrived) {
			arrived[b.ID] = true
		}
		require.Equal(t, "Barracks", labelOf(t, castleCell(-20, 0)))
		posted := map[encounter.MemberID]spatial.Position{}
		for _, id := range []encounter.MemberID{kingReserve, kingReserve2} {
			require.True(t, arrived[string(id)], "%s arrived", id)
			require.Equal(t, "Barracks", labelOf(t, s.at(id)), "%s arrived in the barracks", id)
			require.Equal(t, garrison, s.factionOf(id), "%s answers as the garrison's", id)
			posted[id] = s.at(id)
		}

		inside := castleCell(0, 4)
		closer := func() bool {
			for id, from := range posted {
				if s.enc.Distance(s.at(id), inside) >= s.enc.Distance(from, inside) {
					return false
				}
			}
			return true
		}
		start := s.enc.ToData().Clock.HighWater
		for !closer() && s.enc.ToData().Clock.HighWater < start+3 && clockKind(t, s.enc, hero) == encounter.ClockWorld {
			s.pace(beneathTheGate, castleCell(-1, 7), 1, func() bool { return false })
		}
		require.True(t, closer(), "both walked toward the gate within three world rounds")
	}

	t.Run("after the fight, hostile", func(t *testing.T) {
		t.Parallel()
		s := openTheScript(t, gatehouseSeat, nil)
		s.struck(kingGate1, false)
		require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty), "precondition: the fight turned the garrison")
		s.struck(kingGate1, true)
		s.struck(kingGate2, true)
		s.struck(kingCaptain, true)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "precondition: the gate is unguarded and the fight is over")

		answered(t, s)
		require.True(t, s.hostile(hero, kingReserve), "they come hostile")
		require.True(t, s.hostile(hero, kingReserve2))
	})
}
