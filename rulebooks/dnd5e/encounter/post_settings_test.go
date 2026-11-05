// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// post_settings_test.go pins A POST'S SETTINGS (rpg-project#857, S1c of the
// guard holds the gate; design R6): a post holds both ways unless its
// setting names a direction, and a mover who knows its pass fact walks
// through. The direction is the side the crossing starts on, the post's side
// being the region its cells stand in; it is read by every hold reader — the
// step, the route flood, the straight walk a push takes, the walk's door
// pass, the door verbs and what an observer is told.

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// holdPass is the hold hall's pass fact.
const holdPass = FactID("admitted")

// sidedSetup is [holdSetup] with the hall cut at the leaf into two regions:
// the gatehouse along row 0, the post's side, where the guard stands, and
// the yard below it. Each of the leaf's crossings has one cell in each.
func sidedSetup(door DoorInput, alice spatial.Position, members ...MemberInput) *SetupInput {
	in := holdSetup(everyoneSeesTheWholeMap{}, door, alice, members...)
	in.Field.Regions = []RegionInput{rectRegion("gatehouse", 0, 0, 10, 1), rectRegion("yard", 0, 1, 10, 5)}

	return in
}

// sidedHall opens [sidedSetup].
func sidedHall(t *testing.T, door DoorInput, alice spatial.Position, members ...MemberInput) *Encounter {
	t.Helper()
	enc, err := NewEncounter(sidedSetup(door, alice, members...))
	require.NoError(t, err)

	return enc
}

// setHolder is [holder] with its posts set to a direction and a pass.
func setHolder(
	t *testing.T, id MemberID, at spatial.Position, door DoorID, against PostDirection, pass FactID, cells ...spatial.Position,
) MemberInput {
	t.Helper()
	m := holder(t, id, at, door, cells...)
	for state, p := range m.Posts {
		p.Against, p.Pass = against, pass
		m.Posts[state] = p
	}

	return m
}

// gatehouseGuard is the hold hall's guard on the gatehouse cell of the
// leaf, his post set as given.
func gatehouseGuard(t *testing.T, against PostDirection, pass FactID) MemberInput {
	t.Helper()

	return setHolder(t, postedGuard, holdNorthWest, holdLeaf, against, pass, holdNorthWest)
}

// teach writes a fact into a member's own knowledge, as a won line does.
func teach(t *testing.T, enc *Encounter, member MemberID, fact FactID) {
	t.Helper()
	require.NoError(t, enc.learnFact(member, fact, BeatAnswered, uint64(enc.clock.ToData().HighWater)))
}

// requireBarredDoor requires a door verb refused by the hold. The name in
// its sentence is the sight rule's ([Encounter.seenHolder]): from the far
// side of a shut door the guard is unseen, "someone".
func requireBarredDoor(t *testing.T, refused *DoorRefusal, err error) {
	t.Helper()
	require.NoError(t, err, "a held door's verb is answered, not refused")
	require.NotNil(t, refused, "the output says the door is held")
	require.Contains(t, []string{"Gate guard bars the door", "someone bars the door"}, refused.Reason)
}

// requireDoorWorked requires a door verb that was not held.
func requireDoorWorked(t *testing.T, refused *DoorRefusal, err error, msg string) {
	t.Helper()
	require.NoError(t, err, msg)
	require.Nil(t, refused, msg)
}

// crossing is one way across the leaf: out of the gatehouse into the yard,
// or the reverse.
type crossing struct {
	name     string
	from, to spatial.Position
}

var (
	outOfTheGatehouse = crossing{"out of the gatehouse", holdNorthEast, holdSouthEast}
	intoTheGatehouse  = crossing{"into the gatehouse", holdSouthEast, holdNorthEast}
)

