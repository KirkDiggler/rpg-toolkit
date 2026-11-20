// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_prince_test.go is THE RIGHTFUL KING IN THE CELLS
// (rpg-project#1023, the Keep's shared cell): the castle's wiring of a plain
// line that recruits and a plain line that tells a way out. The party, failed
// at the gate twice and marched to the cells, finds the old king's son among
// the prisoners. Taken on, his side turns the party's, he follows the party
// and tells them the weak place in the cell, a drain at the back that comes
// up in the armoury. Left behind, he tells it all the same. Each rule is
// proved on its own site (the ally walk, the kitchen passage); this is only
// the castle's wiring, played from the jail.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The rightful king's ids, as the file gives them.
const (
	kingPrince    = encounter.MemberID("prince")
	trueCrown     = encounter.FactionID("true-crown")
	kingCellDrain = encounter.PropID("cell-drain")
)

// Where the cells scene goes, by the file's regions and floor.
var (
	theCells       = encounter.RegionID("region-cells")
	theArmoury     = encounter.RegionID("region-armoury")
	besideTheDrain = castleCell(-20, 13)
	eastInTheCell  = castleCell(-18, 13)
)

// princeScene is the cells scene played twice from the jail, once before
// any test, and loaded by each test that reads it. The hero walks to the
// rightful king (nobody knows the drain yet, and he is civil) and says the
// king's purpose; then, from there:
//
//   - recruited: the hero takes him on, walks east across the cell and
//     steps on twice, which gives him rounds to follow;
//   - left: the hero leaves him and walks to the drain.
var princeScene struct {
	once      sync.Once
	err       error
	post      spatial.Position
	recruited []byte
	left      []byte
}

// princeSceneTests are the tests that start from [princeScene].
var princeSceneTests = []string{
	"TestMadKingTheRightfulKingJoinsAndShowsTheWayOut", "TestMadKingTheRightfulKingLeftBehindStillTellsTheWay",
}

// princeHost is the jail's host, not saved and loaded back between rounds:
// the cells' walks are short, and the reload is proved on its own route.
func princeHost() castleHost {
	host := arrestScene.host
	host.noReload = true

	return host
}

// buildThePrinceScene plays [princeScene] from the jailed scene.
func buildThePrinceScene() {
	tb := &fixtureTB{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fixtureFailed); !ok {
				panic(r)
			}
			princeScene.err = fmt.Errorf("the prince scene: %s", tb.failed)
		}
	}()
	jailedScene.once.Do(buildTheJailedScene)
	require.NoError(tb, jailedScene.err)
	s := loadTheSceneOn(tb, jailedScene.blob, arrestScene.compiled, princeHost())
	require.Equal(tb, theCells, s.regionOf(s.at(hero)), "precondition: the hero was marched to the cells")
	require.Equal(tb, theCells, s.regionOf(s.at(kingPrince)), "the rightful king is in the cells")
	princeScene.post = s.at(kingPrince)
	s.walkTo(s.at(kingPrince))
	require.Equal(tb, theCells, s.regionOf(s.at(hero)))
	_, err := s.enc.PropOptionsFor(hero, kingCellDrain)
	require.ErrorIs(tb, err, encounter.ErrNoProp, "precondition: nobody knows the drain is there")
	require.Equal(tb, encounter.StanceNeutral, s.stance(trueCrown, encounter.FactionParty), "he is civil to the party")
	require.True(tb, s.offersTo(hero, kingPrince, "join-us"))
	s.sayPlain(kingPrince, "kill-the-king")
	talked, err := json.Marshal(s.enc.ToData())
	require.NoError(tb, err)

	s.sayPlain(kingPrince, "join-us")
	s.walkTo(eastInTheCell)
	s.stepAway(kingPrince)
	s.stepAway(kingPrince)
	princeScene.recruited, err = json.Marshal(s.enc.ToData())
	require.NoError(tb, err)

	s = loadTheSceneOn(tb, talked, arrestScene.compiled, princeHost())
	s.sayPlain(kingPrince, "leave-him")
	s.walkTo(besideTheDrain)
	princeScene.left, err = json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
}

