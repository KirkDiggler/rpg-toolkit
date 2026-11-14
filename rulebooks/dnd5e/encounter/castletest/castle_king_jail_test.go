// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_king_jail_test.go is the cut to the cells (rpg-project#925, ruled
// 2026-11-14): at the arrested gate a taken member says "We'll come quietly."
// and the taken are set down in the cells, an off-screen walk of five
// rounds; custody becomes jailed, which holds nothing but the cell's door;
// and an arresting guard who sees one of them out of the cells again turns
// the garrison.

// comeQuietly is the line the arrested gate offers a taken member.
const comeQuietly = "come-quietly"

// jailedScene is [arrestScene] with the hero having gone quietly, ONCE per
// test binary in [TestMain], and the record's reading just before the line.
var jailedScene struct {
	once        sync.Once
	err         error
	blob        []byte
	beforeSeq   uint64
	beforeRound int
}

// jailedSceneTests are the tests that start from [jailedScene].
var jailedSceneTests = []string{
	"TestMadKingGoingQuietlySetsTheTakenDownInTheCells", "TestMadKingTheJailIsOneVerbsTime",
	"TestMadKingEachEndOfTheJailIsToldWhatItSaw", "TestMadKingTheJailedAreHeldByNothing",
	"TestMadKingAnEscapeTurnsTheGarrison", "TestMadKingTheJailIsKeptOnReload",
	"TestMadKingASeizedMemberGoesQuietlyBesideTheOthers", "TestMadKingTheSeizedRefuseAndFight",
}

// goQuietly has member say the arrested gate's line to the captain.
func (s *kingScene) goQuietly(member encounter.MemberID) error {
	s.t.Helper()
	_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: member, Target: kingCaptain, OptionID: comeQuietly, Roller: rollsOne{},
		Revision: s.revisionOf(member, kingCaptain, comeQuietly),
	})
	return err
}

// buildTheJailedScene loads the arrest scene and has the hero go quietly.
func buildTheJailedScene() {
	tb := &fixtureTB{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fixtureFailed); !ok {
				panic(r)
			}
			jailedScene.err = fmt.Errorf("the jailed scene: %s", tb.failed)
		}
	}()
	arrestScene.once.Do(buildTheArrestScene)
	require.NoError(tb, arrestScene.err)
	s := loadTheSceneOn(tb, arrestScene.blob, arrestScene.compiled, arrestScene.host)
	next, err := s.enc.NextStorySeq()
	require.NoError(tb, err)
	jailedScene.beforeSeq, jailedScene.beforeRound = next, s.enc.ToData().Clock.HighWater
	require.True(tb, s.offersTo(hero, kingCaptain, comeQuietly), "precondition: the arrested gate offers the line")
	require.NoError(tb, s.goQuietly(hero))
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
	jailedScene.blob = blob
}

// theJail loads the jailed scene into a fresh run.
func theJail(t *testing.T) *kingScene {
	t.Helper()
	jailedScene.once.Do(buildTheJailedScene)
	require.NoError(t, jailedScene.err)
	return loadTheSceneOn(t, jailedScene.blob, arrestScene.compiled, arrestScene.host)
}

// TestMadKingGoingQuietlySetsTheTakenDownInTheCells: the three taken stand
// in the cells, custody is jailed, the record names them, the gate's three
// hold `jailed` about each, and no player is told that record.
func TestMadKingGoingQuietlySetsTheTakenDownInTheCells(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	require.Equal(t, "jailed", s.entrance())
	made := s.arrested()
	require.Equal(t, encounter.CustodyJailed, made.Custody)
	require.Equal(t, takenMembers, made.Jailed)
	for _, m := range takenMembers {
		require.Equal(t, encounter.RegionID("region-cells"), s.regionOf(s.at(m)), "%s is in the cells", m)
		for _, g := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
			require.True(t, s.recognizesOn(g, m, intel.Topic(encounter.TopicJailed)), "%s holds %s jailed", g, m)
		}
		for _, b := range s.payloadsOf(m, encounter.BeatRecognized) {
			require.NotEqual(t, string(encounter.TopicJailed), b["topic"], "%s is not told the guards' record", m)
		}
	}
	require.Equal(t, s.arrestSceneAt(leftTheGate), s.at(leftTheGate), "the wanted were not taken")
}