// requireEveryReaderAgrees drives every reader of a hold across one way of
// the leaf, with the guard's post set as given, and requires each to bar or
// pass it as `barred` says: the step, the route flood (clocks.go's
// walkerEdge), the straight walk a push takes (directive.go's stepInto),
// the walk's door pass on a shut door (walkdoor.go's doorToOpen), and the
// door verbs from the cell the crossing starts on.
func requireEveryReaderAgrees(t *testing.T, against PostDirection, way crossing, barred bool) {
	t.Helper()
	guard := func() MemberInput { return gatehouseGuard(t, against, "") }

	t.Run(way.name+": the step", func(t *testing.T) {
		enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), way.from, guard())
		if barred {
			require.Equal(t, "Gate guard bars the way", heldStop(t, enc, way.to))
			return
		}
		steps(t, enc, way.to, "the post does not bar this way")
	})

	t.Run(way.name+": the route", func(t *testing.T) {
		enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), way.from, guard())
		q, err := enc.cellQuery()
		require.NoError(t, err)
		path, ok, err := q.routeWithin("alice", way.from, way.to, 0)
		require.NoError(t, err)
		require.True(t, ok, "the open hall has a way")
		if barred {
			require.NotEqual(t, []spatial.Position{way.to}, path, "barred, the route goes round the opening")
			return
		}
		require.Equal(t, []spatial.Position{way.to}, path, "not barred, the route goes straight through")
	})

	t.Run(way.name+": a push", func(t *testing.T) {
		enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), way.from, guard())
		out, err := enc.Direct(context.Background(), DirectInput{
			Mover: "alice", Cause: core.Ref{Module: "test", Type: "effects", ID: "shove"},
			Route: []spatial.Position{way.to}, Kind: MoveDisplaced,
		})
		require.NoError(t, err)
		at, _ := enc.canvas.GetEntityPosition("alice")
		if barred {
			require.Zero(t, out.Moved, "the push is cut before the held crossing")
			require.Equal(t, way.from, at)
			return
		}
		require.Equal(t, way.to, at, "the push crosses")
	})

	t.Run(way.name+": the walk's door pass", func(t *testing.T) {
		enc := sidedHall(t, holdLeafDoor(DoorIsClosed()), way.from, guard())
		_, err := enc.walkPath(context.Background(), "alice", enc.members["alice"], []spatial.Position{way.to}, nil, 0, core.Ref{}, "", false)
		require.NoError(t, err)
		at, _ := enc.canvas.GetEntityPosition("alice")
		if barred {
			require.Equal(t, DoorClosed, enc.doorsByID[holdLeaf].state.Kind(), "a walk never opens a door held against it")
			require.Equal(t, way.from, at)
			return
		}
		require.Equal(t, DoorOpen, enc.doorsByID[holdLeaf].state.Kind(), "the walk opens a door not held this way")
		require.Equal(t, way.to, at)
	})

	t.Run(way.name+": the door verbs", func(t *testing.T) {
		enc := sidedHall(t, holdLeafDoor(DoorIsClosed()), way.from, guard())
		held, reason, err := enc.DoorHeldFor("alice", holdLeaf)
		require.NoError(t, err)
		refused, openErr := openRefusal(enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"}))
		if barred {
			require.True(t, held)
			require.Contains(t, reason, "bars the door")
			requireBarredDoor(t, refused, openErr)
			return
		}
		require.False(t, held)
		requireDoorWorked(t, refused, openErr, "the door opens from the side the post does not bar")
	})
}

func TestAPostHoldsBothWaysByDefault(t *testing.T) {
	for _, against := range []PostDirection{"", PostBothWays} {
		requireEveryReaderAgrees(t, against, outOfTheGatehouse, true)
		requireEveryReaderAgrees(t, against, intoTheGatehouse, true)
	}
}

// TestAnEnteringPostLetsTheLeaversOut: the post's side is the gatehouse, so
// an entering post bars the crossing out of it, and a mover coming back the
// other way, leaving the yard, crosses.
func TestAnEnteringPostLetsTheLeaversOut(t *testing.T) {
	requireEveryReaderAgrees(t, PostEntering, outOfTheGatehouse, true)
	requireEveryReaderAgrees(t, PostEntering, intoTheGatehouse, false)
}

