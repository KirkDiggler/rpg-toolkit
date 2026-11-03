// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// postedbody_test.go pins A BODY BLOCKS BY ITS STATE (rpg-project#836 S1,
// ideas/the-guard-holds-the-gate): a creature whose states mark some of them
// blocking blocks exactly while it stands in one, read from the state by the
// fold, the canvas, the observed holding and the walk's own preflight, and
// stored with the recipe it names. Internal, because the machine blob, the
// landed beats and the canvas probe are the composition's own.

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	postedGuard    = MemberID("gate")
	postedGarrison = FactionID("garrison")
	postedCowed    = FactID("gate-cowed")
)

// Where the posted hall's bodies stand: alice at (1,1), the guard on the
// cell east of her, and open floor past him.
var (
	postedAlice = spatial.Position{X: 1, Y: 1}
	postedPost  = spatial.Position{X: 2, Y: 1}
	postedPast  = spatial.Position{X: 3, Y: 1}
)

// postedRecipe is a gate post's states: posted, standing its post on time,
// until it learns gate-cowed; cowed, which stands too and does not block.
func postedGuardRecipe(t *testing.T) *machine.Recipe {
	t.Helper()
	cowed := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineFact(string(postedCowed)), Op: machine.OpEq, Value: true}}

	return recipe(t, "posted", map[string]machine.State{
		"posted": {Transitions: []machine.Transition{
			{On: beatFact, When: cowed, To: "cowed", Reason: "cowed"},
			pickTime,
		}},
		"cowed": {Transitions: []machine.Transition{pickTime}},
	})
}

// postedMember is the garrison's gate guard on its post, blocking while
// posted.
func postedMember(t *testing.T) MemberInput {
	t.Helper()

	return MemberInput{
		ID: postedGuard, Kind: KindMonster, Name: "Gate guard", Faction: postedGarrison,
		Position: postedPost, Table: holds(), Machine: postedGuardRecipe(t), BlockingStates: []string{"posted"},
	}
}

// postedSetup is a 10×5 hall everyone sees across, alice in the party and a
// garrison neutral to it, on the table driver, with every member given.
func postedSetup(sight Sight, members ...MemberInput) *SetupInput {
	in := wakeSetup(&askedDriver{inner: TableDriver{Roller: wakeRollsOne{}}}, members...)
	in.Field.Regions = []RegionInput{rectRegion("hall", 0, 0, 10, 5)}
	in.Field.Factions = []FactionInput{{ID: postedGarrison}}
	in.Field.Dispositions = []DispositionInput{{Between: [2]FactionID{postedGarrison, FactionParty}, Stance: StanceNeutral}}
	in.Retention = RetentionUnbounded
	in.Sight = sight
	in.Sheets = earsSheets{}

	return in
}

// postedHall opens [postedSetup] with everyone seeing everything.
func postedHall(t *testing.T, members ...MemberInput) *Encounter {
	t.Helper()
	enc, err := NewEncounter(postedSetup(everyoneSeesTheWholeMap{}, members...))
	require.NoError(t, err)

	return enc
}

// passageAt is the fold's answer for alice at a cell.
func passageAt(t *testing.T, enc *Encounter, cell spatial.Position) Passage {
	t.Helper()
	fact, err := enc.CellAt(CellAtInput{Cell: cell, Mover: "alice"})
	require.NoError(t, err)

	return fact.Passage
}

// cowGuard teaches the guard gate-cowed and lets a round pass, which is when
// the landed fact is heard.
func cowGuard(t *testing.T, enc *Encounter) {
	t.Helper()
	require.NoError(t, enc.learnFact(postedGuard, postedCowed, BeatAnswered, uint64(enc.clock.ToData().HighWater)))
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "cowed", stateOf(enc, postedGuard), "precondition: the fact moved him off the post")
}

func TestAPostedNeutralBodyBlocksAStepAndARoute(t *testing.T) {
	enc := postedHall(t, postedMember(t))
	hostile, _ := enc.IsHostile("alice", postedGuard)
	require.False(t, hostile, "precondition: the guard is neutral")

	require.Equal(t, PassageBlocked, passageAt(t, enc, postedPost), "a posted body is Blocked")

	_, err := enc.Step(&StepInput{Member: "alice", To: postedPost})
	require.ErrorIs(t, err, ErrBodyInTheWay)
	require.NotErrorIs(t, err, ErrBadPlacement, "a body is not a bad placement")
	var stop *StepObstructedError
	require.ErrorAs(t, err, &stop, "a body stops the walk; it does not break it")
	at, _ := enc.canvas.GetEntityPosition("alice")
	require.Equal(t, postedAlice, at, "the refused step moved nobody")

	q, err := enc.cellQuery()
	require.NoError(t, err)
	path, ok, err := q.routeWithin("alice", postedAlice, postedPast, 0)
	require.NoError(t, err)
	require.True(t, ok, "the open hall has a way round")
	require.NotContains(t, path, postedPost, "the route goes round the posted body")
}