// arrestSceneAt is where a member stood in the arrest scene.
func (s *kingScene) arrestSceneAt(id encounter.MemberID) spatial.Position {
	s.t.Helper()
	arrest := loadTheSceneOn(s.t, arrestScene.blob, arrestScene.compiled, arrestScene.host)
	return arrest.at(id)
}

// TestMadKingTheJailIsOneVerbsTime: the jail fires from inside Speak, and its
// move pays the five authored rounds and refreshes sight inside Speak. Time is
// paid once (the plain line pays none of its own), Speak's answer and the
// conversation's step precede every relocated beat, every relocated beat
// precedes the rounds' first tick, and a refresh afterwards finds nothing to
// change.
func TestMadKingTheJailIsOneVerbsTime(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	require.Equal(t, jailedScene.beforeRound+5, s.enc.ToData().Clock.HighWater, "five rounds, paid once")

	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero, AfterSeq: jailedScene.beforeSeq - 1})
	require.NoError(t, err)
	var order []string
	for _, e := range story {
		var b struct {
			Beat string `json:"beat"`
		}
		require.NoError(t, json.Unmarshal(e.Payload, &b))
		order = append(order, b.Beat)
	}
	answered := slices.Index(order, encounter.BeatAnswered)
	stepped := slices.Index(order, encounter.BeatInteraction)
	relocated := slices.Index(order, encounter.BeatRelocated)
	lastRelocated := lastIndex(order, encounter.BeatRelocated)
	tick := slices.Index(order, encounter.BeatTick)
	require.True(t, answered >= 0 && stepped > answered, "the answer, then the conversation's step: %v", order)
	require.Greater(t, relocated, stepped, "then the move: %v", order)
	require.Greater(t, tick, lastRelocated, "and only then the rounds: %v", order)

	before := len(story)
	require.NoError(t, s.enc.RefreshPerception())
	again, err := s.enc.Story(&encounter.StoryInput{Audience: hero, AfterSeq: jailedScene.beforeSeq - 1})
	require.NoError(t, err)
	require.Len(t, again, before, "a second refresh finds nothing to change")
}

// TestMadKingEachEndOfTheJailIsToldWhatItSaw: the hireling — of the party's
// faction, at the gate, not taken — and the friend on the road who was not
// taken are told the taken left and never where; a prisoner in the cells is
// told they arrived and never from where; a knight in the throne room, seeing
// neither end, is told nothing.
func TestMadKingEachEndOfTheJailIsToldWhatItSaw(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	for _, tc := range []struct {
		reader encounter.MemberID
		end    string
		has    string
		hasnt  string
	}{
		{hireling, encounter.RelocatedLeft, "from", "to"},
		{leftTheGate, encounter.RelocatedLeft, "from", "to"},
		{"prisoner-1", encounter.RelocatedArrived, "to", "from"},
	} {
		told := s.payloadsOf(tc.reader, encounter.BeatRelocated)
		require.Len(t, told, len(takenMembers), "%s is told of each taken", tc.reader)
		for _, b := range told {
			require.Equal(t, tc.end, b["end"], tc.reader)
			require.Contains(t, b, tc.has, tc.reader)
			require.NotContains(t, b, tc.hasnt, "%s is never told the other end", tc.reader)
			require.NotContains(t, b, "words", "nor the words")
		}
	}
	require.Empty(t, s.payloadsOf(kingKeep1, encounter.BeatRelocated), "who saw neither end is told nothing")
	for _, m := range takenMembers {
		for _, b := range s.payloadsOf(m, encounter.BeatRelocated) {
			if b["member"] == string(m) {
				require.Equal(t, "The guards march you to the cells.", b["words"], "%s is told the words", m)
			} else {
				require.NotContains(t, b, "words", "%s is not told a friend's words", m)
			}
		}
	}
}

// TestMadKingTheJailedAreHeldByNothing: jailed holds nothing by itself — a
// jailed member steps about the cell as anyone does.
func TestMadKingTheJailedAreHeldByNothing(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	at := s.at(hero)
	require.NoError(t, s.stepOff(hero), "jailed is no hold: the jailed walk")
	require.NotEqual(t, at, s.at(hero))
}