func TestALeavingPostLetsTheArrivalsIn(t *testing.T) {
	requireEveryReaderAgrees(t, PostLeaving, outOfTheGatehouse, false)
	requireEveryReaderAgrees(t, PostLeaving, intoTheGatehouse, true)
}

func TestAPassFactWalksThroughAHold(t *testing.T) {
	porter := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: holdBeside, Table: holds()}
	enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast, gatehouseGuard(t, PostBothWays, holdPass), porter)

	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "without the pass, barred")

	teach(t, enc, "alice", holdPass)
	steps(t, enc, holdNorthEast, "knowing the pass, she crosses")
	require.Equal(t, "posted", stateOf(enc, postedGuard), "while he stays posted")
	at, _ := enc.canvas.GetEntityPosition(string(postedGuard))
	require.Equal(t, holdNorthWest, at, "and on his cell")
	door, holds := enc.memberHolds(postedGuard, nil)
	require.True(t, holds, "and holding the door")
	require.Equal(t, holdLeaf, door)

	placeAt(t, enc, "alice", holdOffPost)
	placeAt(t, enc, "porter", holdSouthEast)
	out, err := enc.Step(&StepInput{Member: "porter", To: holdNorthEast})
	requireMoved(t, out, err, "an ally needs no pass")

	t.Run("and the door verbs", func(t *testing.T) {
		shut := sidedHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast, gatehouseGuard(t, PostBothWays, holdPass))
		refused, err := openRefusal(shut.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"}))
		requireBarredDoor(t, refused, err)
		teach(t, shut, "alice", holdPass)
		refused, err = openRefusal(shut.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"}))
		requireDoorWorked(t, refused, err, "knowing the pass, she works the door as an ally does")
	})
}

func TestThePassIsTheMoversOwn(t *testing.T) {
	bob := MemberInput{ID: "bob", Kind: KindPlayer, Position: holdSouthEast}
	enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), holdOffPost, gatehouseGuard(t, PostBothWays, holdPass), bob)
	teach(t, enc, "alice", holdPass)

	out, err := enc.Step(&StepInput{Member: "bob", To: holdNorthEast})
	requireStopped(t, out, err, "a companion who never learned it is barred")
	at, _ := enc.canvas.GetEntityPosition("bob")
	require.Equal(t, holdSouthEast, at)

	placeAt(t, enc, "bob", spatial.Position{X: 7, Y: 3})
	placeAt(t, enc, "alice", holdSouthEast)
	steps(t, enc, holdNorthEast, "and the one who learned it crosses")
}

func TestADirectedPostNeedsOneSidePerCrossing(t *testing.T) {
	lodged := func(in *SetupInput) {
		in.Field.Regions = []RegionInput{
			rectRegion("gatehouse", 0, 0, 8, 1), rectRegion("lodge", 8, 0, 2, 1), rectRegion("yard", 0, 1, 10, 5),
		}
	}
	oneRegion := func(in *SetupInput) { in.Field.Regions = []RegionInput{rectRegion("hall", 0, 0, 10, 6)} }
	keep := func(*SetupInput) {}

	for _, tc := range []struct {
		name   string
		guard  MemberInput
		field  func(*SetupInput)
		refuse bool
	}{
		{"its cells straddle two regions", setHolder(t, postedGuard, holdNorthWest, holdLeaf, PostEntering, "", holdNorthWest, holdSouthWest), keep, true},
		{"every crossing wholly on its side", setHolder(t, postedGuard, holdNorthWest, holdLeaf, PostEntering, "", holdNorthWest), oneRegion, true},
		{"no crossing touches its side", setHolder(t, postedGuard, holdOffPost, holdLeaf, PostLeaving, "", holdOffPost), lodged, true},
		{"a word that is no direction", setHolder(t, postedGuard, holdNorthWest, holdLeaf, "inward", "", holdNorthWest), keep, true},
		{"one side per crossing", setHolder(t, postedGuard, holdNorthWest, holdLeaf, PostEntering, "", holdNorthWest), keep, false},
		{"both ways needs no side", setHolder(t, postedGuard, holdNorthWest, holdLeaf, PostBothWays, "", holdNorthWest), oneRegion, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := sidedSetup(holdLeafDoor(DoorIsOpen()), holdSouthEast, tc.guard)
			tc.field(in)
			_, err := NewEncounter(in)
			if tc.refuse {
				require.ErrorIs(t, err, ErrBadMember)
				return
			}
			require.NoError(t, err)
		})
	}

	t.Run("and at the join door", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast)
		_, err := enc.Join(&JoinInput{
			Member: postedGuard, Kind: KindMonster, Faction: postedGarrison, Cell: holdNorthWest,
			Table: holds(), Machine: holdRecipe(t), Posts: map[string]Post{"posted": {Door: holdLeaf, Against: PostEntering}},
		})
		require.ErrorIs(t, err, ErrBadMember, "the one-region hall gives the post no side")
	})
}

