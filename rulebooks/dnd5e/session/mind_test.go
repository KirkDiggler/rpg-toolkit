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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The mind is one word said in three places, and this package is the only
// one that can hear all three.
//
// A monster definition names it (monster.MindRetaliator), a driver looks it
// up (behavior.MindRetaliator), and nothing between them shares a type: the
// name crosses the encounter as an opaque string, exactly as Targeting does,
// so a typo in either constant is a monster that either fails loudly at the
// driver or silently gets the basic behaviour.
//
// The driver's module DOES NOT import its parent's definition vocabulary,
// and that is a law rather than an impossibility: nothing in the module
// graph forbids it — the parent has never required the driver, so there is
// no cycle to hit — and the behavior module in fact imports `encounter` and
// `weapons` from the same parent today. What it keeps out is `monster`.
// A mind is named on the sheet and looked up by the driver (adoption rule
// A5), so the driver reads the word off the view it was handed and stays
// definition-ignorant; a registry keyed on `monster.Mind` would type-check
// perfectly and quietly end that. Which is why this is the one place the
// two can be compared at all, and why the sentence is worth stating: a
// choice can be reversed by accident, and an impossibility cannot.
func TestMindWordsAgree(t *testing.T) {
	words := []struct {
		sheet  monster.Mind
		driver string
	}{
		{sheet: monster.MindRetaliator, driver: behavior.MindRetaliator},
		{sheet: monster.MindBerserker, driver: behavior.MindBerserker},
		{sheet: monster.MindCoward, driver: behavior.MindCoward},
	}

	for _, word := range words {
		t.Run(word.sheet.String(), func(t *testing.T) {
			require.Equal(t, word.sheet.String(), word.driver,
				"the sheet's word and the driver's registry key are the same word")

			parsed, err := monster.ParseMind(word.driver)
			require.NoError(t, err, "and an author may write the driver's key on a sheet")
			require.Equal(t, word.sheet, parsed)
		})
	}
}

// spawnAs is another monster to put on the board beside the skeleton. Its
// word is its own definition's and never named here — which is the whole
// point of the placement proof below.
type spawnAs struct {
	id  string
	ref string
}