// TestMadKingAnEscapeTurnsTheGarrison is the escape row's WIRING on the
// castle: a jailed hero set down back in the gatehouse, in the gate guards'
// sight and out of the cells, teaches the captain `prisoners-escaped`, the
// garrison turns, and the fight forms. Each row of the rule itself — the
// lock picked from inside and the walk out, seen through the opening, held
// not jailed, already turned, no mind — is proved on a small site
// (escape_internal_test.go); picking the castle's cell lock is
// [TestMadKingPickingTheCellLockEndsTheRunFreed]'s.
func TestMadKingAnEscapeTurnsTheGarrison(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	require.Empty(t, s.learners("prisoners-escaped"), "precondition: nobody has escaped")
	back := spatial.Position{X: -5, Y: 9}
	_, err := s.enc.Relocate(&encounter.RelocateInput{
		Moves: []encounter.Relocation{{Member: hero, To: back}},
		Cause: encounter.RelocationCause{Ref: core.Ref{Module: "test", Type: "shortcuts", ID: "back-at-the-gate"}, Words: "You are back at the gate."},
	})
	require.NoError(t, err, "jailed holds nothing, so the exported verb may set the hero down")
	require.Equal(t, encounter.RegionID("region-gatehouse"), s.regionOf(s.at(hero)), "precondition: out of the cells")
	require.True(t, s.learners("prisoners-escaped")[kingCaptain], "the captain learns of the escape")
	require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty), "the garrison turns")
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "and the fight forms")
}

// TestMadKingTheJailIsKeptOnReload: saved and loaded at jailed, nobody moves,
// nothing is told again, and the line is no longer offered.
func TestMadKingTheJailIsKeptOnReload(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	at := map[encounter.MemberID]spatial.Position{}
	for _, m := range takenMembers {
		at[m] = s.at(m)
	}
	n := len(s.payloadsOf(hero, encounter.BeatRelocated))
	s.roundTrip()
	require.Equal(t, "jailed", s.entrance())
	require.Equal(t, encounter.CustodyJailed, s.custody())
	for _, m := range takenMembers {
		require.Equal(t, at[m], s.at(m))
	}
	require.Len(t, s.payloadsOf(hero, encounter.BeatRelocated), n, "nothing fires again")
	require.False(t, s.offersTo(hero, kingCaptain, comeQuietly), "the jailed gate offers the line no more")
}

// TestMadKingATakenMemberWhoLeftStaysWanted: one of the taken leaves the run
// before the line; the other two are jailed, the one who left is not moved,
// is recorded wanted, and the guards who arrested them hold `wanted` about
// them, told to no player.
func TestMadKingATakenMemberWhoLeftStaysWanted(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	_, err := s.enc.Exit(&encounter.ExitInput{Member: seenOutside})
	require.NoError(t, err)
	require.NoError(t, s.goQuietly(hero))

	made := s.arrested()
	require.Equal(t, []encounter.MemberID{hero, inTheArea}, made.Jailed, "only the present are jailed")
	require.Contains(t, wantedOf(made), seenOutside, "the one who left is wanted")
	for _, g := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
		require.True(t, s.recognizesOn(g, seenOutside, intel.Topic(encounter.TopicWanted)), "%s wants them", g)
	}
	for _, m := range []encounter.MemberID{hero, inTheArea} {
		for _, b := range s.payloadsOf(m, encounter.BeatRecognized) {
			require.NotEqual(t, string(encounter.TopicWanted), b["topic"], "%s is not told", m)
		}
	}
}

// lastIndex is where a kind last appears in a list of beat kinds, or -1.
func lastIndex(kinds []string, kind string) int {
	for i := len(kinds) - 1; i >= 0; i-- {
		if kinds[i] == kind {
			return i
		}
	}
	return -1
}