func TestTheDoorVerbsReadTheSettings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		against PostDirection
		from    spatial.Position
		barred  bool
	}{
		{"entering, from his side", PostEntering, holdNorthEast, true},
		{"entering, from the far side", PostEntering, holdSouthEast, false},
		{"leaving, from his side", PostLeaving, holdNorthEast, false},
		{"leaving, from the far side", PostLeaving, holdSouthEast, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, verb := range []struct {
				name  string
				state DoorState
				do    func(enc *Encounter) (*DoorRefusal, error)
			}{
				{"open", DoorIsClosed(), func(enc *Encounter) (*DoorRefusal, error) {
					return openRefusal(enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"}))
				}},
				{"close", DoorIsOpen(), func(enc *Encounter) (*DoorRefusal, error) {
					return closeRefusal(enc.CloseDoor(&CloseDoorInput{Door: holdLeaf, Actor: "alice"}))
				}},
			} {
				enc := sidedHall(t, holdLeafDoor(verb.state), tc.from, gatehouseGuard(t, tc.against, holdPass))
				held, _, err := enc.DoorHeldFor("alice", holdLeaf)
				require.NoError(t, err)
				require.Equal(t, tc.barred, held, "%s: DoorHeldFor", verb.name)
				if !tc.barred {
					refused, err := verb.do(enc)
					requireDoorWorked(t, refused, err, verb.name+": not barred from this side")
					continue
				}
				refused, err := verb.do(enc)
				requireBarredDoor(t, refused, err)
				teach(t, enc, "alice", holdPass)
				held, _, err = enc.DoorHeldFor("alice", holdLeaf)
				require.NoError(t, err)
				require.False(t, held, "%s: with the pass, not held", verb.name)
				refused, err = verb.do(enc)
				requireDoorWorked(t, refused, err, verb.name+": with the pass, the verb works")
			}
		})
	}
}

