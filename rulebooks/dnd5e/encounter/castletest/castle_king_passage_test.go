// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_passage_test.go is THE KITCHEN PASSAGE (rpg-project#958,
// Passages S1): the castle's wiring of a line that tells a record and a line
// that takes you somewhere. The hero, beside the purse and in speaking reach
// of the scullion, picks the purse up and buys the way; the fighter and the
// rogue, by the grate across the kitchen, catch the answer too, come to hold
// the record `kitchen-way`, and so know the passage it reveals. The fighter
// goes down the grate to the chapel, and the rogue follows and comes up
// beside them. Each rule is proved on the small site
// (secretpassage_test.go); this is only the wiring, placed so nobody walks.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	kingPurse = encounter.PropID("purse")
	kingGrate = encounter.PropID("grate")
	kingRogue = encounter.MemberID("rogue")
)

// Where the passage route begins, as offset cells for
// [encounter.MemberInput.Position]: the hero beside the purse and two cells
// from the scullion; the fighter and the rogue on either side of the grate.
var (
	besideThePurse    = spatial.Position{X: -22, Y: -9}
	westOfTheGrate    = spatial.Position{X: -20, Y: -14}
	eastOfTheGrate    = spatial.Position{X: -18, Y: -14}
	theChapel         = encounter.RegionID("region-chapel")
	theGrateTravelers = []encounter.MemberID{kingFighter, kingRogue}
)

// sayToTheGrate has a member say one of the grate's lines.
func (s *kingScene) sayToTheGrate(member encounter.MemberID, option string) {
	s.t.Helper()
	_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: member, Prop: kingGrate, OptionID: option, Roller: rollsOne{},
	})
	require.NoError(s.t, err)
}

// passageScene is the kitchen once the scullion has told the way: built
// once, before any test, and loaded by each test that starts from it.
var passageScene struct {
	once     sync.Once
	err      error
	blob     []byte
	compiled dungeonspec.Compiled
	host     castleHost
}

// passageSceneTests are the tests that start from [passageScene].
var passageSceneTests = []string{"TestMadKingTheScullionTellsTheWayAndTheGrateTakesIt"}

// buildThePassageScene opens the castle with the hero beside the purse and
// the fighter and the rogue by the grate; nobody knows the grate is there,
// and the scullion offers nothing to an empty hand. The hero picks the purse
// up and buys the way.
func buildThePassageScene() {
	tb := &fixtureTB{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fixtureFailed); !ok {
				panic(r)
			}
			passageScene.err = fmt.Errorf("the passage scene: %s", tb.failed)
		}
	}()
	s := openTheCastleAs(tb, castleHost{at: besideThePurse, noReload: true, players: []encounter.MemberInput{
		{ID: kingFighter, Kind: encounter.KindPlayer, Position: westOfTheGrate},
		{ID: kingRogue, Kind: encounter.KindPlayer, Position: eastOfTheGrate},
	}})
	_, err := s.enc.PropOptionsFor(kingFighter, kingGrate)
	require.ErrorIs(tb, err, encounter.ErrNoProp, "nobody knows the grate is there")
	require.False(tb, s.offers(kingScullion, "favour"), "no coin, no way out")

	_, err = s.enc.Hold(&encounter.HoldInput{Member: hero, Target: kingPurse})
	require.NoError(tb, err, "the hero stands beside the purse")
	require.True(tb, s.offers(kingScullion, "favour"), "the scullion talks to a coin")
	_, err = s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: kingScullion, OptionID: "favour", Roller: rollsOne{},
	})
	require.NoError(tb, err)
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
	passageScene.blob, passageScene.compiled, passageScene.host = blob, s.compiled, s.host
}

func TestMadKingTheScullionTellsTheWayAndTheGrateTakesIt(t *testing.T) {
	t.Parallel()
	require.NoError(t, passageScene.err)
	s := loadTheSceneOn(t, passageScene.blob, passageScene.compiled, passageScene.host)
	for _, m := range theGrateTravelers {
		offered, err := s.enc.PropOptionsFor(m, kingGrate)
		require.NoError(t, err, "%s caught the scullion's answer across the kitchen", m)
		require.Len(t, offered, 2, "up to the chapel, down to the cells")
	}

	s.sayToTheGrate(kingFighter, "to-chapel")
	require.Equal(t, theChapel, s.regionOf(s.at(kingFighter)))
	require.NotEqual(t, theChapel, s.regionOf(s.at(kingRogue)), "the rogue stays until they say it")

	s.sayToTheGrate(kingRogue, "to-chapel")
	require.Equal(t, theChapel, s.regionOf(s.at(kingRogue)))
	require.InDelta(t, 1, s.enc.Distance(s.at(kingFighter), s.at(kingRogue)), 0, "the rogue comes up beside the fighter")
}