// TestMadKingTheLineIsOfferedOnlyToTheHeld: "We'll come quietly." is offered
// to a taken member still held; never to a party member at the gate who was
// not taken; and to nobody once a blow on a guard has broken the hold —
// saying it then is refused at the door, the conversation stays arrested,
// nothing is told, and the run saves and loads.
func TestMadKingTheLineIsOfferedOnlyToTheHeld(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	require.True(t, s.offersTo(hero, kingCaptain, comeQuietly), "a held taken member is offered the line")

	gate, err := s.enc.FreeCellsIn("region-gatehouse", 1)
	require.NoError(t, err)
	_, err = s.enc.Relocate(&encounter.RelocateInput{
		Moves: []encounter.Relocation{{Member: neverNear, To: gate[0]}},
		Cause: encounter.RelocationCause{Ref: core.Ref{Module: "test", Type: "shortcuts", ID: "to-the-gate"}, Words: "You are at the gate."},
	})
	require.NoError(t, err)
	require.False(t, s.offersTo(neverNear, kingCaptain, comeQuietly), "a party member who was not taken is never offered it")

	s.swingAs(seenOutside, kingGate2)
	require.Equal(t, encounter.CustodyBroken, s.custody(), "precondition: the blow broke the hold")
	require.Equal(t, "arrested", s.entrance(), "precondition: the conversation is still arrested")
	for _, m := range takenMembers {
		require.False(t, s.offersTo(m, kingCaptain, comeQuietly), "%s is no longer offered it", m)
	}
	before := len(s.payloadsOf(hero, encounter.BeatInteraction))
	require.ErrorIs(t, s.goQuietly(hero), encounter.ErrNoOption, "saying it is refused at the door")
	require.Equal(t, "arrested", s.entrance(), "and the conversation stays arrested")
	require.Len(t, s.payloadsOf(hero, encounter.BeatInteraction), before, "nothing is told")
	s.roundTrip()
	require.Equal(t, "arrested", s.entrance())
}

// TestMadKingARefusedJailChangesNothing: a jail that would refuse — here its
// arrest's cell names no region, edited into the saved run — refuses the line
// whole, before anything is written: the conversation stays arrested, the
// record is as long as it was, custody stays held, and the run saves and
// loads.
func TestMadKingARefusedJailChangesNothing(t *testing.T) {
	t.Parallel()
	arrestScene.once.Do(buildTheArrestScene)
	require.NoError(t, arrestScene.err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(arrestScene.blob, &raw))
	for _, it := range raw["interactions"].([]any) {
		if arrest, ok := it.(map[string]any)["arrest"].(map[string]any); ok {
			arrest["cell"] = "region-nowhere"
		}
	}
	blob, err := json.Marshal(raw)
	require.NoError(t, err)
	s := loadTheSceneOn(t, blob, arrestScene.compiled, arrestScene.host)
	require.True(t, s.offersTo(hero, kingCaptain, comeQuietly), "precondition: the held hero is offered the line")
	next, err := s.enc.NextStorySeq()
	require.NoError(t, err)

	require.ErrorIs(t, s.goQuietly(hero), encounter.ErrBadPlacement, "the jail would refuse, so the line is refused")
	require.Equal(t, "arrested", s.entrance(), "the conversation never stepped")
	after, err := s.enc.NextStorySeq()
	require.NoError(t, err)
	require.Equal(t, next, after, "nothing was written")
	require.Equal(t, encounter.CustodyHeld, s.custody())
	for _, m := range takenMembers {
		require.NotEqual(t, encounter.RegionID("region-cells"), s.regionOf(s.at(m)), "%s was not moved", m)
	}
	s.roundTrip()
	require.Equal(t, "arrested", s.entrance(), "and the run saves and loads")
}

// seizeLeftTheGate sets the one who left the gate down beside the captain,
// within the gate's hail; the refresh that follows seizes them (wanted.go).
func (s *kingScene) seizeLeftTheGate() {
	s.t.Helper()
	captain := s.at(kingCaptain)
	for _, d := range []spatial.Position{{X: -1, Y: 1}, {X: 0, Y: 1}, {X: -1, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 2}, {X: -2, Y: 1}} {
		to := spatial.Position{X: captain.X + d.X, Y: captain.Y + d.Y}
		_, err := s.enc.Relocate(&encounter.RelocateInput{
			Moves: []encounter.Relocation{{Member: leftTheGate, To: to}},
			Cause: encounter.RelocationCause{Ref: core.Ref{Module: "test", Type: "shortcuts", ID: "to-the-gate"}, Words: "You are at the gate."},
		})
		if err == nil {
			break
		}
	}
	require.LessOrEqual(s.t, s.enc.Distance(captain, s.at(leftTheGate)), 2.0, "precondition: beside the captain")
	require.Equal(s.t, encounter.CustodyHeld, takenOf(s.arrested())[leftTheGate].Custody, "precondition: seized on sight and held")
}

