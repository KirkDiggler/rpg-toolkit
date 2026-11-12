// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// barred_test.go pins THE BUMP IS A BEAT THE BODY HEARS (rpg-project#842, S2
// of the guard holds the gate; design R2 and R7): a player's step refused by
// a body, on its cell or across a crossing its post holds, and a door verb a
// post holds, write `barred`; the barrier alone hears it, at once, in the
// verb that bumped; his table answers in his voice; the verb answers with
// output, never an error, and spends nothing. Internal, because the landed
// beats, the machine blob and the pace are the composition's own.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The guard's two lines: the first bump's question and the shorter one a
// bump gets while he is still challenging.
const (
	bumpLine      = "Whoa there, buddy. Can I help you?"
	bumpAgainLine = "I said, can I help you?"
	bumpKnown     = FactID("party-recognized")
)

// bumpRecipe is a gate post that answers a bump: posted, a bump moves him to
// challenged and picks `barred`; challenged, a bump picks `challenged`, and
// three rounds on he is posted again. Cowed holds nothing and hears no bump.
func bumpRecipe(t *testing.T) *machine.Recipe {
	t.Helper()
	cowed := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineFact(string(postedCowed)), Op: machine.OpEq, Value: true}}

	return recipe(t, "posted", map[string]machine.State{
		"posted": {Transitions: []machine.Transition{
			{On: beatFact, When: cowed, To: "cowed", Reason: "cowed"},
			{On: beatBarred, To: "challenged", Do: []machine.Act{{Pick: "barred"}, {Anchor: true}}, Reason: "bumped"},
			{On: beatTurn, Do: []machine.Act{{Pick: string(AnswerTime)}}},
			pickTime,
		}},
		"challenged": {Transitions: []machine.Transition{
			{On: beatFact, When: cowed, To: "cowed", Reason: "cowed"},
			{On: beatBarred, Do: []machine.Act{{Pick: "challenged"}}},
			{On: beatTurn, Do: []machine.Act{{Pick: string(AnswerTime)}}},
			{On: beatTime, When: &machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: 3}}, To: "posted", Reason: "nothing came of it"},
			pickTime,
		}},
		"cowed": {Transitions: []machine.Transition{pickTime}},
	})
}

// bumpTable is a gate post's table: he holds on time, and says his lines.
func bumpTable() Table {
	t := holds()
	t["barred"] = []Answer{{Weight: 1, Say: bumpLine}}
	t["challenged"] = []Answer{{Weight: 1, Say: bumpAgainLine}}

	return t
}

// bumpGuard is the posted hall's guard, answering a bump.
func bumpGuard(t *testing.T) MemberInput {
	t.Helper()
	g := postedMember(t)
	g.Behaviors.Instances[0].Instance.Recipe = *bumpRecipe(t)
	g.Behaviors.Instances[0].Instance.Table = bumpTable()
	g.BlockingStates = []string{"posted", "challenged"}

	return g
}

// bumpHolder is [holder] answering a bump: posted and challenged hold the
// door at the cells given.
func bumpHolder(t *testing.T, id MemberID, at spatial.Position, door DoorID, cells ...spatial.Position) MemberInput {
	t.Helper()
	h := holder(t, id, at, door, cells...)
	post := Post{Door: door, Cells: cells}
	h.Behaviors.Instances[0].Instance.Recipe = *bumpRecipe(t)
	h.Behaviors.Instances[0].Instance.Table = bumpTable()
	h.BlockingStates = []string{"posted", "challenged"}
	h.Posts = map[string]Post{"posted": post, "challenged": post}

	return h
}

// bumpEar is a garrison bystander who listens: posted until a sound within
// six hexes, then alert.
func bumpEar(t *testing.T) MemberInput {
	t.Helper()
	ear := listener("ear", 6, 3, postedRecipe(t))
	ear.Faction = postedGarrison

	return ear
}

// storyOf is every beat alice is told, in record order, decoded.
func storyOf(t *testing.T, enc *Encounter) []map[string]any {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	out := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		beat["seq"] = float64(entry.Seq)
		out = append(out, beat)
	}

	return out
}