func TestObservedHoldsFollowTheSettings(t *testing.T) {
	porter := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: holdBeside, Table: holds()}
	bob := MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 7, Y: 0}}
	enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast, gatehouseGuard(t, PostEntering, holdPass), porter, bob)
	seen := func(viewer MemberID) map[MemberID]HeldDoor {
		t.Helper()
		held, err := enc.ObservedHolds(&ViewInput{Member: viewer})
		require.NoError(t, err)
		return held
	}
	gatehouseSide := []spatial.Position{holdNorthWest, holdNorthEast}

	require.Equal(t, HeldDoor{Door: holdLeaf, Against: PostEntering, Side: gatehouseSide}, seen("alice")[postedGuard],
		"from the yard, the far side: the door, the direction and the gatehouse cells of its crossings")
	require.Equal(t, HeldDoor{Door: holdLeaf, Against: PostEntering, Side: gatehouseSide}, seen("bob")[postedGuard],
		"from the gatehouse, the side it bars: told the same, whatever region the viewer stands in")
	require.NotContains(t, seen("porter"), postedGuard, "an ally is told nothing")

	teach(t, enc, "alice", holdPass)
	require.NotContains(t, seen("alice"), postedGuard, "a viewer with the pass is told nothing")

	// THE SETTINGS ARE THE LIVE POST'S: changed with no new sighting, the
	// answer follows them.
	for state, p := range enc.members[postedGuard].Posts {
		p.Against, p.Pass = "", ""
		enc.members[postedGuard].Posts[state] = p
	}
	require.Equal(t, HeldDoor{Door: holdLeaf, Against: PostBothWays, Side: gatehouseSide}, seen("alice")[postedGuard],
		"an unset direction is told as both ways, and the pass she knows no longer lets her through")

	t.Run("a both-ways post with no one side tells no side", func(t *testing.T) {
		in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsOpen()), holdSouthEast, gatehouseGuard(t, PostBothWays, ""))
		one, err := NewEncounter(in)
		require.NoError(t, err)
		held, err := one.ObservedHolds(&ViewInput{Member: "alice"})
		require.NoError(t, err)
		require.Equal(t, HeldDoor{Door: holdLeaf, Against: PostBothWays}, held[postedGuard])
	})
}

// TestThePassIsReadOncePerPath: a read asks the journal for a mover's pass
// once, however many held edges its routes ask about. Taught mid-read, the
// read keeps the answer it read first; the next read sees the fact.
func TestThePassIsReadOncePerPath(t *testing.T) {
	enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast, gatehouseGuard(t, PostBothWays, holdPass))
	q, err := enc.cellQuery()
	require.NoError(t, err)

	path, ok, err := q.routeWithin("alice", holdSouthEast, holdNorthEast, 0)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, []spatial.Position{holdNorthEast}, path, "precondition: barred, the route goes round")
	known, read := q.passes[passKey{mover: "alice", fact: holdPass}]
	require.True(t, read, "the route read her pass")
	require.False(t, known)

	teach(t, enc, "alice", holdPass)
	require.True(t, q.heldEdge("alice", holdSouthWest, holdNorthWest), "the same read answers from what it read once")
	require.True(t, q.heldEdge("alice", holdSouthEast, holdNorthEast), "on every held edge")

	fresh, err := enc.cellQuery()
	require.NoError(t, err)
	require.False(t, fresh.heldEdge("alice", holdSouthEast, holdNorthEast), "a new read reads the journal again")
}

