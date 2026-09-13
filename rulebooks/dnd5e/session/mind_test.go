// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The mind is one word said in three places, and this package is the only
// one that can hear all three.
//
// A monster definition names it (monster.MindRetaliator), a driver looks it
// up (behavior.MindRetaliator), and nothing between them shares a type: the
// name crosses the encounter as an opaque string, exactly as Targeting does,
// so a typo in either constant is a monster that either fails loudly at the
// driver or silently gets the basic behaviour. The driver's module cannot
// import its parent, so this is the one place the two can be compared at
// all.
func TestMindWordsAgree(t *testing.T) {
	require.Equal(t, string(monster.MindRetaliator), behavior.MindRetaliator,
		"the sheet's word and the driver's registry key are the same word")
}

// mindScene is one fighter and one skeleton on a board, driven by whatever
// driver the test hands it.
func mindScene(t *testing.T, driver session.TurnDriver) (*session.Manager, *fakeEncounters) {
	t.Helper()

	sessions, encounters := newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: driver,
		Sessions: sessions, Encounters: encounters,
		Characters: newFakeCharacters(armedFighter("fighter")), Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(40, 6),
	})
	require.NoError(t, err)
	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	require.NoError(t, err)
	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 1, Y: 0},
	})
	require.NoError(t, err)

	return mgr, encounters
}

// The mind crosses at placement, and only for the member whose sheet names
// one.
//
// Both entry verbs share one placement path, so a mind defaulted anywhere
// inside it would give a disconnected player's driven turn a monster's
// grudge. The player is the control: the same call, the same path, and the
// empty word, which is how an absent mind says so.
func TestTheSheetsMindCrossesAtPlacement(t *testing.T) {
	_, encounters := mindScene(t, session.Pass{})

	data, err := encounters.GetEncounter(context.Background(), "world")
	require.NoError(t, err)

	minds := map[string]string{}
	for _, member := range data.Members {
		minds[string(member.ID)] = member.Mind
	}

	require.Equal(t, string(monster.MindRetaliator), minds["skel-1"],
		"the skeleton's own sheet names the retaliator, and placement carries the word")
	require.Equal(t, "", minds["fighter"],
		"a player names none, and nothing on the shared path invents one")
}

// A spawned skeleton's mind reaches the driver that has to act on it.
//
// The stored record above is half the journey. This is the other half: the
// turn projects the word back out into the view a driver reads, which is the
// only thing a mind ever looks at. A projection that dropped it would leave
// the record above perfectly correct and every monster basic.
func TestASpawnedSkeletonsMindReachesItsDriver(t *testing.T) {
	recorder := &recordingBehavior{next: session.Pass{}}
	mgr, _ := mindScene(t, recorder)

	declarationID := currentEndTurnID(t, mgr, "sess", "fighter")
	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "fighter", DeclarationID: declarationID,
	})
	require.NoError(t, err)

	view := requireRecordedView(t, recorder.views, func(view session.MonsterView) bool {
		return view.Self == "skel-1"
	})
	require.Equal(t, string(monster.MindRetaliator), view.Mind,
		"the driver is told which mind to think with")
	require.NotEmpty(t, view.Holdings,
		"and handed the testimony it is expected to read (rule A1: as values)")
}

// The bow skeleton's scene at the session seam: the same arrow the driver's
// own module proves, re-proved on this package's own types.
//
// It is not a duplicate of that test. What is being asserted here is that
// the whole view a mind reads — the sheet's word, holdings on two channels,
// the clock's high-water, reach, and the step away — survives this
// package's boundary and comes back as an intent a host can act on. The
// driver module's test would still pass with every one of those fields
// dropped between here and there.
var (
	mindedBow   = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "shortbow"}
	mindedBlade = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "shortsword"}

	mindedSkeletonAt = spatial.Position{X: 6, Y: 4}
	mindedAliceAt    = spatial.Position{X: 6, Y: 0} // far, and only the bow reaches her
	mindedBobAt      = spatial.Position{X: 5, Y: 4} // next to the skeleton
)