// bumped steps alice into a body and requires a stop: no error, nothing
// moved, the output's stop naming the barrier.
func bumped(t *testing.T, enc *Encounter, to spatial.Position, barrier MemberID) *StepOutput {
	t.Helper()
	before, _ := enc.canvas.GetEntityPosition("alice")
	out, err := enc.Step(&StepInput{Member: "alice", To: to})
	require.NoError(t, err, "a bump is answered, not refused")
	require.NotNil(t, out.Stopped, "the step was stopped by a body")
	require.Equal(t, barrier, out.Stopped.Barrier)
	require.Equal(t, to, out.Stopped.Cell, "stopped at the cell it was aimed at")
	after, _ := enc.canvas.GetEntityPosition("alice")
	require.Equal(t, before, after, "the bump moved nobody")
	require.Equal(t, before, out.Stepped.From)
	require.Equal(t, before, out.Stepped.To, "the output reports no movement")

	return out
}

// answersBy is every answered beat of one creature, in record order.
// lastSaid is the words of the last line a creature said, from alice's
// speech versions (rpg-project#933), empty when she caught none.
func lastSaid(t *testing.T, enc *Encounter, creature MemberID) string {
	t.Helper()
	said := ""
	for _, b := range storyOf(t, enc) {
		if b["beat"] == BeatSpeech && b["speaker"] == string(creature) {
			if w, ok := b["words"].(string); ok {
				said = w
			}
		}
	}
	return said
}

func answersBy(t *testing.T, enc *Encounter, creature MemberID) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, beat := range beatsTagged(t, enc, BeatAnswered) {
		if beat["creature"] == string(creature) {
			out = append(out, beat)
		}
	}

	return out
}

func TestABumpWritesBarredThenTheAnswer(t *testing.T) {
	enc := postedHall(t, bumpGuard(t))

	out := bumped(t, enc, postedPost, postedGuard)
	require.Equal(t, "Gate guard stands in the way", out.Stopped.Reason, "the stop names the body she sees")

	var barred, state, answered map[string]any
	for _, beat := range storyOf(t, enc) {
		switch {
		case beat["beat"] == BeatBarred:
			barred = beat
		case beat["beat"] == BeatState && beat["member"] == string(postedGuard):
			state = beat
		case beat["beat"] == BeatAnswered && beat["creature"] == string(postedGuard) && beat["key"] == "behavior/6:policy/barred":
			answered = beat
		}
	}
	require.NotNil(t, barred, "the bump is on the record")
	require.Equal(t, "alice", barred["mover"])
	require.Equal(t, string(postedGuard), barred["barrier"])
	require.Equal(t, map[string]any{"x": postedPost.X, "y": postedPost.Y}, barred["cell"])
	require.NotContains(t, barred, "door", "a step names no door")
	require.Equal(t, float64(out.Seq), barred["seq"], "the output's seq is the bump's")
	// AND ITS ID IS THE BUMP'S (rpg-project#879 S1): the record id a stream
	// consumer resumes after names the barred beat, the same entry as Seq.
	require.NotEmpty(t, out.ID, "a bump names the id of the beat it wrote")
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	var barredID string
	for _, entry := range story {
		if entry.Seq == out.Seq {
			barredID = entry.ID
		}
	}
	require.Equal(t, barredID, out.ID, "the output's id is the barred beat's")

	require.NotNil(t, state, "he heard it at once: his state turned in this step")
	require.Equal(t, "challenged", state["to"])
	require.NotNil(t, answered, "and his table answered")
	require.NotNil(t, answeredSaying(t, enc, postedGuard, bumpLine), "in his own words, in alice's speech version")
	require.Less(t, barred["seq"].(float64), state["seq"].(float64), "the bump first")
	require.Less(t, state["seq"].(float64), answered["seq"].(float64), "then the turn, then the answer")
}

func TestBarredIsAScopedBeat(t *testing.T) {
	require.True(t, ValidMachineBeat("barred"))
	require.True(t, MachineBeatScoped("barred"), "a bump happened somewhere")
	scoped := recipe(t, "posted", map[string]machine.State{"posted": {Transitions: []machine.Transition{
		{On: beatBarred, When: &machine.Guard{Scope: &machine.ScopeLeaf{Within: 1}}, Do: []machine.Act{{Pick: "barred"}}},
	}}})
	require.NoError(t, ValidateMachine(scoped), "a bump within a hex compiles")
}

