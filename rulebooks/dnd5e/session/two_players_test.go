// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TwoPlayersTestSuite is rpg-project#257 slice 3's own gate: the stream has
// never been proven whole with two real humans in one session. Every prior
// wave's own fixtures (MonsterTurnTestSuite, MoveTurnClockSuite,
// TestRoundTwoStruckReachesTheLiveSubscriber) join at most one real player
// — nothing before this suite has ever had two joined characters, each with
// their own story, watching the same fight at once.
//
// armedBarbarian ("barbarian") joins alongside armedFighter ("fighter") —
// the design doc's own two-browser walk (rpg-project's ideas/stream-whole
// design.md on the origin/ideas/stream-whole branch, §6), reduced to what
// each member's story can pin. skel-1 is spawned in plain sight of both and
// pulls them into one bubble; skel-2 is spawned behind a genuine
// LOS+movement wall, matching
// TestBlindSkeletonBehindAWallNeverJoinsTheFight's own fixture shape, and
// never joins.
//
// Both players are given 100 HP (armedFighter's own 24/28 raised the same
// way TestRoundTwoStruckReachesTheLiveSubscriber's own fixture is: "high hit
// points keep two ordinary rounds of being hit from wandering into a
// defeat-ends-the-fight scene by accident") — deliberately, since this
// suite's own killing-blow case is written separately and this suite's own
// turn-order walk must stay clear of a defect found while writing this
// suite (reported to team-lead separately, not yet issue-numbered): a
// driven monster that downs one of two players without deciding the
// fight re-enters the drive loop for an undocumented second turn. NOT
// exercised here on purpose.
type TwoPlayersTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
	// launched is the one launch SetupTest made: both players and both
	// skeletons on the board at once.
	launched *session.LaunchOutput
}

func TestTwoPlayersSuite(t *testing.T) {
	suite.Run(t, new(TwoPlayersTestSuite))
}

// armedBarbarian is armedFighter's own twin, wearing the other class this
// suite needs to prove two DIFFERENT real players rather than two copies of
// the same one — everything else about the sheet (weapon, proficiency,
// ability scores, effective AC) stays identical on purpose, so the fight's
// own arithmetic is exactly duelAC's, proven once and not re-derived here.
func armedBarbarian(id string) *character.Data {
	c := armedFighter(id)
	setClass(c, classes.Barbarian)
	return c
}

// twoPlayerTomb is one open hall wide enough to close a few cells of
// distance in, walled solid between authored columns atX and atX+1 exactly
// as TestBlindSkeletonBehindAWallNeverJoinsTheFight's own fixture is — the
// same hexSeamWalls every seam in this package is drawn with, reused rather
// than re-derived so a wall in this suite means what it already means
// everywhere else.
func twoPlayerTomb(width, height, atX int) scene {
	return scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, width, height)},
			Walls:   hexSeamWalls(atX+1, height, -1),
		},
		Endings: []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
	}
}

func (s *TwoPlayersTestSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()

	fighter := armedFighter("fighter")
	fighter.HitPoints, fighter.MaxHitPoints = 100, 100
	barbarian := armedBarbarian("barbarian")
	barbarian.HitPoints, barbarian.MaxHitPoints = 100, 100

	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(fighter, barbarian),
	})
	s.Require().NoError(err)
	s.mgr = mgr

	world := twoPlayerTomb(12, 6, 6)
	world.Party = []sceneSeat{seatAt("fighter", 0, 0), seatAt("barbarian", 1, 0)}
	world.Monsters = []dungeonspec.MonsterPlacement{
		skeletonAt("skel-1", 3, 0),
		skeletonAt("skel-2", 8, 0), // past the wall at column 6
	}
	s.launched = launchScene(s.T(), mgr, world)
}

// storyMarks is where each named member's story stands now: the last record
// sequence they witnessed, for [toldSince] to read after.
func storyMarks(t *testing.T, mgr *session.Manager, members ...string) map[string]uint64 {
	t.Helper()
	marks := make(map[string]uint64, len(members))
	for _, who := range members {
		marks[who] = lastSeq(storyBeats(t, mgr, "sess", who))
	}
	return marks
}