// princeSceneFrom loads one of [princeScene]'s saved runs.
func princeSceneFrom(t *testing.T, which func() []byte) *kingScene {
	t.Helper()
	princeScene.once.Do(buildThePrinceScene)
	require.NoError(t, princeScene.err)

	return loadTheSceneOn(t, which(), arrestScene.compiled, princeHost())
}

// sayPlain has the hero say one of a creature's plain lines: no check.
func (s *kingScene) sayPlain(target encounter.MemberID, option string) {
	s.t.Helper()
	_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: target, OptionID: option, Roller: rollsOne{},
		Revision: s.revisionOf(hero, target, option),
	})
	require.NoError(s.t, err, "%s offers %q", target, option)
}

// stepAway has the hero take one step away from a member and end the walk:
// one more round of the world for the creatures to act in.
func (s *kingScene) stepAway(from encounter.MemberID) {
	s.t.Helper()
	out, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveAway, Anchor: s.at(from), Budget: 1})
	require.NoError(s.t, err)
	require.NotEmpty(s.t, out.Path, "the hero has room to step away (%s)", out.StoppedBy)
	_, err = s.enc.Step(&encounter.StepInput{Member: hero, To: out.Path[0], EndWalk: true})
	require.NoError(s.t, err)
}

// throughTheDrain has the hero, beside the drain, take it.
func (s *kingScene) throughTheDrain() {
	s.t.Helper()
	offered, err := s.enc.PropOptionsFor(hero, kingCellDrain)
	require.NoError(s.t, err, "the hero knows the drain")
	require.NotEmpty(s.t, offered)
	_, err = s.enc.Interact(&encounter.InteractInput{
		Range: speakReach, Actor: hero, Prop: kingCellDrain, OptionID: "to-armoury", Roller: rollsOne{},
	})
	require.NoError(s.t, err)
	require.Equal(s.t, theArmoury, s.regionOf(s.at(hero)), "the passage comes up in the armoury")
}

// TestMadKingTheRightfulKingJoinsAndShowsTheWayOut: taken on with no check,
// the rightful king's side is the party's and the king's court his enemy;
// he followed the hero across the cell, and the hero knows the drain and
// takes it out to the armoury, out of the gate's sight.
func TestMadKingTheRightfulKingJoinsAndShowsTheWayOut(t *testing.T) {
	t.Parallel()
	s := princeSceneFrom(t, func() []byte { return princeScene.recruited })

	require.True(t, s.learners("prince-recruited")[kingPrince], "he took the party's offer")
	require.Equal(t, encounter.StanceAllied, s.stance(trueCrown, encounter.FactionParty), "his side is the party's")
	require.False(t, s.hostile(kingPrince, hero))
	require.Equal(t, encounter.StanceHostile, s.stance(trueCrown, loyalists), "the king's court is his enemy")
	require.False(t, s.offersTo(hero, kingPrince, "join-us"), "taken on once")

	post := princeScene.post
	require.Positive(t, s.pickedTowardAlly(kingPrince), "he follows the party")
	require.NotEqual(t, post, s.at(kingPrince), "he left where he stood")
	require.Less(t, s.enc.Distance(s.at(kingPrince), s.at(hero)), s.enc.Distance(post, s.at(hero)),
		"and came toward the hero")

	s.throughTheDrain()
	require.False(t, s.learners("prisoners-escaped")[kingCaptain], "nobody at the gate saw the hero come out")
}

// TestMadKingTheRightfulKingLeftBehindStillTellsTheWay: left behind, he is
// not the party's side and holds where he stands, and still tells the drain.
func TestMadKingTheRightfulKingLeftBehindStillTellsTheWay(t *testing.T) {
	t.Parallel()
	s := princeSceneFrom(t, func() []byte { return princeScene.left })

	require.False(t, s.learners("prince-recruited")[kingPrince])
	require.Equal(t, encounter.StanceNeutral, s.stance(trueCrown, encounter.FactionParty), "left, he is not the party's side")
	s.throughTheDrain()
	require.Equal(t, princeScene.post, s.at(kingPrince), "he stays in the cell")
}