func TestAHeldEdgeBumpAddressesTheNearestHolder(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		bumpHolder(t, "far", holdOffPost, holdLeaf, holdOffPost),
		bumpHolder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))

	out := bumped(t, enc, holdNorthEast, postedGuard)
	require.Equal(t, "Gate guard bars the way", out.Stopped.Reason)

	barred := beatsTagged(t, enc, BeatBarred)
	require.NotEmpty(t, barred)
	require.Equal(t, string(postedGuard), barred[len(barred)-1]["barrier"], "the beat is addressed to the nearest holder")
	require.Equal(t, map[string]any{"x": holdNorthEast.X, "y": holdNorthEast.Y}, barred[len(barred)-1]["cell"],
		"at the far cell of the crossing, where the step was refused into")
	require.Equal(t, "challenged", stateOf(enc, postedGuard), "who heard it")
	require.Equal(t, "posted", stateOf(enc, "far"), "and the other holder did not")
	require.Empty(t, answersBy(t, enc, "far"), "nor answered")
}

func TestABumpMatchesOnlyItsOwnBeat(t *testing.T) {
	enc := postedHall(t, bumpGuard(t), bumpEar(t))
	require.NoError(t, enc.makeSound(noiseAttack, "alice", uint64(enc.clock.ToData().HighWater)))
	clang := func() bool {
		for _, b := range enc.landed {
			if b.Beat == beatHeard && b.Member == "ear" && b.Subject == "alice" {
				return true
			}
		}
		return false
	}
	require.True(t, clang(), "precondition: a clang short of a pace waits for the raise")

	bumped(t, enc, postedPost, postedGuard)

	require.Equal(t, "challenged", stateOf(enc, postedGuard), "the bump was heard")
	require.True(t, clang(), "the clang still waits")
	require.Equal(t, "posted", stateOf(enc, "ear"), "and nobody heard it early")
	for _, b := range enc.landed {
		require.NotEqual(t, beatBarred, b.Beat, "a bump is never landed")
	}
}

func TestBystandersHearTheAnswerOnTheNextRaise(t *testing.T) {
	enc := postedHall(t, bumpGuard(t), bumpEar(t))

	bumped(t, enc, postedPost, postedGuard)
	require.Equal(t, "posted", stateOf(enc, "ear"), "his voice waits for time to pass")
	voice := false
	for _, b := range enc.landed {
		voice = voice || (b.Beat == beatHeard && b.Member == "ear" && b.Subject == postedGuard)
	}
	require.True(t, voice, "his voice landed on the bystander")

	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "alert", stateOf(enc, "ear"), "the raise brings it")
}

func TestBumpingAWallWritesNothingNew(t *testing.T) {
	cases := map[string]struct {
		open func(t *testing.T) *Encounter
		to   spatial.Position
	}{
		"a wall, with him behind it": {
			open: func(t *testing.T) *Encounter {
				in := postedSetup(everyoneSeesTheWholeMap{}, bumpGuard(t))
				in.Field.Walls = sealedSeam(1, 5)
				enc, err := NewEncounter(in)
				require.NoError(t, err)
				return enc
			},
			to: postedPost,
		},
		"a shut door": {
			open: func(t *testing.T) *Encounter {
				return holdHall(t, holdEdgeDoor(DoorIsClosed()), holdSouthEast, bumpGuard(t))
			},
			to: holdNorthEast,
		},
		"a masked wall with a holder behind it": {
			open: func(t *testing.T) *Encounter {
				in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsClosed()), holdSouthEast,
					bumpHolder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
				in.Field.Concealments = []ConcealmentInput{{ID: "secret-gate", Checks: []CheckApproach{{Ability: "perception", DC: 30}}, Doors: []DoorID{holdLeaf}}}
				in.CheckResolver = holdNeverFinds{}
				in.Witness = nobodyPerceives{}
				enc, err := NewEncounter(in)
				require.NoError(t, err)
				require.True(t, enc.hiddenDoorTo("alice", holdLeaf), "precondition: alice has not found the door")
				return enc
			},
			to: holdNorthEast,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			enc := tc.open(t)
			before := len(storyOf(t, enc))

			out, err := enc.Step(&StepInput{Member: "alice", To: tc.to})
			require.Error(t, err, "%s refuses the step as it always has", name)
			require.Nil(t, out)
			var stop *StepObstructedError
			require.ErrorAs(t, err, &stop, "and stops the walk")
			require.NotErrorIs(t, err, ErrBodyInTheWay)

			require.Len(t, storyOf(t, enc), before, "%s writes nothing new", name)
			require.Equal(t, "posted", stateOf(enc, postedGuard), "and nobody heard anything")
		})
	}
}