// mindedView is the skeleton's turn as this package hands it over: alice far
// and bob adjacent, both standing, both currently seen.
func mindedView(t *testing.T, at uint64, extra ...session.Holding) session.MonsterView {
	t.Helper()

	bow, blade := mindedBow.String(), mindedBlade.String()
	holdings := []session.Holding{
		mindedSighting(t, "alice", mindedAliceAt, at),
		mindedSighting(t, "bob", mindedBobAt, at),
	}
	holdings = append(holdings, extra...)

	return session.MonsterView{
		Self:     "skeleton",
		Position: mindedSkeletonAt,
		Mind:     behavior.MindRetaliator,
		Actions: []session.ActionView{
			{Ref: blade, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
			{Ref: bow, Name: "Shortbow", RangeFeet: 80, Kind: "ranged"},
		},
		Holdings: holdings,
		At:       at,
		Seen: []session.SeenMember{
			{
				ID: "alice", Kind: session.KindPlayer, Standing: true,
				Position: mindedAliceAt, DistanceCells: 4,
				InReach:  map[string]bool{bow: true, blade: false},
				Path:     []spatial.Position{{X: 6, Y: 3}},
				AwayPath: []spatial.Position{{X: 6, Y: 5}},
			},
			{
				ID: "bob", Kind: session.KindPlayer, Standing: true,
				Position: mindedBobAt, DistanceCells: 1,
				InReach:  map[string]bool{bow: true, blade: true},
				AwayPath: []spatial.Position{{X: 7, Y: 4}},
			},
		},
		Budget: session.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// mindedSighting is sustained sight testimony, the shape a pass leaves behind.
func mindedSighting(t *testing.T, who string, at spatial.Position, when uint64) session.Holding {
	t.Helper()

	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationKnown, Position: at,
	})
	require.NoError(t, err)

	return session.Holding{
		Subject: who, Payload: payload, Channel: string(perception.Sight),
		Observed: 0, Confirmed: when, CurrentVia: []string{string(perception.Sight)},
	}
}

// mindedShotBy is the deed the encounter lands when somebody shoots the
// skeleton where it can see them: its own channel, current on nothing.
func mindedShotBy(who string, when uint64) session.Holding {
	return session.Holding{
		Subject: string(deed.Subject(core.EntityID(who))),
		Channel: string(deed.Channel),
		Payload: deed.Encode(deed.Deed{
			Verb: encounter.DeedAttack, Actor: core.EntityID(who),
			Target: "skeleton", Where: mindedAliceAt.String(),
		}),
		Observed: when, Confirmed: when,
	}
}

func TestMindedTurnsOnWhoeverShotIt(t *testing.T) {
	driver, err := session.Minded(nil)
	require.NoError(t, err)

	intent, err := driver.Act(mindedView(t, 3, mindedShotBy("alice", 3)))
	require.NoError(t, err)
	require.Equal(t, session.Attack{Target: "alice", Action: mindedBow.String()}, intent,
		"at alice, with the action that reaches her — bob is closer and did not shoot")
}

func TestMindedGoesBackToClosestWithNoDeed(t *testing.T) {
	driver, err := session.Minded(nil)
	require.NoError(t, err)

	intent, err := driver.Act(mindedView(t, 3))
	require.NoError(t, err)
	require.Equal(t, session.Attack{Target: "bob", Action: mindedBlade.String()}, intent,
		"the same view without the deed: the closest standing player, in melee")
}

// A member whose sheet names no mind is driven exactly as Behavior() drives
// it — rule A5, asserted against the reference driver's own answer rather
// than a copy of it.
func TestMindedFallsBackToTheBasicDriver(t *testing.T) {
	driver, err := session.Minded(nil)
	require.NoError(t, err)

	view := mindedView(t, 3, mindedShotBy("alice", 3))
	view.Mind = ""

	minded, err := driver.Act(view)
	require.NoError(t, err)
	basic, err := session.Behavior().Act(view)
	require.NoError(t, err)
	require.Equal(t, basic, minded, "no mind named, no grudge held")
}

// A mind nobody implements is a wiring fault, and it crosses the boundary as
// one rather than turning into a quiet pass.
func TestMindedRefusesAnUnknownMind(t *testing.T) {
	driver, err := session.Minded(nil)
	require.NoError(t, err)

	view := mindedView(t, 3)
	view.Mind = "genius"

	_, err = driver.Act(view)
	require.ErrorIs(t, err, behavior.ErrUnknownMind)
}

// Patience is the host's to set, and setting it shorter forgets a shot that
// the default would still hold a grudge over.
//
// The control matters more than the assertion: the same deed, the same
// clock, two drivers, two different targets. A Patience the constructor
// ignored would give the same answer twice.
func TestMindedPatienceIsTheHostsToSet(t *testing.T) {
	patient, err := session.Minded(nil)
	require.NoError(t, err)
	forgetful, err := session.Minded(&session.MindedInput{Patience: 1})
	require.NoError(t, err)

	// Two ticks after the shot: exactly the default's reach, and one past a
	// patience of one.
	view := mindedView(t, 5, mindedShotBy("alice", 3))

	held, err := patient.Act(view)
	require.NoError(t, err)
	require.Equal(t, session.Attack{Target: "alice", Action: mindedBow.String()}, held,
		"the default still remembers a shot two ticks old")

	forgotten, err := forgetful.Act(view)
	require.NoError(t, err)
	require.Equal(t, session.Attack{Target: "bob", Action: mindedBlade.String()}, forgotten,
		"a one-tick patience does not, and the host is what chose that")
}

// The driver a host wires is ONE driver for the whole session, and it
// remembers across turns — which is the reason Minded returns a value rather
// than a function.
func TestMindedRemembersAcrossTurns(t *testing.T) {
	driver, err := session.Minded(nil)
	require.NoError(t, err)

	first, err := driver.Act(mindedView(t, 3, mindedShotBy("alice", 3)))
	require.NoError(t, err)
	second, err := driver.Act(mindedView(t, 4, mindedShotBy("alice", 3)))
	require.NoError(t, err)
	require.Equal(t, first, second, "still alice, one tick later")
}
