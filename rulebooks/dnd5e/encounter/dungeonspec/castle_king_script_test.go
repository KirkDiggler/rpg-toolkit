// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_script_test.go is THE QA CASTLE'S SCRIPT (rpg-project#639): the
// castle is the standing QA script, and each piece of it the content stages is
// one test here, so a later edit that breaks a step fails by the step's name.
//
//  1. Start outside, neutral, with guards at the gate door.
//  2. The stranger around the corner gives the feast, on a line whose success
//     and failure both teach it.
//  3. With the fact, the troupe talks its way in; without it, the line is not
//     offered.
//  4. Without it, sneak at the front door: the gate patrol leaves a window.
//  5. Sneaking in plain sight does nothing.
//  6. Fight the gate guards: nobody comes.
//  7. Open the gate: the far room answers.
//
// THE HOST IS THE CASTLE'S, with every creature watching a 120-degree arc,
// the arc the session seeds every monster with: a cone is what the script
// shows, so the script is asked of cones.

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
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

	// theCorner is the cell behind the corner wall, beside the stranger: out
	// of the gate's sight.
	theCorner = castleCell(-3, 7)

	// besideTheCorner is the floor just south of the corner, where a party
	// waits in the open for the patrol to turn.
	besideTheCorner = castleCell(-3, 8)

	// theDash is the way from the corner to the cell under the gate, a step
	// at a time, along the gate wall.
	theDash = []spatial.Position{castleCell(-3, 8), castleCell(-2, 8), castleCell(-1, 7), castleCell(0, 7)}

	// pastTheGate is the courtyard cell under the gate, past the shut door: a
	// walk toward it ends beneath the gate.
	pastTheGate = castleCell(0, 6)
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

// pace walks the hero out and back between two cells, a cell per step, until
// a condition holds or the steps run out, and reports whether it held.
func (s *kingScene) pace(a, b spatial.Position, steps int, until func() bool) bool {
	s.t.Helper()
	for i := 0; i < steps; i++ {
		if until() {
			return true
		}
		to := a
		if s.at(hero) == a {
			to = b
		}
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: true})
		require.NoError(s.t, err)
	}

	return until()
}

// room is the region a cell lies in: the room its walls close.
func (s *kingScene) room(cell spatial.Position) encounter.RegionID {
	s.t.Helper()
	id, ok := s.enc.RegionAt(cell)
	require.True(s.t, ok, "%v is floor", cell)

	return id
}