func TestADrivenWalkWritesNoBarred(t *testing.T) {
	t.Run("a push into him", func(t *testing.T) {
		enc := postedHall(t, bumpGuard(t))
		out, err := enc.Direct(context.Background(), DirectInput{
			Mover: "alice", Cause: core.Ref{Module: "test", Type: "effects", ID: "shove"},
			Route: []spatial.Position{postedPost, postedPast}, Kind: MoveDisplaced,
		})
		require.NoError(t, err)
		require.Zero(t, out.Moved, "the push stops at him")
		require.Empty(t, beatsTagged(t, enc, BeatBarred), "a driven walk writes no bump")
		require.Equal(t, "posted", stateOf(enc, postedGuard))
	})

	t.Run("a creature's own step into him", func(t *testing.T) {
		enc := postedHall(t, bumpGuard(t), monster("porter", 3, 1, holds()))
		_, moved, err := enc.stepTo(enc.members["porter"], postedPost)
		require.NoError(t, err)
		require.False(t, moved, "the step stops silently")
		require.Empty(t, beatsTagged(t, enc, BeatBarred), "and writes no bump")
		require.Equal(t, "posted", stateOf(enc, postedGuard))
	})
}

func TestASecondBumpGetsTheChallengedRow(t *testing.T) {
	enc := postedHall(t, bumpGuard(t))

	bumped(t, enc, postedPost, postedGuard)
	bumped(t, enc, postedPost, postedGuard)

	answers := answersBy(t, enc, postedGuard)
	require.NotEmpty(t, answers)
	last := answers[len(answers)-1]
	require.Equal(t, "behavior/6:policy/challenged", last["key"], "a bump while challenging picks the shorter row")
	require.NotNil(t, answeredSaying(t, enc, postedGuard, bumpAgainLine))
	require.Equal(t, "challenged", stateOf(enc, postedGuard))

	for range 3 {
		require.NoError(t, enc.spendWorldAction("alice"))
	}
	require.Equal(t, "posted", stateOf(enc, postedGuard), "three rounds on, he is posted again")

	bumped(t, enc, postedPost, postedGuard)
	require.Equal(t, bumpLine, lastSaid(t, enc, postedGuard), "and asks the question afresh")
}

func TestABumpWithNoListenerWritesOnlyBarred(t *testing.T) {
	enc := postedHall(t, postedMember(t))
	before := storyOf(t, enc)

	out := bumped(t, enc, postedPost, postedGuard)

	after := storyOf(t, enc)
	require.Len(t, after, len(before)+1, "one beat")
	require.Equal(t, BeatBarred, after[len(after)-1]["beat"], "the bump")
	require.Nil(t, out.Formed)
	require.Nil(t, out.Outcome)
	require.Empty(t, out.IntelDeltas, "nothing answered, so nothing was looked at")
}

func TestABumpSpendsNothing(t *testing.T) {
	enc := postedHall(t, bumpGuard(t))
	high := enc.clock.ToData().HighWater
	pace := enc.members["alice"].PaceCells

	// More bumps than her pace's cells in a round: a step that paid would
	// have raised the world by now.
	for range 8 {
		bumped(t, enc, postedPost, postedGuard)
	}

	require.Equal(t, high, enc.clock.ToData().HighWater, "no round passed")
	require.Equal(t, pace, enc.members["alice"].PaceCells, "and no pace was spent")
}

func TestABumpThatFormsAFightReturnsIt(t *testing.T) {
	g := bumpGuard(t)
	g.Behaviors.Instances[0].Instance.Table["barred"] = []Answer{{Weight: 1, Say: "It's them!", Fact: string(bumpKnown)}}
	in := postedSetup(everyoneSeesTheWholeMap{}, g)
	in.Field.Factions = []FactionInput{{ID: postedGarrison, Mind: postedGuard}}
	in.Field.Dispositions = []DispositionInput{{
		Between: [2]FactionID{postedGarrison, FactionParty}, Stance: StanceNeutral,
		Until: []UntilStep{{When: TriggerFact{Fact: bumpKnown}, Then: StanceHostile}},
	}}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	out := bumped(t, enc, postedPost, postedGuard)

	require.NotNil(t, out.Formed, "the fight his answer formed comes back on the output")
	require.Contains(t, out.Formed.Order, MemberID("alice"))
	require.Contains(t, out.Formed.Order, postedGuard)
	require.NotEmpty(t, beatsTagged(t, enc, BeatFightStarted), "and is on the record")
	require.NotEmpty(t, out.IntelDeltas, "with what the look saw of it")
}

