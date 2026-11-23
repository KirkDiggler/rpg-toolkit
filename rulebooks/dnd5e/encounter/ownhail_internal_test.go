// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// ownhail_internal_test.go pins A CREATURE HAILS FOR ITSELF, FROM ITS OWN
// MACHINE (rpg-project#1139, slice rpg-project#1140): a creature with no
// conversation whose current state has `on: approached` steps that all
// require a `within` hails whoever's step crosses in to that distance in its
// sight. Nothing stands afterwards: no address, no `addressed` beat. Internal,
// because the address records and the landed machine state are the
// composition's own.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	ohLine   = "Halt. Who goes there?"
	ohGuard  = MemberID("watch-1")
	ohGuard2 = MemberID("watch-2")
	ohBob    = MemberID("bob")
	ohGreet  = "greeted"
)

// ohStates is the creature's `approached` step: it requires within 2 and
// anything extra, picks a line, and moves to `to` when one is given.
func ohStates(extra []machine.Guard, to string) map[string]machine.State {
	return ohStatesWithin(2, extra, to)
}

// ohStatesWithin is [ohStates] at the distance given.
func ohStatesWithin(within int, extra []machine.Guard, to string) map[string]machine.State {
	step := machine.Transition{
		On:   beatApproached,
		When: &machine.Guard{All: append([]machine.Guard{{Scope: &machine.ScopeLeaf{Within: within}}}, extra...)},
		Do:   []machine.Act{{Pick: "approached"}},
		To:   to,
	}
	states := map[string]machine.State{"post": {Transitions: []machine.Transition{step}}}
	if to != "" {
		states[to] = machine.State{}
	}

	return states
}

// ohNotGreeted is `not noted:greeted`.
func ohNotGreeted() machine.Guard {
	return machine.Guard{Not: &machine.Guard{Fact: &machine.FactLeaf{Name: MachineNoted(ohGreet), Op: machine.OpEq, Value: true}}}
}

// ohGuardIn is a creature with no conversation, standing at the cell, running
// the states and saying the line when its `approached` pick runs.
func ohGuardIn(t *testing.T, id MemberID, at spatial.Position, states map[string]machine.State) MemberInput {
	t.Helper()
	g := listener(id, at.X, at.Y, recipe(t, "post", states))
	g.Name, g.Faction = "Watchman", postedGarrison
	table := holds()
	table["approached"] = []Answer{{Weight: 1, Say: ohLine}}
	g.Behaviors.Instances[0].Instance.Table = table

	return g
}

// ohSetup is the posted hall with alice at the cell given and the members.
func ohSetup(t *testing.T, sight Sight, alice spatial.Position, members ...MemberInput) *SetupInput {
	t.Helper()
	in := postedSetup(sight, members...)
	in.Members[0].Position = alice

	return in
}

// ohHall opens [ohSetup] with everyone seeing everything.
func ohHall(t *testing.T, alice spatial.Position, members ...MemberInput) *Encounter {
	t.Helper()
	enc, err := NewEncounter(ohSetup(t, everyoneSeesTheWholeMap{}, alice, members...))
	require.NoError(t, err)

	return enc
}

// ohHails is how many times a creature said its hail line, in alice's story.
func ohHails(t *testing.T, enc *Encounter, who MemberID) int {
	t.Helper()
	n := 0
	for _, b := range storyOf(t, enc) {
		if b["beat"] == BeatSpeech && b["speaker"] == string(who) && b["words"] == ohLine {
			n++
		}
	}

	return n
}

// ohStep is one member's step to a cell, at the hail reach.
func ohStep(t *testing.T, enc *Encounter, who MemberID, to spatial.Position) *StepOutput {
	t.Helper()
	out, err := enc.Step(&StepInput{Member: who, To: to, Reach: hailReach})
	require.NoError(t, err)
	require.Nil(t, out.Stopped, "precondition: nothing is in the way at %v", to)

	return out
}

// ohAt is a cell of the hall's first row-one line.
func ohAt(x float64) spatial.Position { return spatial.Position{X: x, Y: 1} }

func TestOwnHailHailsWhoCrossesIn(t *testing.T) {
	enc := ohHall(t, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")))
	require.Equal(t, 3.0, distanceTo(t, enc, "alice", ohGuard), "precondition: three cells away")

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Equal(t, 2.0, distanceTo(t, enc, "alice", ohGuard), "precondition: two cells away")
	require.Equal(t, []MemberID{ohGuard}, out.Hailed)
	require.Equal(t, 1, ohHails(t, enc, ohGuard), "he said his line")
	require.Empty(t, out.Addressed, "a creature's own hail is no conversation's address")
}

func TestOwnHailHailsAndMovesOn(t *testing.T) {
	enc := ohHall(t, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "warned")))

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Equal(t, []MemberID{ohGuard}, out.Hailed)
	require.Equal(t, "warned", stateOf(enc, ohGuard), "his step changed his state")
}