func TestLeavingThePostOpensTheCellInThatBeat(t *testing.T) {
	enc := postedHall(t, postedMember(t))
	_, err := enc.Step(&StepInput{Member: "alice", To: postedPost})
	require.ErrorIs(t, err, ErrBodyInTheWay, "precondition: posted, he holds the cell")

	cowGuard(t, enc)

	require.Equal(t, PassagePassThrough, passageAt(t, enc, postedPost), "cowed, his space is a neutral's again")
	_, err = enc.Step(&StepInput{Member: "alice", To: postedPost})
	require.NoError(t, err, "the next step passes, with nothing reloaded or refreshed by hand")
}

func TestTheCanvasAndTheFoldAgreeAfterATransition(t *testing.T) {
	enc := postedHall(t, postedMember(t))
	probe := &memberEntity{id: "probe", kind: KindPlayer, blocks: func() bool { return false }}

	require.False(t, enc.canvas.CanPlaceEntity(probe, postedPost), "the canvas refuses the posted cell")
	require.Equal(t, PassageBlocked, passageAt(t, enc, postedPost), "and so does the fold")

	cowGuard(t, enc)

	require.True(t, enc.canvas.CanPlaceEntity(probe, postedPost), "the canvas opens it in the transition's beat")
	require.Equal(t, PassagePassThrough, passageAt(t, enc, postedPost), "and so does the fold")
}

func TestWhatAnObserverHoldsFollowsTheState(t *testing.T) {
	enc := postedHall(t, postedMember(t))
	held, err := enc.ObservedPassages(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, PassageBlocked, held[postedGuard], "alice sees him holding the post")

	cowGuard(t, enc)

	held, err = enc.ObservedPassages(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, PassagePassThrough, held[postedGuard], "alice sees him stand aside, in the round he did")
}

func TestAHostileBodyBlocksInAnyState(t *testing.T) {
	guard := postedMember(t)
	guard.Faction = ""
	enc := postedHall(t, guard)
	hostile, _ := enc.IsHostile("alice", postedGuard)
	require.True(t, hostile, "precondition: a monster of the monsters' side")

	enc.members[postedGuard].Machine.State = "cowed"
	require.False(t, enc.memberBlocks(postedGuard), "precondition: cowed does not block")
	require.Equal(t, PassageBlocked, passageAt(t, enc, postedPost), "hostility blocks whatever the state")
}

func TestATabledMonsterDeclaredBlockingStillBlocks(t *testing.T) {
	for _, declared := range []bool{true, false} {
		guard := MemberInput{
			ID: postedGuard, Kind: KindMonster, Faction: postedGarrison, Position: postedPost,
			Table: holds(), BlocksMovement: declared,
		}
		enc := postedHall(t, guard)
		require.NotNil(t, enc.programFor(enc.members[postedGuard]), "precondition: he runs the default machine")
		want := PassagePassThrough
		if declared {
			want = PassageBlocked
		}
		require.Equal(t, want, passageAt(t, enc, postedPost), "declared %v: the creation's word stands", declared)
	}
}

func TestBlockingStatesAreRefusedWithoutARecipe(t *testing.T) {
	guard := postedMember(t)
	guard.Machine = nil
	_, err := NewEncounter(postedSetup(everyoneSeesTheWholeMap{}, guard))
	require.ErrorIs(t, err, ErrBadMember, "a blocking state with no recipe to name it")

	enc := postedHall(t)
	_, err = enc.Join(&JoinInput{
		Member: postedGuard, Kind: KindMonster, Faction: postedGarrison, Cell: postedPost,
		Table: holds(), BlockingStates: []string{"posted"},
	})
	require.ErrorIs(t, err, ErrBadMember, "and at the join door")
}

func TestBlockingStatesAreRefusedForAnUnknownState(t *testing.T) {
	guard := postedMember(t)
	guard.BlockingStates = []string{"posted", "asleep"}
	_, err := NewEncounter(postedSetup(everyoneSeesTheWholeMap{}, guard))
	require.ErrorIs(t, err, ErrBadMember)
	require.ErrorContains(t, err, "asleep")
}

func TestABodyRefusalNamesTheBody(t *testing.T) {
	reason := func(t *testing.T, sight Sight) string {
		t.Helper()
		enc, err := NewEncounter(postedSetup(sight, postedMember(t)))
		require.NoError(t, err)
		_, err = enc.Step(&StepInput{Member: "alice", To: postedPost})
		var stop *StepObstructedError
		require.ErrorAs(t, err, &stop)

		return stop.PublicReason()
	}

	require.Equal(t, "Gate guard stands in the way", reason(t, everyoneSeesTheWholeMap{}), "a body she sees is named")
	require.Equal(t, "someone stands in the way", reason(t, wakeBlind{}), "a body she does not see is someone")
}