// heldDoorVerb is one door verb on the hold hall's leaf, alice's attempt,
// with the state the door starts in and its refusal and state off the output.
type heldDoorVerb struct {
	start DoorState
	try   func(enc *Encounter) (*DoorRefusal, DoorStateKind, uint64, error)
}

func heldDoorVerbs() map[string]heldDoorVerb {
	return map[string]heldDoorVerb{
		"open": {start: DoorIsClosed(), try: func(enc *Encounter) (*DoorRefusal, DoorStateKind, uint64, error) {
			out, err := enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"})
			if err != nil {
				return nil, "", 0, err
			}
			return out.Refused, out.State, out.Seq, nil
		}},
		"close": {start: DoorIsOpen(), try: func(enc *Encounter) (*DoorRefusal, DoorStateKind, uint64, error) {
			out, err := enc.CloseDoor(&CloseDoorInput{Door: holdLeaf, Actor: "alice"})
			if err != nil {
				return nil, "", 0, err
			}
			return out.Refused, out.State, out.Seq, nil
		}},
		"unlock": {start: DoorIsLocked(Lock{Approaches: []CheckApproach{{Ability: "dexterity", DC: 12}}}), try: func(enc *Encounter) (*DoorRefusal, DoorStateKind, uint64, error) {
			out, err := enc.Unlock(&UnlockInput{Door: holdLeaf, Actor: "alice", Beaten: true, Applied: CheckApproach{Ability: "dexterity", DC: 12}})
			if err != nil {
				return nil, "", 0, err
			}
			return out.Refused, out.State, out.Seq, nil
		}},
	}
}

func TestARefusedDoorVerbIsABump(t *testing.T) {
	for name, verb := range heldDoorVerbs() {
		t.Run(name, func(t *testing.T) {
			enc := holdHall(t, holdLeafDoor(verb.start), holdSouthEast,
				bumpHolder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest))
			was := enc.doorsByID[holdLeaf].state.Kind()

			refused, state, seq, err := verb.try(enc)

			require.NoError(t, err, "a held door's verb is answered, not refused")
			require.NotNil(t, refused, "the output says the door is held")
			require.Equal(t, postedGuard, refused.Holder)
			require.Equal(t, "Gate guard bars the door", refused.Reason)
			require.Equal(t, was, state, "the door is as it was")
			require.Equal(t, was, enc.doorsByID[holdLeaf].state.Kind())

			barred := beatsTagged(t, enc, BeatBarred)
			require.Len(t, barred, 1, "the attempt is the bump")
			require.Equal(t, "alice", barred[0]["mover"])
			require.Equal(t, string(postedGuard), barred[0]["barrier"])
			require.Equal(t, string(holdLeaf), barred[0]["door"], "a door verb names its door")
			require.Equal(t, name, barred[0]["verb"], "and itself")
			require.NotContains(t, barred[0], "cell", "a door verb names its door, not a cell")
			require.Empty(t, beatsTagged(t, enc, BeatDoor), "and the door wrote nothing")
			require.Equal(t, "challenged", stateOf(enc, postedGuard), "he heard it")
			answers := answersBy(t, enc, postedGuard)
			require.NotEmpty(t, answers)
			require.NotNil(t, answeredSaying(t, enc, postedGuard, bumpLine), "and answered")

			story := storyOf(t, enc)
			for _, beat := range story {
				if beat["beat"] == BeatBarred {
					require.Equal(t, float64(seq), beat["seq"], "the output's seq is the bump's")
				}
			}
		})
	}
}

func TestAnAllyWorksAHeldDoorWithNoBump(t *testing.T) {
	porter := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: holdSouthEast, Behaviors: ordinaryBehavior(holds())}
	enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdNorthEast,
		bumpHolder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest), porter)

	out, err := enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "porter"})
	require.NoError(t, err)
	require.Nil(t, out.Refused, "his own side works the door")
	require.Equal(t, DoorOpen, out.State)
	require.Empty(t, beatsTagged(t, enc, BeatBarred), "and bumps nobody")
}