// labelOf is the map label of the file's authoring region that holds a
// cell, or "" for none: the studio's name for the room. The compiled field
// names a region by the walls that close it, and the barracks and the armoury
// are open to the courtyard, so the label is what tells them apart.
func labelOf(t *testing.T, cell spatial.Position) string {
	t.Helper()
	raw, err := os.ReadFile(kingFixture)
	require.NoError(t, err)
	type axial struct{ Q, R float64 }
	var doc struct {
		Room struct {
			Scene struct {
				MapLabels []struct {
					ID   string
					Text string
				} `yaml:"mapLabels"`
				AuthoringRegions []struct {
					LabelID  string `yaml:"labelId"`
					Boundary struct {
						Cells []axial
					}
				} `yaml:"authoringRegions"`
			}
		}
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	texts := map[string]string{}
	for _, l := range doc.Room.Scene.MapLabels {
		texts[l.ID] = l.Text
	}
	for _, region := range doc.Room.Scene.AuthoringRegions {
		for _, c := range region.Boundary.Cells {
			if c.Q == cell.X && c.R == cell.Y {
				return texts[region.LabelID]
			}
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

// atPosts reports whether both gate guards dwell on the two given posts.
func (s *kingScene) atPosts(west, east spatial.Position) bool {
	s.t.Helper()
	g1, g2 := dwellOf(s.t.(*testing.T), s.enc, kingGate1), dwellOf(s.t.(*testing.T), s.enc, kingGate2)

	return g1 != nil && g2 != nil &&
		spatial.Position{X: g1.Cell.X, Y: g1.Cell.Y} == west && s.at(kingGate1) == west &&
		spatial.Position{X: g2.Cell.X, Y: g2.Cell.Y} == east && s.at(kingGate2) == east
}

// heardTheStranger walks the hero round the corner to the stranger and has
// it ask what the bustle is, the check beaten or not.
func (s *kingScene) heardTheStranger(beaten bool) {
	s.t.Helper()
	s.stepTo(besideTheCorner)
	s.stepTo(theCorner)
	s.said(kingPedlar, "tonight", "persuasion", beaten)
}

// TestMadKingScriptStartsOutsideNeutral is piece 1: the party starts before
// the gate, outside the castle, with the garrison and the stranger civil, and
// a gate guard keeps a post within two cells of the gate door.
func TestMadKingScriptStartsOutsideNeutral(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, partyStart(t), nil)

	start := s.at(hero)
	require.Equal(t, "Before the Gate", labelOf(t, start), "the party starts before the gate")
	gate := s.room(start)
	require.NotEqual(t, gate, s.room(pastTheGate), "the gate door is between the start and the castle")
	require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty))
	require.Equal(t, encounter.StanceNeutral, s.stance(townsfolk, encounter.FactionParty))
	for _, id := range []encounter.MemberID{kingGate1, kingGate2, kingCaptain} {
		require.Equal(t, gate, s.room(s.at(id)), "%s stands before the gate", id)
	}

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

// TestMadKingScriptTheStrangerGivesTheFeast is piece 2: round the corner, the
// stranger answers "What's all the bustle?" with the feast on a won check and
// on a lost one, and the hero knows `feast-tonight` either way. The captain,
// who cannot see round the corner, does not.
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

// TestMadKingScriptASneakWindowAtTheDoor is piece 4: the gate patrol stands
// at its door posts watching the cell under the gate, then at its beat posts
// with its back to it. The window is the beat: a hero who declares behind the
// corner and crosses to the gate then is rolled against by nobody, and no
// zone holds the cell under the gate; the same crossing while the door posts
// are kept is rolled against both gate guards, whose zones both hold it.
func TestMadKingScriptASneakWindowAtTheDoor(t *testing.T) {
	t.Parallel()

	// dash declares the hero behind the corner once both gate guards dwell on
	// the given posts, then walks it to the cell under the gate.
	dash := func(t *testing.T, west, east spatial.Position) (*kingScene, *partyStealth, int) {
		t.Helper()
		stealth := scriptStealth()
		s := openTheScript(t, partyStart(t), stealth)
		s.stepTo(besideTheCorner)
		require.True(t, s.pace(besideTheCorner, castleCell(-4, 8), 60, func() bool { return s.atPosts(west, east) }),
			"the gate guards come to the posts")
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: theCorner, EndWalk: true})
		require.NoError(t, err)
		s.declare(hero)
		_, judged := s.enc.ToData().SneakVerdicts[hero][kingGate1]
		require.False(t, judged, "behind the corner gate-1 does not see the hero declare")
		_, judged = s.enc.ToData().SneakVerdicts[hero][kingGate2]
		require.False(t, judged, "nor gate-2")
		_, judged = s.enc.ToData().SneakVerdicts[hero][kingCaptain]
		require.False(t, judged, "nor the captain")

		snuck := len(s.beatsOf(hero, encounter.BeatSnuck))
		for _, cell := range theDash {
			_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell})
			require.NoError(t, err)
			require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "no fight formed")
		}
		require.Equal(t, beneathTheGate, s.at(hero))

		return s, stealth, snuck
	}

	t.Run("in the window nothing rolls", func(t *testing.T) {
		t.Parallel()
		s, stealth, before := dash(t, westBeatPost, eastBeatPost)
		require.Empty(t, stealth.asked, "nobody rolled against the hero")
		require.Len(t, s.beatsOf(hero, encounter.BeatSnuck), before, "no snuck beat")
		for _, z := range zonesOf(t, s.enc, hero) {
			require.False(t, cellSet(z.Cells)[beneathTheGate], "%s does not watch the cell under the gate", z.Watcher)
		}
	})

	t.Run("at the door posts the crossing rolls against both gate guards", func(t *testing.T) {
		t.Parallel()
		s, stealth, before := dash(t, westDoorPost, eastDoorPost)
		dcs := map[int]bool{}
		for _, in := range stealth.asked {
			dcs[in.Approaches[0].DC] = true
		}
		require.True(t, dcs[gate1Passive], "rolled against gate-1")
		require.True(t, dcs[gate2Passive], "rolled against gate-2")
		watchers := map[string]bool{}
		for _, b := range s.beatsOf(hero, encounter.BeatSnuck)[before:] {
			watchers[b.Watcher] = true
		}
		require.True(t, watchers[string(kingGate1)], "a snuck beat names gate-1")
		require.True(t, watchers[string(kingGate2)], "a snuck beat names gate-2")
		for _, guard := range []encounter.MemberID{kingGate1, kingGate2} {
			var zone map[spatial.Position]bool
			for _, z := range zonesOf(t, s.enc, hero) {
				if z.Watcher == guard {
					zone = cellSet(z.Cells)
				}
			}
			require.True(t, zone[beneathTheGate], "%s watches the cell under the gate from its door post", guard)
		}
	})
}

// TestMadKingScriptSneakingInPlainSightDoesNothing is piece 5: declared at the
// party start, inside the gate's zones, sneaking appends no beat, rolls
// nothing and turns nobody (sneaking R10: a declaration in plain view is a lost
// verdict, unrolled).
func TestMadKingScriptSneakingInPlainSightDoesNothing(t *testing.T) {
	t.Parallel()
	stealth := scriptStealth()
	s := openTheScript(t, partyStart(t), stealth)
	before := s.enc.ToData().Log.NextSeq

	out, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: hero, Sneaking: true})
	require.NoError(t, err)
	require.True(t, out.Sneaking)
	require.Nil(t, out.Formed)

	inZone := false
	for _, z := range zonesOf(t, s.enc, hero) {
		inZone = inZone || cellSet(z.Cells)[s.at(hero)]
	}
	require.True(t, inZone, "precondition: the party start is inside a watcher's zone")

	require.Equal(t, before, s.enc.ToData().Log.NextSeq, "no beat")
	require.Empty(t, stealth.asked, "nothing rolled")
	require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty), "no stance changed")
	require.Equal(t, encounter.StanceNeutral, s.stance(townsfolk, encounter.FactionParty))
	for watcher, v := range s.enc.ToData().SneakVerdicts[hero] {
		require.False(t, v.Beaten, "%s holds a lost verdict", watcher)
		require.False(t, v.Armed, "%s's pair does not roll", watcher)
	}
}

// TestMadKingScriptFightBringsNobody is piece 6: the hero spears a gate guard,
// the fight forms at the gate and the gate shouts the alarm; over three rounds
// nobody arrives, the courtyard stays empty, and no creature beyond the gate
// leaves its room.
func TestMadKingScriptFightBringsNobody(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, partyStart(t), nil)
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
			s.pace(beneathTheGate, castleCell(-1, 8), 1, func() bool { return false })
		}
		require.True(t, closer(), "both walked toward the gate within three world rounds")
	}

	t.Run("after the fight, hostile", func(t *testing.T) {
		t.Parallel()
		s := openTheScript(t, partyStart(t), nil)
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