func TestAStoredPostKeepsItsSettings(t *testing.T) {
	saved := func(t *testing.T, against PostDirection, pass FactID) []byte {
		t.Helper()
		raw, err := json.Marshal(sidedHall(t, holdLeafDoor(DoorIsOpen()), holdNorthEast,
			gatehouseGuard(t, against, pass)).ToData())
		require.NoError(t, err)

		return raw
	}
	load := func(raw []byte) (*Encounter, error) {
		var data EncounterData
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}

		return LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: postedSetup(everyoneSeesTheWholeMap{}).Capabilities})
	}
	// editPosted rewrites one field of the guard's stored posted post.
	editPosted := func(t *testing.T, raw []byte, key string, value any) []byte {
		t.Helper()
		var doc map[string]any
		require.NoError(t, json.Unmarshal(raw, &doc))
		for _, row := range doc["members"].([]any) {
			m := row.(map[string]any)
			if m["id"] != string(postedGuard) {
				continue
			}
			post := m["posts"].(map[string]any)["posted"].(map[string]any)
			if value == nil {
				delete(post, key)
			} else {
				post[key] = value
			}
		}
		out, err := json.Marshal(doc)
		require.NoError(t, err)

		return out
	}

	t.Run("a direction and a pass round trip", func(t *testing.T) {
		enc, err := load(saved(t, PostEntering, holdPass))
		require.NoError(t, err)
		for _, m := range enc.ToData().Members {
			if m.ID == postedGuard {
				require.Equal(t, PostEntering, m.Posts["posted"].Against)
				require.Equal(t, holdPass, m.Posts["posted"].Pass)
			}
		}
		require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdSouthEast), "the loaded post bars the way out of the gatehouse")
		teach(t, enc, "alice", holdPass)
		steps(t, enc, holdSouthEast, "and lets the pass through")
	})

	t.Run("a post with no settings stores none", func(t *testing.T) {
		raw := saved(t, "", "")
		require.NotContains(t, string(raw), `"against"`)
		require.NotContains(t, string(raw), `"pass"`)
	})

	t.Run("a record without them loads as both ways and no pass", func(t *testing.T) {
		raw := editPosted(t, editPosted(t, saved(t, PostLeaving, holdPass), "against", nil), "pass", nil)
		enc, err := load(raw)
		require.NoError(t, err)
		require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdSouthEast), "both ways: barred out of the gatehouse")
		teach(t, enc, "alice", holdPass)
		require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdSouthEast), "and no pass")
	})

	for name, edit := range map[string]func(t *testing.T, raw []byte) []byte{
		"an unknown direction": func(t *testing.T, raw []byte) []byte { return editPosted(t, raw, "against", "inward") },
		"a direction with no one side": func(t *testing.T, raw []byte) []byte {
			return editPosted(t, raw, "cells", []any{map[string]any{"x": 4, "y": 0}, map[string]any{"x": 3, "y": 1}})
		},
	} {
		t.Run(name+" is refused", func(t *testing.T) {
			_, err := load(edit(t, saved(t, PostEntering, "")))
			require.ErrorIs(t, err, ErrInvalidData)
		})
	}
}

// TestASettingsChangeIsAPostChange: a state change between two posts on the
// same door and cells that differ only in their settings changes what his
// post holds, so his cell goes to the next look as any post change does
// ([Encounter.afterStep], rpg-project#852 H5). Two states with the same
// settings change nothing there.
func TestASettingsChangeIsAPostChange(t *testing.T) {
	for _, tc := range []struct {
		name    string
		alert   PostDirection
		changed bool
	}{
		{"entering to both ways", PostBothWays, true},
		{"entering to entering", PostEntering, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			guard := gatehouseGuard(t, PostEntering, "")
			alert := guard.Posts["alert"]
			alert.Against = tc.alert
			guard.Posts["alert"] = alert
			enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast, guard)
			m := enc.members[postedGuard]
			p := enc.programFor(m)
			require.NotNil(t, p)
			enc.pendingCells, enc.blockingTurned = nil, false

			m.Machine.State = "alert"
			require.NoError(t, enc.afterStep(postedGuard, m, p, machine.Stepped{Fired: true, From: "posted", To: "alert"},
				uint64(enc.clock.ToData().HighWater), false))
			require.Equal(t, tc.changed, slices.Contains(enc.pendingCells, holdNorthWest), "his cell goes to the next look")
			require.Equal(t, tc.changed, enc.blockingTurned)
		})
	}
}

// TestObservedHoldsReadTheAuthoredPostAfterHeLeavesIt: between a state
// change and the next look, the viewer's current sighting still names the
// door while his new state names no post. The settings then come from the
// state of his that posts that door ([Encounter.livePost]), so the viewer is
// told the authored direction and side, not a bare both ways.
func TestObservedHoldsReadTheAuthoredPostAfterHeLeavesIt(t *testing.T) {
	enc := sidedHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast, gatehouseGuard(t, PostEntering, ""))
	enc.members[postedGuard].Machine.State = "cowed"
	_, holds := enc.memberHolds(postedGuard, nil)
	require.False(t, holds, "precondition: cowed, he holds nothing now")

	held, err := enc.ObservedHolds(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, HeldDoor{Door: holdLeaf, Against: PostEntering, Side: []spatial.Position{holdNorthWest, holdNorthEast}},
		held[postedGuard], "the sighting not yet looked again, the settings are the post's he held")
}