// scopedHolder is [bumpHolder] whose posted state hears a bump only within
// the hexes given, measured from the cell the bump names.
func scopedHolder(t *testing.T, within int, at spatial.Position, door DoorID) MemberInput {
	t.Helper()
	h := bumpHolder(t, postedGuard, at, door, at)
	posted := h.Behaviors.Instances[0].Instance.Recipe.States["posted"]
	for i, tr := range posted.Transitions {
		if tr.On == beatBarred {
			tr.When = &machine.Guard{Scope: &machine.ScopeLeaf{Within: within}}
			posted.Transitions[i] = tr
		}
	}
	h.Behaviors.Instances[0].Instance.Recipe.States["posted"] = posted

	return h
}

// TestABumpIsHeardWithinItsScope: a holder three hexes from the far cell of
// the crossing ignores a bump he hears only within one hex, and answers one
// he hears within three. The bump is measured from the cell it names.
func TestABumpIsHeardWithinItsScope(t *testing.T) {
	for within, answers := range map[int]bool{1: false, 3: true} {
		enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast, scopedHolder(t, within, holdOffPost, holdLeaf))
		require.InDelta(t, 3.0, enc.Distance(holdOffPost, holdNorthEast), 0, "precondition: three hexes off")

		bumped(t, enc, holdNorthEast, postedGuard)

		require.Len(t, beatsTagged(t, enc, BeatBarred), 1, "within %d: the bump is written either way", within)
		require.Equal(t, answers, len(answersBy(t, enc, postedGuard)) > 0, "within %d", within)
	}
}

// TestADoorBumpIsMeasuredFromTheDoorBesideTheActor: alice at the edge door's
// west side tries it; the bump is measured from the door's cell nearest her,
// four hexes from a holder posted off to the east, never from the door's
// cell nearest him (three) nor from where she stands (five).
func TestADoorBumpIsMeasuredFromTheDoorBesideTheActor(t *testing.T) {
	for within, answers := range map[int]bool{3: false, 4: true} {
		enc := holdHall(t, holdEdgeDoor(DoorIsClosed()), holdSouthWest, scopedHolder(t, within, holdOffPost, holdEdge))
		require.InDelta(t, 5.0, enc.Distance(holdOffPost, holdSouthWest), 0, "precondition: she stands five off")

		refused, err := openRefusal(enc.OpenDoor(&OpenDoorInput{Door: holdEdge, Actor: "alice"}))
		requireDoorHeld(t, refused, err, "Gate guard bars the door")

		require.Equal(t, answers, len(answersBy(t, enc, postedGuard)) > 0, "within %d", within)
	}
}

// bumpActs is the correlation of every beat from the bump on, by beat kind,
// requiring the bump, his turn, his answer and alice's version of his line
// (rpg-project#933: a player is written a speech version, not a voice
// `heard`) are among them.
func bumpActs(t *testing.T, enc *Encounter) map[string]string {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	out := map[string]string{}
	from := false
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		kind, _ := beat["beat"].(string)
		from = from || kind == BeatBarred
		if from {
			out[kind] = entry.Correlation
		}
	}
	for _, kind := range []string{BeatBarred, BeatState, BeatAnswered, BeatSpeech} {
		require.Contains(t, out, kind, "the bump wrote %s", kind)
	}

	return out
}

// TestTheAnswerIsInTheBumpsAct is act.go's Law 2 for a bump: a reaction
// reopens its trigger's act. The bump, his turn, his answer and his voice
// share the act of the verb that bumped, a step's or a door verb's, so a
// reader ties his line to the bump.
func TestTheAnswerIsInTheBumpsAct(t *testing.T) {
	t.Run("a step", func(t *testing.T) {
		enc := postedHall(t, bumpGuard(t))
		var act string
		require.NoError(t, enc.inAct("move", "", func() error {
			act = enc.CurrentAct()
			bumped(t, enc, postedPost, postedGuard)
			return nil
		}))
		for kind, corr := range bumpActs(t, enc) {
			require.Equal(t, act, corr, "%s is in the move's act", kind)
		}
	})

	t.Run("a door verb", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast,
			bumpHolder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest))
		var act string
		require.NoError(t, enc.inAct("open_door", "", func() error {
			act = enc.CurrentAct()
			refused, err := openRefusal(enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"}))
			requireDoorHeld(t, refused, err, "Gate guard bars the door")
			return nil
		}))
		for kind, corr := range bumpActs(t, enc) {
			require.Equal(t, act, corr, "%s is in the door verb's act", kind)
		}
	})
}