// TestMadKingASeizedMemberGoesQuietlyBesideTheOthers: after the first group
// is jailed, the one who left is seen at the gate and seized; they are
// offered "We'll come quietly." again, and saying it re-enters `jailed`,
// whose jail takes only them — the one present and held — onto a fresh cell
// beside the others. A save and load after the seizure re-fires nothing.
func TestMadKingASeizedMemberGoesQuietlyBesideTheOthers(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	first := map[encounter.MemberID]spatial.Position{}
	for _, m := range takenMembers {
		first[m] = s.at(m)
	}
	s.seizeLeftTheGate()
	told := len(s.payloadsOf(leftTheGate, encounter.BeatArrested))
	s.roundTrip()
	require.Len(t, s.payloadsOf(leftTheGate, encounter.BeatArrested), told, "a load seizes nothing again")
	require.True(t, s.heldNow(leftTheGate), "and they are still held")
	require.True(t, s.offersTo(leftTheGate, kingCaptain, comeQuietly), "the seized are offered the line")

	require.NoError(t, s.goQuietly(leftTheGate))
	require.Equal(t, encounter.RegionID("region-cells"), s.regionOf(s.at(leftTheGate)), "set down in the cells")
	for _, m := range takenMembers {
		require.Equal(t, first[m], s.at(m), "%s was not moved again", m)
		require.NotEqual(t, first[m], s.at(leftTheGate), "a fresh cell")
	}
	made := s.arrested()
	require.Contains(t, made.Jailed, leftTheGate)
	require.Equal(t, encounter.CustodyJailed, takenOf(made)[leftTheGate].Custody)
}

// TestMadKingTheSeizedRefuseAndFight: the one seized at the gate says "You
// will never take me alive."; the garrison turns, the fight forms with the
// guard, and the hold breaks.
func TestMadKingTheSeizedRefuseAndFight(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	s.seizeLeftTheGate()
	s.neverAlive(leftTheGate)
	require.Equal(t, encounter.CustodyBroken, takenOf(s.arrested())[leftTheGate].Custody, "the hold is broken")
	require.Equal(t, encounter.CustodyJailed, s.custody(), "the jailed stay jailed")
}

// TestMadKingTheFightLineAtTheArrest: at the first arrest the held speaker
// refuses: the garrison turns, the fight forms, and the hold breaks for every
// member held.
func TestMadKingTheFightLineAtTheArrest(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	s.neverAlive(hero)
	require.Equal(t, encounter.CustodyBroken, s.custody(), "the hold is broken")
	require.False(t, s.heldNow(inTheArea), "for every member held")
}

// neverAlive has member refuse the arrest to the captain, and checks the
// garrison turned and the fight formed with member in it.
func (s *kingScene) neverAlive(member encounter.MemberID) {
	s.t.Helper()
	require.True(s.t, s.offersTo(member, kingCaptain, "never-alive"), "the held are offered the fight line")
	_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: member, Target: kingCaptain, OptionID: "never-alive", Roller: rollsOne{},
		Revision: s.revisionOf(member, kingCaptain, "never-alive"),
	})
	require.NoError(s.t, err)
	require.Equal(s.t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty), "the garrison turns")
	require.Equal(s.t, encounter.ClockTurn, clockKind(s.t, s.enc, member), "the fight forms")
}

// TestMadKingTheSeizedAreHailed: the one who left stands in the gatehouse
// three cells from the nearest gate speaker, outside the gate's hail and not seized; the
// step that brings them within it seizes them, and the gate addresses them
// with the lines the held are offered.
func TestMadKingTheSeizedAreHailed(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	captain := s.at(kingCaptain)
	free, err := s.enc.FreeCellsIn("region-gatehouse", 40)
	require.NoError(t, err)
	var start spatial.Position
	found := false
	nearest := func(c spatial.Position) float64 {
		d := s.enc.Distance(captain, c)
		for _, g := range []encounter.MemberID{kingGate1, kingGate2} {
			d = min(d, s.enc.Distance(s.at(g), c))
		}
		return d
	}
	for _, c := range free {
		if nearest(c) == 3 {
			start, found = c, true
			break
		}
	}
	require.True(t, found, "precondition: a free gatehouse cell three from the nearest gate speaker")
	_, err = s.enc.Relocate(&encounter.RelocateInput{
		Moves: []encounter.Relocation{{Member: leftTheGate, To: start}},
		Cause: encounter.RelocationCause{Ref: core.Ref{Module: "test", Type: "shortcuts", ID: "into-the-gatehouse"}, Words: "You are in the gatehouse."},
	})
	require.NoError(t, err)
	require.Contains(t, wantedOf(s.arrested()), leftTheGate, "precondition: beyond the hail, not seized")

	var next spatial.Position
	found = false
	for _, c := range free {
		if s.enc.Distance(start, c) == 1 && nearest(c) <= 2 {
			next, found = c, true
			break
		}
	}
	require.True(t, found, "precondition: a step into the hail")
	_, err = s.enc.Step(&encounter.StepInput{Member: leftTheGate, To: next, EndWalk: true, Reach: speakReach})
	require.NoError(t, err)
	require.Equal(t, encounter.CustodyHeld, takenOf(s.arrested())[leftTheGate].Custody, "seized on sight")
	addressed, err := s.enc.AddressFor(&encounter.AddressForInput{Member: leftTheGate, Reach: speakReach})
	require.NoError(t, err)
	require.NotNil(t, addressed.Address, "and the gate addresses them")
}