// toldSince is every beat who witnessed after marks were taken.
func toldSince(t *testing.T, mgr *session.Manager, marks map[string]uint64, who string) []witnessed {
	t.Helper()
	return storyBeatsAfter(t, mgr, "sess", who, marks[who])
}

// struckBeat is a landed attack's payload, as far as these scenes read it.
type struckBeat struct {
	Actor   string   `json:"actor"`
	Targets []string `json:"targets"`
}

// beatStruck is the encounter's word for an attack that landed.
var beatStruck = string(encounter.OutcomeStruck)

// TestTwoPlayersOneSession is the proof itself (rpg-project#257 slice 3,
// design rpg-project#258 §4): fighter and barbarian, each with their own
// story, walking two full rounds
// against a driven skeleton while a second, walled-off skeleton never hears
// a thing worth keeping quiet about.
func (s *TwoPlayersTestSuite) TestTwoPlayersOneSession() {
	ctx := context.Background()

	s.Require().Len(s.launched.Formed, 1,
		"skel-1 is in plain sight of both players and must pull them into one bubble, and the only one")
	// testDice{}'s flat rolls tie every initiative roll, so the order falls
	// to the ID tie-break the seam documents (initiative.go) — alphabetical,
	// same as TestSkeletonAttacksFromRange's own "fighter's ID sorts first"
	// comment. "barbarian" < "fighter" < "skel-1", so the REAL order this
	// dice+ID combination produces is barbarian first, not the fighter-first
	// prose a walkthrough might expect — asserted here rather than fought.
	s.Equal([]string{"barbarian", "fighter", "skel-1"}, s.launched.Formed[0].Order,
		"and a wall in the way keeps skel-2 out of it: a placement, not an ambush")

	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "skel-2"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "skel-2 never joins the fight it cannot perceive")

	// --- (d): a player's Move/Attack on the other player's turn ---
	//
	// It is barbarian's turn (first in the real order). fighter tries to act
	// anyway, on both verbs the brief names, and neither may record
	// anything: a refusal this early must not have touched the story.
	players := []string{"fighter", "barbarian"}
	beforeRefusal := storyMarks(s.T(), s.mgr, players...)

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{{X: 0, Y: 1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotYourTurn)

	_, err = s.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "fighter", Target: "skel-1"})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotYourTurn)

	for _, who := range players {
		s.Empty(toldSince(s.T(), s.mgr, beforeRefusal, who), "a refusal this early must tell %s nothing at all", who)
	}

	// --- (c): two full rounds, turn order and Next walked exactly ---
	roundsStart := storyMarks(s.T(), s.mgr, "fighter", "barbarian", "skel-2")

	// Round 1, barbarian: passes her own turn (a player's turn needs no
	// declared action before EndTurn — the same convention every prior
	// suite's own driven-turn gates use).
	out1, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "barbarian", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "barbarian")})
	s.Require().NoError(err)
	s.Equal("fighter", out1.Next)
	s.False(out1.RoundWrapped)

	// Round 1, fighter: her own end drives skel-1's WHOLE turn inside this
	// one call — using its shortbow from range against barbarian (the closer
	// of the two), then wrapping the round
	// back to barbarian, exactly as TestSkeletonAttacksFromRange's own
	// single-player shape does with a second real player added.
	beforeRound1Drive := storyMarks(s.T(), s.mgr, "fighter")
	out2, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "fighter")})
	s.Require().NoError(err)
	s.Equal("barbarian", out2.Next)
	s.True(out2.RoundWrapped, "skel-1 was last in the order — its own end wraps the round")

	round1Drive := toldSince(s.T(), s.mgr, beforeRound1Drive, "fighter")
	// The fiction of the round, named rather than counted: her turn ending,
	// the skeleton's swing, the skeleton's turn ending.
	//
	// THIS USED TO BE A COUNT OF THREE. A driven turn now writes the roll that
	// chose each of its intents (`answered`) and a wrapping round raises the
	// world clock (`tick`), both of which are rpg-project#465 working — so a
	// bare length would have to be bumped to seven and would then be pinning
	// how many intents a turn takes, which is not what this test is about.
	s.Require().Subset(beatNames(round1Drive),
		[]string{encounter.BeatTurnEnded, beatStruck},
		"turn-ended(fighter), struck(skel-1), turn-ended(skel-1) — the shortbow needs no approach")

	// Round 2, barbarian: same shape as round 1's own opening.
	out3, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "barbarian", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "barbarian")})
	s.Require().NoError(err)
	s.Equal("fighter", out3.Next)
	s.False(out3.RoundWrapped)

	// Round 2, fighter: skel-1 remains inside shortbow range, so this
	// round's driven turn is again a bare strike with no move beat.
	beforeRound2Drive := storyMarks(s.T(), s.mgr, "fighter")
	out4, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "fighter")})
	s.Require().NoError(err)
	s.Equal("barbarian", out4.Next)
	s.True(out4.RoundWrapped)

	round2Drive := toldSince(s.T(), s.mgr, beforeRound2Drive, "fighter")
	s.Require().Subset(beatNames(round2Drive),
		[]string{encounter.BeatTurnEnded, beatStruck},
		"round two is round one's shape: a bare strike with no move beat")

	// Each player sees the skeleton's struck beats, not only her own turn
	// ending — the actual content check behind (a)'s "each sees the other's"
	// clause, independent of the audience-scoping question below.
	for _, who := range players {
		s.NotEmpty(beatsNamed(toldSince(s.T(), s.mgr, roundsStart, who), beatStruck),
			"%s must see skel-1's struck beats", who)
	}

	// --- (a), as a documented negative control ---
	//
	// NOT what a perception-scoped audience would do. skel-2 is walled off with no line of sight to the fight (confirmed
	// above: ClockOf never leaves ClockWorld) and TODAY still receives every
	// beat of it: Record, appendClockBeat and appendMovementBeat
	// (rulebooks/dnd5e/encounter) address every beat to rosterIDs() — every
	// CURRENT member of the whole encounter, unconditionally, with no sight,
	// LOS, or bubble-membership check anywhere in that path. That is
	// rpg-toolkit#940 ("story beats are addressed to every member regardless
	// of perception"), open, and docs/ideas/session-sdk/scenes.md's own
	// Scene 7 names the identical gap for a human two rooms away. When #940
	// lands and audiences become perception-scoped, skel-2's own count below
	// flips to zero and THAT failure is the reminder to update this test —
	// not a change made speculatively here.
	s.NotEmpty(toldSince(s.T(), s.mgr, roundsStart, "skel-2"),
		"TODAY the composition addresses every beat to every member (rpg-toolkit#940, rpg-project#257); "+
			"when audiences are perception-scoped this becomes Empty")

	// --- (e): the driven killing-blow case, two players ---
	// Covered in TestDrivenKillingBlowDissolvesCleanlyWithTwoPlayers, its own
	// test method — deliberately not folded into this walk, since forcing a
	// down here would risk this suite's own turn-order assertions above
	// wandering into the newly found double-turn defect (reported separately) by accident.
}