// guardFirst rolls the gate guard to the top of a fight's order.
type guardFirst struct{}

func (guardFirst) RollInitiative(members []MemberID) ([]MemberID, error) {
	out := []MemberID{}
	for _, id := range members {
		if id == postedGuard {
			out = append([]MemberID{id}, out...)
			continue
		}
		out = append(out, id)
	}

	return out, nil
}

// TestAFightTurnAfterABumpMintsItsOwnAct: only a world-clock reaction
// reopens the bump's act. When his answer starts a fight and he acts first,
// his first fight turn is a driven activation of its own, a `time:` act.
func TestAFightTurnAfterABumpMintsItsOwnAct(t *testing.T) {
	g := bumpGuard(t)
	g.Behaviors.Instances[0].Instance.Table["barred"] = []Answer{{Weight: 1, Say: "It's them!", Fact: string(bumpKnown)}}
	in := postedSetup(everyoneSeesTheWholeMap{}, g)
	in.Field.Factions = []FactionInput{{ID: postedGarrison, Mind: postedGuard}}
	in.Field.Dispositions = []DispositionInput{{
		Between: [2]FactionID{postedGarrison, FactionParty}, Stance: StanceNeutral,
		Until: []UntilStep{{When: TriggerFact{Fact: bumpKnown}, Then: StanceHostile}},
	}}
	in.Initiative = guardFirst{}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	var act string
	require.NoError(t, enc.inAct("move", "", func() error {
		act = enc.CurrentAct()
		out := bumped(t, enc, postedPost, postedGuard)
		require.NotNil(t, out.Formed, "precondition: his answer started a fight")
		require.Equal(t, postedGuard, out.Formed.Order[0], "precondition: and he acts first")
		return nil
	}))

	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	// His answer is his first answered beat; his fight turn's pick is the
	// first one after the fight formed.
	var answer, turn string
	formed := false
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		formed = formed || beat["beat"] == BeatFightStarted
		if beat["beat"] != BeatAnswered || beat["creature"] != string(postedGuard) {
			continue
		}
		switch {
		case answer == "":
			answer = entry.Correlation
		case formed && turn == "":
			turn = entry.Correlation
		}
	}
	require.Equal(t, act, answer, "his answer is in the bump's act")
	require.NotEmpty(t, turn, "precondition: he took his fight turn")
	require.NotEqual(t, act, turn, "his fight turn is not")
	require.True(t, strings.HasPrefix(turn, "time:"), "it mints its own act: %s", turn)
}

// TestAWalkMayEndOnABodyItSeesBlocking is R8 (rpg-project#835): a click on
// a blocking body sends a walk with his cell as its goal. The walk end
// accepts it, because the last step is refused and writes the bump, and the
// mover ends on the cell before him.
func TestAWalkMayEndOnABodyItSeesBlocking(t *testing.T) {
	enc := postedHall(t, bumpGuard(t))
	held, err := enc.ObservedPassages(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, PassageBlocked, held[postedGuard], "precondition: alice sees him blocking")

	require.NoError(t, enc.ValidateWalkEnd(CellAtInput{Cell: postedPost, Mover: "alice"}), "his cell is a walk's goal")

	require.NoError(t, enc.inAct("move", "", func() error {
		act := enc.CurrentAct()
		out, err := enc.Step(&StepInput{Member: "alice", To: postedPost, EndWalk: true})
		stop := requireStopped(t, out, err, "the last step is the bump")
		require.Equal(t, postedGuard, stop.Barrier)
		for kind, corr := range bumpActs(t, enc) {
			require.Equal(t, act, corr, "%s is in the walk's act", kind)
		}
		return nil
	}))
}

// TestAWalkMayNotEndOnABodyItSeesPassThrough: nobody ends on a neutral's
// cell that does not block, so its cell is still refused as a goal.
func TestAWalkMayNotEndOnABodyItSeesPassThrough(t *testing.T) {
	bystander := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: postedPost, Behaviors: ordinaryBehavior(holds())}
	enc := postedHall(t, bystander)
	held, err := enc.ObservedPassages(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, PassagePassThrough, held["porter"], "precondition: alice sees him pass-through")

	require.ErrorIs(t, enc.ValidateWalkEnd(CellAtInput{Cell: postedPost, Mover: "alice"}), ErrBadPlacement)
}