func TestADrivenWalkStopsSilentlyAtABody(t *testing.T) {
	enc := postedHall(t, postedMember(t))
	_, stepped, err := enc.stepTo(enc.members["alice"], postedPost)
	require.NoError(t, err, "a driven walk swallows the body's refusal")
	require.False(t, stepped, "and stops")
}

func TestAWalkMayAimAtASeenNeutralBodyButNotAHostileOne(t *testing.T) {
	enc := postedHall(t, postedMember(t))
	require.NoError(t, enc.ValidateWalkEnd(CellAtInput{Mover: "alice", Cell: postedPost}),
		"walking into a posted neutral is the gesture: the step stops it, the preflight does not")

	guard := postedMember(t)
	guard.Faction = ""
	hostile := postedHall(t, guard)
	require.ErrorIs(t, hostile.ValidateWalkEnd(CellAtInput{Mover: "alice", Cell: postedPost}), ErrBadPlacement,
		"a hostile body stays unplannable")
}

func TestAStoredEncounterLoadsItsBlocking(t *testing.T) {
	waiting := postedMember(t)
	waiting.ID, waiting.Position, waiting.Arrives = "relief", spatial.Position{X: 6, Y: 3}, TriggerRound{Round: 99}

	saved := func(t *testing.T) []byte {
		t.Helper()
		raw, err := json.Marshal(postedHall(t, postedMember(t), waiting).ToData())
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
	// edit rewrites the stored blocking states of every member and reserve:
	// nil drops the key, as a record from before it existed.
	edit := func(t *testing.T, raw []byte, states []string) []byte {
		t.Helper()
		var doc map[string]any
		require.NoError(t, json.Unmarshal(raw, &doc))
		for _, list := range []string{"members", "reserve"} {
			rows, _ := doc[list].([]any)
			for _, row := range rows {
				m := row.(map[string]any)
				if states == nil {
					delete(m, "blocking_states")
				} else if _, ok := m["blocking_states"]; ok {
					m["blocking_states"] = states
				}
			}
		}
		out, err := json.Marshal(doc)
		require.NoError(t, err)

		return out
	}

	t.Run("a member and a reserve keep their blocking states", func(t *testing.T) {
		raw := saved(t)
		enc, err := load(raw)
		require.NoError(t, err)
		data := enc.ToData()
		var member []string
		for _, m := range data.Members {
			if m.ID == postedGuard {
				member = m.BlockingStates
			}
		}
		require.Equal(t, []string{"posted"}, member)
		require.True(t, slices.ContainsFunc(data.Reserve, func(r ReserveData) bool {
			return r.ID == "relief" && slices.Equal(r.BlockingStates, []string{"posted"})
		}), "the reserve keeps its blocking states too")
		_, err = enc.Step(&StepInput{Member: "alice", To: postedPost})
		require.ErrorIs(t, err, ErrBodyInTheWay, "and the loaded guard still holds his post")
	})

	t.Run("a record without them loads as today", func(t *testing.T) {
		enc, err := load(edit(t, saved(t), nil))
		require.NoError(t, err)
		require.Equal(t, PassagePassThrough, passageAt(t, enc, postedPost), "no blocking states: the creation's word, false")
	})

	t.Run("a stored state the recipe lacks is refused", func(t *testing.T) {
		_, err := load(edit(t, saved(t), []string{"asleep"}))
		require.ErrorIs(t, err, ErrInvalidData)
		require.True(t, errors.Is(err, ErrInvalidData))
	})
}

// postedDown reports the members in its set down, everyone else standing.
type postedDown map[MemberID]bool

func (d postedDown) Standing([]MemberID) ([]MemberID, error) { return nil, nil }

func (d postedDown) Assess(members []MemberID) (*ParticipationAssessment, error) {
	var reported []MemberID
	for id := range d {
		reported = append(reported, id)
	}

	return testAssessmentFromDown(members, reported), nil
}

// TestADownedPostedBodyIsStoodOn: a posted guard who is down holds nothing.
// The fold calls a downed monster's cell standable whatever its state, and
// the canvas agrees, so the step that the fold allows is taken (B3).
func TestADownedPostedBodyIsStoodOn(t *testing.T) {
	down := postedDown{}
	in := postedSetup(everyoneSeesTheWholeMap{}, postedMember(t))
	in.Participation = down
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	require.Equal(t, PassageBlocked, passageAt(t, enc, postedPost), "precondition: standing, he holds his post")

	down[postedGuard] = true
	require.Equal(t, "posted", stateOf(enc, postedGuard), "precondition: still posted")
	require.Equal(t, PassageStandable, passageAt(t, enc, postedPost), "the fold: a downed monster's cell is standable")
	_, err = enc.Step(&StepInput{Member: "alice", To: postedPost, EndWalk: true})
	require.NoError(t, err, "and the canvas agrees: the step is taken")
}