// TestMadKingAnArrestRecordWithCustodyOnTheRecordHolds: the arrest scene's
// record names its custody once, on the record, and no member's own — the
// shape every record had up to encounter v0.210.0; it loads and holds.
func TestMadKingAnArrestRecordWithCustodyOnTheRecordHolds(t *testing.T) {
	t.Parallel()
	arrestScene.once.Do(buildTheArrestScene)
	require.NoError(t, arrestScene.err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(arrestScene.blob, &raw))
	for _, it := range raw["interactions"].([]any) {
		if arrest, ok := it.(map[string]any)["arrest"].(map[string]any); ok {
			require.Equal(t, "held", arrest["custody"])
			for _, tk := range arrest["taken"].([]any) {
				require.NotContains(t, tk.(map[string]any), "custody", "the old shape: no member's own phase")
			}
		}
	}
	s := loadTheSceneOn(t, arrestScene.blob, arrestScene.compiled, arrestScene.host)
	require.True(t, s.heldNow(hero), "the record's phase holds the taken")
}

// TestMadKingStandAsideIsClosedToWhomTheGateSeeks: the captain offers
// "Stand aside or bleed." to a stranger — a party member neither taken nor
// wanted — and not to a held member nor to a wanted one.
func TestMadKingStandAsideIsClosedToWhomTheGateSeeks(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	require.True(t, s.offersTo(neverNear, kingCaptain, "stand-aside"), "a stranger is offered it")
	require.False(t, s.offersTo(hero, kingCaptain, "stand-aside"), "a held member is not")
	require.Contains(t, wantedOf(s.arrested()), leftTheGate, "precondition: still wanted")
	require.False(t, s.offersTo(leftTheGate, kingCaptain, "stand-aside"), "nor a wanted one")
}

// TestMadKingTheFightLineIsShouted: the captain stands down the road, the
// room beside the gatehouse through its open south doorway, beyond a spoken
// line's carry; the held hero refuses to gate-1, whose answer is shouted, so
// the captain catches it, learns of the refusal, and the garrison turns.
func TestMadKingTheFightLineIsShouted(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	road, err := s.enc.FreeCellsIn("room-1-region", 60)
	require.NoError(t, err)
	var far spatial.Position
	found := false
	for _, c := range road {
		if s.enc.Distance(s.at(kingGate1), c) > 8 && s.enc.Distance(s.at(hero), c) > 8 && s.enc.Distance(s.at(hero), c) < 14 {
			far, found = c, true
			break
		}
	}
	require.True(t, found, "precondition: a road cell beyond a spoken line's carry")
	_, err = s.enc.Relocate(&encounter.RelocateInput{
		Moves: []encounter.Relocation{{Member: kingCaptain, To: far}},
		Cause: encounter.RelocationCause{Ref: core.Ref{Module: "test", Type: "shortcuts", ID: "down-the-road"}, Words: "You walk down the road."},
	})
	require.NoError(t, err)

	require.True(t, s.offersTo(hero, kingGate1, "never-alive"))
	_, err = s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: kingGate1, OptionID: "never-alive", Roller: rollsOne{},
		Revision: s.revisionOf(hero, kingGate1, "never-alive"),
	})
	require.NoError(t, err)
	require.True(t, s.learners("refused-arrest")[kingCaptain], "the captain catches the shout")
	require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty), "and the garrison turns")
}