// TestDrivenKillingBlowDissolvesCleanlyWithTwoPlayers is (e): #1202's own
// dissolved-bubble case, reproduced with two real players rather than one —
// the shape unreachable before rpg-toolkit#1207's own fix, since a driven
// strike that downs a teammate without deciding the fight used to hand the
// SAME monster an undocumented second turn (see two_players_test.go's own
// suite doc). Held as its own standalone test rather than a case inside
// TestTwoPlayersOneSession precisely because it needs low HP to force both
// downs, which that suite's own survival walk deliberately avoids.
//
// fighter and barbarian both start at 8 HP — one hit (8 damage under the
// lawful testDice{}, a d6 rolling its face plus the shortsword's 2, verified
// the same way every other fixture in this package is) downs either outright.
// skel-1 is placed adjacent to barbarian, one cell short of fighter.
//
// Round 1: barbarian's own turn passes. Fighter's own EndTurn drives skel-1,
// whose ONE attack downs barbarian — the fight is NOT decided (fighter still
// stands) — and skel-1's own turn ends normally, wrapping back to fighter.
// This is the exact scenario rpg-toolkit#1207 fixed: without it, skel-1
// would have gone on to a second, undocumented turn and struck fighter too,
// right here.
//
// Round 2: fighter's own EndTurn drives skel-1 again. skel-1's ONE attack
// this time downs fighter — the last player standing — deciding the fight:
// #1202's own dissolved-bubble path fires, EndTurnOutput.Next reports the
// CALLER (fighter) rather than a bubble that no longer exists, and both
// players — barbarian already down, fighter just now — are told the fight
// ended.
func TestDrivenKillingBlowDissolvesCleanlyWithTwoPlayers(t *testing.T) {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	fighter := armedFighter("fighter")
	// One hit downs either side: the lawful damage die caps the skeleton's
	// shortsword at 6 + 2, so 8 hit points is the low-HP fixture the scene
	// has always needed from the dice.
	fighter.HitPoints, fighter.MaxHitPoints = 8, 8
	barbarian := armedBarbarian("barbarian")
	barbarian.HitPoints, barbarian.MaxHitPoints = 8, 8

	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: sessions, Encounters: encounters,
		Characters: newFakeCharacters(fighter, barbarian),
	})
	require.NoError(t, err)

	ctx := context.Background()
	sc := tombWith(12, 6, skeletonAt("skel-1", 2, 0))
	sc.Party = append(sc.Party, seatAt("barbarian", 1, 0))
	spawned := launchScene(t, mgr, sc)
	require.Len(t, spawned.Formed, 1)
	require.Equal(t, []string{"barbarian", "fighter", "skel-1"}, spawned.Formed[0].Order,
		"same tie-break as TestTwoPlayersOneSession's own — barbarian sorts first")

	// Round 1: barbarian passes her own turn.
	out1, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "barbarian", DeclarationID: currentEndTurnID(t, mgr, "sess", "barbarian")})
	require.NoError(t, err)
	require.Equal(t, "fighter", out1.Next)

	// Round 1: fighter's own end drives skel-1's WHOLE turn — the fix under
	// test is that this is exactly ONE attack, not two.
	beforeRound1 := storyMarks(t, mgr, "fighter", "barbarian", "skel-1")
	out2, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(t, mgr, "sess", "fighter")})
	require.NoError(t, err)
	require.Equal(t, "barbarian", out2.Next,
		"a Dying player retains the initiative slot for an explicit save")
	require.True(t, out2.RoundWrapped)

	round1 := func(who string) []witnessed { return toldSince(t, mgr, beforeRound1, who) }
	require.NotEmpty(t, round1("fighter"))

	round1Struck := beatsNamed(round1("fighter"), beatStruck)
	require.Len(t, round1Struck, 1, "the fix: skel-1's own turn is exactly one attack, not two")
	var body struckBeat
	round1Struck[0].decode(t, &body)
	require.Equal(t, []string{"barbarian"}, body.Targets)
	require.Equal(t, "skel-1", body.Actor)

	// Fighter — the survivor — was never touched this round: not struck,
	// still standing, still in the fight.
	for _, who := range []string{"fighter", "barbarian"} {
		for _, b := range beatsNamed(round1(who), beatStruck) {
			var struck struckBeat
			b.decode(t, &struck)
			require.NotContains(t, struck.Targets, "fighter", "fighter must not be attacked in round 1 — that is rpg-toolkit#1207's own defect")
		}
	}
	clockOf, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, clockOf.Clock, "fighter is alive and the fight is still running")

	// Both players receive the combat beats. Fighter additionally observes
	// barbarian's fall, and HEARS the strike that caused it — a clang at
	// DC 10 in her own room, and her sheet's passive 11 reaches it
	// (rpg-project#569). Barbarian receives neither: no sight testimony about
	// self, and the blow that made the sound downed her before it landed, so
	// she was not standing to hear it. What she does see is the skeleton turn
	// toward her to swing (sneaking R13).
	heardBy := map[string]int{}
	for _, who := range []string{"fighter", "barbarian", "skel-1"} {
		if n := len(beatsNamed(round1(who), encounter.BeatHeard)); n > 0 {
			heardBy[who] = n
		}
	}
	require.Equal(t, map[string]int{"fighter": 1}, heardBy, "the clang reaches the one standing listener")
	// Sightings are compared by who they name rather than by count: the
	// skeleton turns toward barbarian to swing (sneaking R13), so BOTH
	// watchers are told to look at it again, and fighter's one beat also
	// names barbarian's fall.
	// One beat per recipient (the batching invariant): fighter's single
	// beat names both barbarian's fall and the skeleton's turn.
	sightedBy := func(recipient string) [][]string {
		var beats [][]string
		for _, b := range beatsNamed(round1(recipient), encounter.BeatSighted) {
			var sighted struct {
				Changed []string `json:"changed"`
			}
			b.decode(t, &sighted)
			beats = append(beats, sighted.Changed)
		}
		return beats
	}
	fighterSighted := sightedBy("fighter")
	require.Len(t, fighterSighted, 1, "fighter's witness update arrives as ONE sighted beat")
	require.ElementsMatch(t, []string{"barbarian", "skel-1"}, fighterSighted[0],
		"fighter sees barbarian fall and the skeleton turn, in one beat")
	// Pins rpg-toolkit#2074's CURRENT behaviour, not a ruling: a downed
	// member still sights, while hearing skips her. Whichever way #2074 is
	// ruled, this assertion moves with it.
	require.Equal(t, [][]string{{"skel-1"}}, sightedBy("barbarian"),
		"downed barbarian still sees the skeleton turn on her (rpg-toolkit#2074), and never "+
			"testifies about herself")
	withoutSighted := func(names []string) []string {
		return slices.DeleteFunc(names, func(n string) bool { return n == encounter.BeatSighted })
	}
	fighterNames := withoutSighted(beatNames(round1("fighter")))
	at := slices.Index(fighterNames, encounter.BeatHeard)
	require.GreaterOrEqual(t, at, 0, "fighter received a heard beat")
	fighterNames = slices.Delete(fighterNames, at, at+1)
	require.ElementsMatch(t, fighterNames, withoutSighted(beatNames(round1("barbarian"))),
		"barbarian receives every beat fighter does but the clang")

	// Barbarian now owns the explicit save turn instead of being silently
	// spliced. An ordinary success reports END_TURN, then her declared End Turn
	// hands control to fighter.
	save, err := mgr.DeathSave(ctx, &session.DeathSaveInput{
		Session: "sess", Member: "barbarian",
		DeclarationID: currentDeclaration(t, mgr, "sess", "barbarian", session.VerbDeathSave).ID,
	})
	require.NoError(t, err)
	require.Equal(t, session.DeathSaveContinuationEndTurn, save.Continuation)
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "barbarian",
		DeclarationID: currentEndTurnID(t, mgr, "sess", "barbarian"),
	})
	require.NoError(t, err)

	// Round 2: fighter's own end drives skel-1 a second time. This time
	// skel-1's one attack downs fighter too — the party now has no Conscious
	// member, so supplied PartyDefeated closes the fight mid-drive.
	beforeRound2 := storyMarks(t, mgr, "fighter", "barbarian")
	out3, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(t, mgr, "sess", "fighter")})
	require.NoError(t, err, "a driven turn that wins the fight must not abort the caller's own EndTurn")
	require.Equal(t, "fighter", out3.Next,
		"there is no bubble left to name a turn IN; EndTurnOutput.Next reports the caller instead (rpg-toolkit#1202)")
	require.True(t, out3.RoundWrapped,
		"the retained Dying slot made round 2 begin before the final driven settlement")

	round2Fighter := toldSince(t, mgr, beforeRound2, "fighter")
	round2Struck := beatsNamed(round2Fighter, beatStruck)
	require.Len(t, round2Struck, 1, "skel-1's own killing blow is still exactly one attack")
	var body2 struckBeat
	round2Struck[0].decode(t, &body2)
	require.Equal(t, []string{"fighter"}, body2.Targets)

	fightEnded := beatsNamed(round2Fighter, encounter.BeatFightEnded)
	require.Len(t, fightEnded, 1, "fighter — the caller — hears the fight end")
	require.Equal(t, string(session.DissolveByDefeat), fightEnded[0].Fields["cause"])

	// Both players get fight_ended — barbarian, already down since round 1,
	// hears it too (rpg-toolkit#940's own current broadcast behaviour).
	require.Len(t, beatsNamed(toldSince(t, mgr, beforeRound2, "barbarian"), encounter.BeatFightEnded), 1,
		"barbarian must also receive fight_ended")

	for _, who := range []string{"fighter", "barbarian"} {
		final, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: who})
		require.NoError(t, err)
		require.Equal(t, session.ClockWorld, final.Clock, "%s: the fight the killing blow ended left nobody still in it", who)
	}
}