func TestOwnHailNotForStandingNear(t *testing.T) {
	enc := ohHall(t, ohAt(4), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")))
	require.Equal(t, 2.0, distanceTo(t, enc, "alice", ohGuard), "precondition: already within two")

	out := ohStep(t, enc, "alice", spatial.Position{X: 4, Y: 2})
	require.LessOrEqual(t, distanceTo(t, enc, "alice", ohGuard), 2.0, "precondition: still within two")
	require.Empty(t, out.Hailed, "standing near is not crossing in")
	require.Zero(t, ohHails(t, enc, ohGuard), "he says nothing")
}

func TestOwnHailAgainAfterLeaving(t *testing.T) {
	enc := ohHall(t, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")))
	require.Equal(t, []MemberID{ohGuard}, ohStep(t, enc, "alice", ohAt(4)).Hailed)

	require.Empty(t, ohStep(t, enc, "alice", ohAt(3)).Hailed, "stepping out beyond two hails nobody")
	require.Equal(t, []MemberID{ohGuard}, ohStep(t, enc, "alice", ohAt(4)).Hailed, "and back in is hailed again")
	require.Equal(t, 2, ohHails(t, enc, ohGuard))
}

func TestOwnHailEachPerson(t *testing.T) {
	enc := ohHall(t, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")),
		MemberInput{ID: ohBob, Kind: KindPlayer, Position: spatial.Position{X: 3, Y: 2}})
	require.Equal(t, []MemberID{ohGuard}, ohStep(t, enc, "alice", ohAt(4)).Hailed)

	out := ohStep(t, enc, ohBob, spatial.Position{X: 4, Y: 2})
	require.Equal(t, []MemberID{ohGuard}, out.Hailed, "he hails bob as well, nothing stands for alice")
	require.Equal(t, 2, ohHails(t, enc, ohGuard))
}

func TestOwnHailEachGuardForHimself(t *testing.T) {
	// Each writes his own distance: watch-2 two cells, watch-1 three. One step
	// brings alice to exactly the distance of both, so the nearer, watch-2,
	// goes first although watch-1 sorts before him by id.
	enc := ohHall(t, ohAt(3),
		ohGuardIn(t, ohGuard, ohAt(7), ohStatesWithin(3, nil, "")),
		ohGuardIn(t, ohGuard2, ohAt(6), ohStates(nil, "")))
	require.Greater(t, distanceTo(t, enc, "alice", ohGuard), 3.0, "precondition: beyond both")
	require.Greater(t, distanceTo(t, enc, "alice", ohGuard2), 2.0)

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Equal(t, 3.0, distanceTo(t, enc, "alice", ohGuard), "precondition: watch-1 is three off")
	require.Equal(t, 2.0, distanceTo(t, enc, "alice", ohGuard2), "precondition: watch-2 is two off")

	require.Equal(t, []MemberID{ohGuard2, ohGuard}, out.Hailed, "both hail, nearest first")
	require.Equal(t, 1, ohHails(t, enc, ohGuard))
	require.Equal(t, 1, ohHails(t, enc, ohGuard2))
}

func TestOwnHailTiesGoByID(t *testing.T) {
	enc := ohHall(t, ohAt(3),
		ohGuardIn(t, ohGuard2, spatial.Position{X: 6, Y: 2}, ohStates(nil, "")),
		ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")))

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Equal(t, distanceTo(t, enc, "alice", ohGuard), distanceTo(t, enc, "alice", ohGuard2), "precondition: the same distance")
	require.Equal(t, []MemberID{ohGuard, ohGuard2}, out.Hailed, "equally near: by id")
}

// ohBlind is everyone seeing the whole map but one member who sees nothing.
type ohBlind struct{ who MemberID }

// Sight answers zero for the blind one and the whole map for the rest.
func (s ohBlind) Sight(members []MemberID) (map[MemberID]int, error) {
	out, _ := everyoneSeesTheWholeMap{}.Sight(members)
	if _, ok := out[s.who]; ok {
		out[s.who] = 0
	}

	return out, nil
}

func TestOwnHailOnlyWhomHeSees(t *testing.T) {
	in := ohSetup(t, ohBlind{who: ohGuard}, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")))
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Equal(t, 2.0, distanceTo(t, enc, "alice", ohGuard), "precondition: crossed in")
	seen, err := enc.sees(ohGuard, "alice")
	require.NoError(t, err)
	require.False(t, seen, "precondition: he does not see her")
	require.Empty(t, out.Hailed)
	require.Zero(t, ohHails(t, enc, ohGuard))
}

func TestOwnHailHisWhenDecides(t *testing.T) {
	enc := ohHall(t, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates([]machine.Guard{ohNotGreeted()}, "")),
		MemberInput{ID: ohBob, Kind: KindPlayer, Position: spatial.Position{X: 3, Y: 2}})
	tbHoldAs(t, enc, ohGuard, ohGreet, "alice")

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Empty(t, out.Hailed, "he has noted greeted about alice: his step does not fire")
	require.Zero(t, ohHails(t, enc, ohGuard))

	out = ohStep(t, enc, ohBob, spatial.Position{X: 4, Y: 2})
	require.Equal(t, []MemberID{ohGuard}, out.Hailed, "bob is still hailed")
}

// tbHoldAs has a holder note a topic about the subjects, at once.
func tbHoldAs(t *testing.T, enc *Encounter, holder MemberID, topic string, subjects ...MemberID) {
	t.Helper()
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, holder, subjects, TopicID(topic), ChannelJudgement,
		recognitionLine{speaker: "alice", spokenTo: holder}))
}

func TestOwnHailNotWhenDown(t *testing.T) {
	in := ohSetup(t, everyoneSeesTheWholeMap{}, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")))
	in.Participation = &propagationStanding{down: []MemberID{ohGuard}}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Equal(t, 2.0, distanceTo(t, enc, "alice", ohGuard), "precondition: crossed in")
	require.Empty(t, out.Hailed, "a creature who is down hails nobody")
	require.Zero(t, ohHails(t, enc, ohGuard))
}

func TestOwnHailNotInAFight(t *testing.T) {
	raider := MemberInput{ID: "rex", Kind: KindMonster, Faction: saRaiders, Position: spatial.Position{X: 8, Y: 1}}
	in := ohSetup(t, everyoneSeesTheWholeMap{}, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")), raider)
	in.Field.Factions = append(in.Field.Factions, FactionInput{ID: saRaiders})
	in.Field.Dispositions = append(in.Field.Dispositions,
		DispositionInput{Between: [2]FactionID{postedGarrison, saRaiders}, Stance: StanceHostile})
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	_, _, err = enc.refreshSight(enc.rosterIDs())
	require.NoError(t, err)
	fighting, err := enc.inFight(ohGuard)
	require.NoError(t, err)
	require.True(t, fighting, "precondition: a fight holds him")
	// THE FIGHT TAKES THE WHOLE ROSTER (every member is in it at setup), so the
	// hailer's own fight and the mover's are not separable here.

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Equal(t, 2.0, distanceTo(t, enc, "alice", ohGuard), "precondition: crossed in")
	require.Empty(t, out.Hailed, "a creature in a fight hails nobody")
	require.Zero(t, ohHails(t, enc, ohGuard))
}

func TestOwnHailACarrierKeepsTheConversationsRule(t *testing.T) {
	g := hailingGuard(t, hailGuard, spatial.Position{X: 6, Y: 1})
	g.Behaviors.Instances[0].Instance.Recipe = *recipe(t, "posted", map[string]machine.State{
		"posted": {Transitions: []machine.Transition{
			{On: beatApproached, When: &machine.Guard{All: []machine.Guard{{Scope: &machine.ScopeLeaf{Within: 2}}}}, Do: []machine.Act{{Pick: "approached"}}},
		}},
	})
	in := hailSetup(t, everyoneSeesTheWholeMap{})
	in.Members[1] = g
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1})

	out := walk(t, enc, "alice", spatial.Position{X: 4, Y: 1})[0]
	require.Len(t, out.Addressed, 1, "the conversation's rule hails her exactly once")
	require.Empty(t, out.Hailed, "and the creature's own pass leaves a carrier alone")
	require.Len(t, beatsNamed(t, enc, BeatAddressed), 1)
}

func TestOwnHailWritesNoAddress(t *testing.T) {
	in := ohSetup(t, everyoneSeesTheWholeMap{}, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), ohStates(nil, "")))
	in.Field.Interactions = []InteractionInput{{ID: hailGate, Recipe: hailRecipe(), Approach: &InteractionApproach{Within: 2}}}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Equal(t, []MemberID{ohGuard}, out.Hailed, "precondition: he hailed")
	require.Empty(t, out.Addressed)
	require.Nil(t, enc.interactions[hailGate].address, "no address is written")
	require.Empty(t, beatsNamed(t, enc, BeatAddressed), "and no addressed beat is appended")
	require.Empty(t, beatsNamed(t, enc, BeatAddressEnded))
}

func TestOwnHailNoWithinIsNeverAsked(t *testing.T) {
	states := map[string]machine.State{"post": {Transitions: []machine.Transition{
		{On: beatApproached, Do: []machine.Act{{Pick: "approached"}}},
	}}}
	enc := ohHall(t, ohAt(3), ohGuardIn(t, ohGuard, ohAt(6), states))

	out := ohStep(t, enc, "alice", ohAt(4))
	require.Empty(t, out.Hailed, "a step with no within, and no conversation, is never asked")
	require.Zero(t, ohHails(t, enc, ohGuard))
}

func TestOwnHailTheWordsSayBothWays(t *testing.T) {
	approached, ok := BeatWordOf("approached")
	require.True(t, ok)
	require.Equal(t, "hails someone", approached.Label)
	require.Equal(t, "when someone comes within its distance in its sight "+
		"(or, for a conversation's carrier, under that conversation's hail rule)", approached.Short)
	require.Equal(t, "when someone's step brings them within {whose} distance (the step's own `within`) and {who} sees them; "+
		"or, when {who} carries a conversation that declares an approach, under that conversation's hail rule", approached.Long)
	require.Equal(t, "the one {who} hailed", approached.About)
}