// mindScene is one fighter and one skeleton on a board, plus whatever else
// the test asks for, driven by whatever driver it hands over.
func mindScene(t *testing.T, driver session.TurnDriver, extra ...spawnAs) (*session.Manager, *fakeEncounters) {
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

	for i, spawn := range extra {
		_, err = mgr.Spawn(ctx, &session.SpawnInput{
			Session: "sess", ID: spawn.id, Ref: spawn.ref,
			Position: spatial.Position{X: float64(i + 2), Y: 0},
		})
		require.NoError(t, err)
	}

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
	_, encounters := mindScene(t, session.Pass{},
		spawnAs{id: "thug-1", ref: refs.Monsters.Thug().String()},
		spawnAs{id: "goblin-1", ref: refs.Monsters.Goblin().String()},
	)

	data, err := encounters.GetEncounter(context.Background(), "world")
	require.NoError(t, err)

	minds := map[string]string{}
	for _, member := range data.Members {
		minds[string(member.ID)] = member.Mind
	}

	// EACH MEMBER'S OWN WORD, not one word for the encounter. Three monsters
	// placed by one call to one path, and the path reads each definition
	// rather than deciding anything — which is what a second word proves and
	// a single skeleton never could.
	require.Equal(t, string(monster.MindRetaliator), minds["skel-1"],
		"the skeleton's own sheet names the retaliator, and placement carries the word")
	require.Equal(t, string(monster.MindBerserker), minds["thug-1"],
		"and the thug's names the berserker, on the same path in the same call")
	require.Equal(t, string(monster.MindCoward), minds["goblin-1"],
		"and the goblin's names the coward")
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
		mindedSighting(t, "alice", mindedAliceAt, at, weapons.LightCrossbow),
		mindedSighting(t, "bob", mindedBobAt, at, weapons.Longsword),
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

// mindedSighting is sustained sight testimony, the shape a pass leaves behind:
// where the figure was, and what was seen in their hands, which is what a
// grudge turns on now that it is a weapon rule first.
func mindedSighting(t *testing.T, who string, at spatial.Position, when uint64, mainHand string) session.Holding {
	t.Helper()

	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationKnown, Position: at,
		Equipment: &encounter.HeldEquipment{MainHand: mainHand},
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

// Three members of ONE encounter, shot the same way on the same turn, decide
// three different things — because each thinks with the word its own sheet
// names.
//
// This is where the patience knob went (rpg-toolkit#1745). It used to be
// the host's: one number, set at construction, for every mind in the
// process. The test that proved a host could set it is gone, and this is
// what replaced it, because the knob it proved could not have produced this
// scene at all — one driver, one turn, three profiles.
//
// ONE driver answers all three, which is the other half. The driver files a
// mind per MEMBER id, so the skeleton's retaliator, the thug's berserker and
// the goblin's coward live side by side in the value a host hands to one
// session. Every word the rulebook ships is routed through this seam here;
// the vocabulary test above pins the words themselves, and this is the only
// place all three are spent.
//
// The scene is the same one the driver's own module proves each word
// against, rebuilt on this package's types: the shooter put the crossbow
// away and closed, and a bystander stands in reach. The skeleton lets her
// go and swings at the bystander. The thug does not care what she is
// holding. The coward does not answer shots at all and will not let the
// bystander stand that close.
//
// THE THREE ANSWERS ARE PAIRWISE DIFFERENT, and that is the assertion that
// bites: the words collapsing to one — the same registry key answering for
// every member — leaves three identical intents, whichever key it is.
func TestEveryMemberOfOneEncounterThinksWithItsOwnWord(t *testing.T) {
	driver, err := session.Minded(nil)
	require.NoError(t, err)

	act := func(member, word string) session.TurnIntent {
		t.Helper()
		intent, actErr := driver.Act(mindedSwap(t, member, word))
		require.NoError(t, actErr)
		return intent
	}

	skeleton := act("skeleton", behavior.MindRetaliator)
	thug := act("thug", behavior.MindBerserker)
	goblin := act("goblin", behavior.MindCoward)

	require.Equal(t, session.Attack{Target: "bob", Action: mindedBlade.String()}, skeleton,
		"the skeleton swings at the man beside it: alice cannot shoot back any more")
	require.Equal(t, session.Attack{Target: "alice", Action: mindedBow.String()}, thug,
		"the thug shoots past him at alice, who shot it, sword or no sword")
	require.Equal(t, session.Move{Path: []spatial.Position{{X: 7, Y: 4}}}, goblin,
		"the goblin holds no grudge and wants bob further off: one step back, the way the board laid it out")

	require.NotEqual(t, skeleton, thug,
		"same board, same shot, same turn, same driver: only the word differs")
	require.NotEqual(t, thug, goblin)
	require.NotEqual(t, skeleton, goblin)
}

// mindedSwap is #1745's scene at this package's boundary: member was shot
// by a crossbow from across the room two ticks ago; alice has since drawn a
// longsword and closed to three cells; bob has been standing next to it the
// whole time and has attacked nobody. The bow reaches both of them and the
// blade reaches only bob.
func mindedSwap(t *testing.T, member, word string) session.MonsterView {
	t.Helper()

	bow, blade := mindedBow.String(), mindedBlade.String()
	closed := spatial.Position{X: 6, Y: 1}

	return session.MonsterView{
		Self:     member,
		Position: mindedSkeletonAt,
		Mind:     word,
		Actions: []session.ActionView{
			{Ref: blade, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
			{Ref: bow, Name: "Shortbow", RangeFeet: 80, Kind: "ranged"},
		},
		Holdings: []session.Holding{
			mindedSighting(t, "alice", closed, 5, weapons.Longsword),
			mindedSighting(t, "bob", mindedBobAt, 5, weapons.Longsword),
			mindedShotAt(member, "alice", 3),
		},
		At: 5,
		Seen: []session.SeenMember{
			{
				ID: "alice", Kind: session.KindPlayer, Standing: true,
				Position: closed, DistanceCells: 3,
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

// mindedShotAt is mindedShotBy for a scene whose victim is not called
// skeleton — a deed names who it was done to, and this file now has two
// members being shot the same way.
func mindedShotAt(target, who string, when uint64) session.Holding {
	shot := mindedShotBy(who, when)
	shot.Payload = deed.Encode(deed.Deed{
		Verb: encounter.DeedAttack, Actor: core.EntityID(who),
		Target: core.EntityID(target), Where: mindedAliceAt.String(),
	})

	return shot
}

// One driver, the same view twice, the same answer — the driver's answer is
// stable across calls.
//
// What this does NOT prove is that anything was REMEMBERED. No mind shipped
// here has memory that shows up in an answer: the retaliator names its
// contact off the view it was handed, so a driver that registered a fresh
// mind on every call would pass this test identically. Minded returns a
// value rather than a function because the driver holds state — a game with
// per-member sheets, places and names — and is not safe for concurrent use,
// which is true whether or not that state is observable from out here.
func TestMindedAnswersAreStableAcrossCalls(t *testing.T) {
	driver, err := session.Minded(nil)
	require.NoError(t, err)

	first, err := driver.Act(mindedView(t, 3, mindedShotBy("alice", 3)))
	require.NoError(t, err)
	second, err := driver.Act(mindedView(t, 4, mindedShotBy("alice", 3)))
	require.NoError(t, err)
	require.Equal(t, first, second, "the same driver, asked again one tick later, answers the same")
}
