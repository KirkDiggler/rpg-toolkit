// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// address_test.go pins THE GUARD HAILS (rpg-project#923, Gate S3 of #769):
// a carrier of an interaction with an approach who sees a character come
// inside it addresses them, once per interaction, and the address ends —
// progress untouched — when the predicate no longer holds. Internal, because
// the address and the machine blob are the composition's own.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	hailGate    = InteractionID("front-gate")
	hailLine    = "Hold up there, friend. What brings you to the gate?"
	hailReach   = 3
	hailGuard   = MemberID("gate-1")
	hailGuard2  = MemberID("gate-2")
	hailVisitor = MemberID("billy")
)

// hailRecipe is the front gate: fresh until a claim admits; a warning moves
// it to warned (anchoring it), which still offers the claim.
func hailRecipe() machine.Recipe {
	return machine.Recipe{
		Ref:     core.Ref{Module: "test", Type: "interactions", ID: "front-gate"},
		Initial: "fresh",
		States: map[string]machine.State{
			"fresh": {Transitions: []machine.Transition{
				{On: "admitted", To: "admitted", Reason: "let through"},
				{On: "warned", To: "warned", Do: []machine.Act{{Anchor: true}}, Reason: "warned once"},
			}},
			"warned":   {Transitions: []machine.Transition{{On: "admitted", To: "admitted", Reason: "let through"}}},
			"admitted": {Terminal: true},
		},
	}
}

// warnTheGate advances the gate to warned, out of its initial state.
func warnTheGate(t *testing.T, enc *Encounter) {
	t.Helper()
	require.NoError(t, enc.advanceInteraction(Option{ID: "warn", Advance: &Advance{Interaction: hailGate, Success: "warned"}}, true, recognitionLine{}))
	require.Equal(t, "warned", enc.interactions[hailGate].blob.State, "precondition: the gate moved on")
}

// hailClaim is the one claim at the gate: offered while it is fresh, and
// admitting on its verdict.
func hailClaim() []Option {
	return []Option{{
		ID: "guest", Say: "We're invited.",
		Requires: []Trigger{TriggerInteraction{Interaction: hailGate, In: []string{"fresh", "warned"}}},
		Advance:  &Advance{Interaction: hailGate, Success: "admitted"},
		Success:  []Answer{{Weight: 1, Say: "In you go."}},
	}}
}

// hailingGuard is a gate post carrying the claim: a bump gets his bump
// line, an approach his hail.
func hailingGuard(t *testing.T, id MemberID, at spatial.Position) MemberInput {
	t.Helper()
	g := listener(id, at.X, at.Y, recipe(t, "posted", map[string]machine.State{
		"posted": {Transitions: []machine.Transition{
			{On: beatBarred, Do: []machine.Act{{Pick: "barred"}}},
			{On: beatApproached, Do: []machine.Act{{Pick: "approached"}}},
			pickTime,
		}},
	}))
	g.Name, g.Faction = "Gate guard", postedGarrison
	g.BlockingStates = []string{"posted"}
	g.Options = hailClaim()
	table := holds()
	table["barred"] = []Answer{{Weight: 1, Say: bumpLine}}
	table["approached"] = []Answer{{Weight: 1, Say: hailLine}}
	g.Behaviors.Instances[0].Instance.Table = table

	return g
}

// hailSetup is the posted hall with the front gate (approach 2), one guard
// at (6,1) and whatever else is given.
func hailSetup(t *testing.T, sight Sight, members ...MemberInput) *SetupInput {
	t.Helper()
	in := postedSetup(sight, append([]MemberInput{hailingGuard(t, hailGuard, spatial.Position{X: 6, Y: 1})}, members...)...)
	in.Field.Interactions = []InteractionInput{{ID: hailGate, Recipe: hailRecipe(), Approach: &InteractionApproach{Within: 2}}}

	return in
}

// hailHall opens [hailSetup] with everyone seeing everything.
func hailHall(t *testing.T, members ...MemberInput) *Encounter {
	t.Helper()
	enc, err := NewEncounter(hailSetup(t, everyoneSeesTheWholeMap{}, members...))
	require.NoError(t, err)

	return enc
}

// walk steps a member cell by cell, measured at the hail reach, and
// returns each step's output.
func walk(t *testing.T, enc *Encounter, who MemberID, cells ...spatial.Position) []*StepOutput {
	t.Helper()
	out := make([]*StepOutput, 0, len(cells))
	for _, cell := range cells {
		stepped, err := enc.Step(&StepInput{Member: who, To: cell, Reach: hailReach})
		require.NoError(t, err)
		require.Nil(t, stepped.Stopped, "precondition: nothing is in the way at %v", cell)
		out = append(out, stepped)
	}

	return out
}

// addressOf is the standing address of a member at the hail reach.
func addressOf(t *testing.T, enc *Encounter, who MemberID) *Addressed {
	t.Helper()
	got, err := enc.AddressFor(&AddressForInput{Member: who, Reach: hailReach})
	require.NoError(t, err)

	return got.Address
}

// beatsNamed is every beat of one kind alice is told, in order.
func beatsNamed(t *testing.T, enc *Encounter, beat string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, b := range storyOf(t, enc) {
		if b["beat"] == beat {
			out = append(out, b)
		}
	}

	return out
}

// distanceTo is how far a member stands from the guard.
func distanceTo(t *testing.T, enc *Encounter, who, guard MemberID) float64 {
	t.Helper()
	a, ok := enc.canvas.GetEntityPosition(string(who))
	require.True(t, ok)
	b, ok := enc.canvas.GetEntityPosition(string(guard))
	require.True(t, ok)

	return enc.Distance(a, b)
}

func TestAnApproachInsideTheDistanceIsHailed(t *testing.T) {
	enc := hailHall(t)
	blob := enc.interactions[hailGate].blob.State

	steps := walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1})
	for _, s := range steps {
		require.Empty(t, s.Addressed, "beyond two cells nobody hails")
	}
	require.Nil(t, addressOf(t, enc, "alice"))

	hailed := walk(t, enc, "alice", spatial.Position{X: 4, Y: 1})[0]
	require.Equal(t, 2.0, distanceTo(t, enc, "alice", hailGuard), "precondition: two cells off")
	require.Len(t, hailed.Addressed, 1, "the step reports the address it wrote")
	require.Equal(t, hailGate, hailed.Addressed[0].Interaction)
	require.Equal(t, hailGuard, hailed.Addressed[0].Guard)
	require.Equal(t, spatial.Position{X: 4, Y: 1}, hailed.Stepped.To, "the step itself was taken")

	require.Equal(t, &Addressed{Interaction: hailGate, Guard: hailGuard}, addressOf(t, enc, "alice"))
	require.Equal(t, &InteractionAddress{Member: "alice", Guard: hailGuard}, enc.interactions[hailGate].address)

	addressed := beatsNamed(t, enc, BeatAddressed)
	require.Len(t, addressed, 1)
	require.Equal(t, string(hailGate), addressed[0]["interaction"])
	require.Equal(t, "alice", addressed[0]["member"])
	require.Equal(t, string(hailGuard), addressed[0]["guard"])
	require.Equal(t, float64(hailed.Addressed[0].Seq), addressed[0]["seq"], "the output names the beat")

	var hail map[string]any
	for _, b := range storyOf(t, enc) {
		if b["beat"] == BeatAnswered && b["creature"] == string(hailGuard) && b["say"] == hailLine {
			hail = b
		}
	}
	require.NotNil(t, hail, "the approached wake picks his line")
	require.Less(t, addressed[0]["seq"].(float64), hail["seq"].(float64), "the address, then his hail")
	require.Equal(t, blob, enc.interactions[hailGate].blob.State, "an address is not progress")
}

func TestAnAddressSurvivesSaveAndLoad(t *testing.T) {
	enc := hailHall(t)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})
	capabilities := hailSetup(t, everyoneSeesTheWholeMap{}).Capabilities

	data := enc.ToData()
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: capabilities})
	require.NoError(t, err)
	require.Equal(t, &Addressed{Interaction: hailGate, Guard: hailGuard}, addressOf(t, loaded, "alice"))

	for name, edit := range map[string]func(*InteractionAddress){
		"an unknown member":        func(a *InteractionAddress) { a.Member = "nobody" },
		"an unknown guard":         func(a *InteractionAddress) { a.Guard = "nobody" },
		"a guard who carries none": func(a *InteractionAddress) { a.Guard = "alice"; a.Member = hailGuard },
		"a member addressing self": func(a *InteractionAddress) { a.Member = hailGuard },
	} {
		t.Run(name, func(t *testing.T) {
			bad := enc.ToData()
			address := *bad.Interactions[0].Address
			edit(&address)
			bad.Interactions[0].Address = &address
			_, err := LoadEncounter(&LoadEncounterInput{Data: bad, Capabilities: capabilities})
			require.ErrorIs(t, err, ErrInvalidData)
		})
	}
}

func TestSteppingOutOfReachEndsTheAddressAndKeepsTheProgress(t *testing.T) {
	enc := hailHall(t)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})
	warnTheGate(t, enc)
	before := enc.ToData().Interactions[0].Blob
	require.NotEqual(t, hailRecipe().Initial, before.State, "precondition: progress a reset would lose")
	require.NotNil(t, before.Anchor, "precondition: and an anchor a reset would lose")

	walk(t, enc, "alice", spatial.Position{X: 3, Y: 1})
	require.Equal(t, 3.0, distanceTo(t, enc, "alice", hailGuard))
	require.NotNil(t, addressOf(t, enc, "alice"), "inside Speak reach the address stands")
	require.Empty(t, beatsNamed(t, enc, BeatAddressEnded))

	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1})
	require.Nil(t, addressOf(t, enc, "alice"), "beyond reach it is gone")
	require.Nil(t, enc.interactions[hailGate].address, "the sweep cleared it")
	ended := beatsNamed(t, enc, BeatAddressEnded)
	require.Len(t, ended, 1)
	require.Equal(t, string(AddressOutOfReach), ended[0]["reason"])
	require.Equal(t, "alice", ended[0]["member"])
	require.Equal(t, before, enc.ToData().Interactions[0].Blob, "ending touches no progress")
}

func TestLeavingAndReturningIsHailedAgain(t *testing.T) {
	enc := hailHall(t)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})
	walk(t, enc, "alice", spatial.Position{X: 3, Y: 1}, spatial.Position{X: 2, Y: 1})
	require.Nil(t, addressOf(t, enc, "alice"))

	back := walk(t, enc, "alice", spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})
	require.Empty(t, back[0].Addressed)
	require.Len(t, back[1].Addressed, 1, "coming back inside the distance is hailed again")
	require.Len(t, beatsNamed(t, enc, BeatAddressed), 2)
}

func TestNoSecondAddressWhileOneStands(t *testing.T) {
	enc := hailHall(t,
		hailingGuard(t, hailGuard2, spatial.Position{X: 6, Y: 3}),
		MemberInput{ID: hailVisitor, Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 3}},
	)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})
	require.NotNil(t, addressOf(t, enc, "alice"))

	steps := walk(t, enc, hailVisitor, spatial.Position{X: 2, Y: 3}, spatial.Position{X: 3, Y: 3}, spatial.Position{X: 4, Y: 3})
	require.LessOrEqual(t, distanceTo(t, enc, hailVisitor, hailGuard2), 2.0, "precondition: inside gate-2's distance")
	for _, s := range steps {
		require.Empty(t, s.Addressed, "one addressee per interaction")
	}
	require.Nil(t, addressOf(t, enc, hailVisitor))
	require.Equal(t, &Addressed{Interaction: hailGate, Guard: hailGuard}, addressOf(t, enc, "alice"), "the first still stands")
	require.Len(t, beatsNamed(t, enc, BeatAddressed), 1)
}

func TestAnUnseenApproachIsNotHailed(t *testing.T) {
	enc, err := NewEncounter(hailSetup(t, nobodySees{}))
	require.NoError(t, err)

	steps := walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1}, spatial.Position{X: 5, Y: 1})
	for _, s := range steps {
		require.Empty(t, s.Addressed, "a guard who does not see her hails nobody")
	}
	require.Nil(t, enc.interactions[hailGate].address)
	require.Empty(t, beatsNamed(t, enc, BeatAddressed))
}

func TestABumpWhileAddressedIsTheBumpLine(t *testing.T) {
	enc := hailHall(t)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1}, spatial.Position{X: 5, Y: 1})
	require.Len(t, beatsNamed(t, enc, BeatAddressed), 1, "precondition: hailed once on the way")

	out := bumped(t, enc, spatial.Position{X: 6, Y: 1}, hailGuard)
	require.Empty(t, out.Addressed, "a bump writes no address")
	answers := answersBy(t, enc, hailGuard)
	require.Equal(t, bumpLine, answers[len(answers)-1]["say"], "his bump line, as today")
	require.Len(t, beatsNamed(t, enc, BeatAddressed), 1, "still one address")
	require.Equal(t, &Addressed{Interaction: hailGate, Guard: hailGuard}, addressOf(t, enc, "alice"))
}

func TestNoReplyEndsTheAddress(t *testing.T) {
	enc := hailHall(t)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})
	require.NoError(t, enc.advanceInteraction(hailClaim()[0], true, recognitionLine{}))
	require.Equal(t, "admitted", enc.interactions[hailGate].blob.State, "precondition: the gate admitted")

	require.Nil(t, addressOf(t, enc, "alice"), "the read asks the predicate: nothing is offered")
	walk(t, enc, "alice", spatial.Position{X: 5, Y: 1})
	ended := beatsNamed(t, enc, BeatAddressEnded)
	require.Len(t, ended, 1)
	require.Equal(t, string(AddressNoReply), ended[0]["reason"])
}

func TestApproachReachIsTheHostsBound(t *testing.T) {
	in := []InteractionInput{{ID: hailGate, Recipe: hailRecipe(), Approach: &InteractionApproach{Within: 3}}}
	require.NoError(t, ValidateApproachReach(in, 3))
	err := ValidateApproachReach(in, 2)
	require.ErrorIs(t, err, ErrNoField)
	require.ErrorContains(t, err, "front-gate")

	for _, within := range []int{0, -1} {
		bad := hailSetup(t, everyoneSeesTheWholeMap{})
		bad.Field.Interactions[0].Approach.Within = within
		_, err := NewEncounter(bad)
		require.Error(t, err, "within %d is refused at the door", within)
		require.True(t, errors.Is(err, ErrNoField))
	}
}

// answeredSaying is the answered beat of one creature saying line, decoded,
// nil when there is none.
func answeredSaying(t *testing.T, enc *Encounter, creature MemberID, line string) map[string]any {
	t.Helper()
	for _, b := range storyOf(t, enc) {
		if b["beat"] == BeatAnswered && b["creature"] == string(creature) && b["say"] == line {
			return b
		}
	}
	return nil
}

func TestTheHailIsSpokenToTheMover(t *testing.T) {
	enc := hailHall(t)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})

	hail := answeredSaying(t, enc, hailGuard, hailLine)
	require.NotNil(t, hail)
	require.Equal(t, "alice", hail["to"], "his hail is said to the member he addressed")
	require.Equal(t, "", hail["target"], "the intent's target is not who he speaks to")
}

func TestTheBumpLineIsSpokenToTheMover(t *testing.T) {
	enc := hailHall(t)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1}, spatial.Position{X: 5, Y: 1})
	bumped(t, enc, spatial.Position{X: 6, Y: 1}, hailGuard)

	line := answeredSaying(t, enc, hailGuard, bumpLine)
	require.NotNil(t, line)
	require.Equal(t, "alice", line["to"], "his bump line is said to the member who bumped")
}

func TestATimeLineIsSpokenToNobody(t *testing.T) {
	const muttered = "Cold night for it."
	crier := listener("crier", 8, 3, recipe(t, "ready", map[string]machine.State{"ready": {Transitions: []machine.Transition{pickTime}}}))
	crier.Faction = postedGarrison
	crier.Behaviors.Instances[0].Instance.Table = Table{AnswerTime: {{Weight: 1, Say: muttered}}}
	enc := hailHall(t, crier)

	require.NoError(t, enc.spendWorldAction("alice"))
	line := answeredSaying(t, enc, "crier", muttered)
	require.NotNil(t, line, "precondition: his time pick said something")
	require.Equal(t, "", line["to"], "a line nobody prompted is said to nobody")
}

func TestASilentHailIsSpokenToNobody(t *testing.T) {
	enc, err := NewEncounter(func() *SetupInput {
		in := hailSetup(t, everyoneSeesTheWholeMap{})
		for i := range in.Members {
			if in.Members[i].ID == hailGuard {
				in.Members[i].Behaviors.Instances[0].Instance.Table["approached"] = []Answer{{Weight: 1, Hold: true}}
			}
		}
		return in
	}())
	require.NoError(t, err)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})

	var picked map[string]any
	for _, b := range storyOf(t, enc) {
		if b["beat"] == BeatAnswered && b["creature"] == string(hailGuard) && b["key"] == "behavior/6:policy/approached" {
			picked = b
		}
	}
	require.NotNil(t, picked, "precondition: his approached row was picked")
	require.Equal(t, "", picked["say"])
	require.Equal(t, "", picked["to"], "a row that says nothing is said to nobody")
}

// partialSight is everybody seeing far, but alice seeing only as far as her
// own sight says — one cell when dim, far otherwise. Switchable mid-scene.
type partialSight struct{ dim *bool }

func (s partialSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 1_000_000
		if id == "alice" && *s.dim {
			out[id] = 1
		}
	}
	return out, nil
}

// The review probe, turned round: a guard who sees a member who cannot see
// him does not hail her, and her walk goes on.
func TestAGuardSheCannotSeeDoesNotHail(t *testing.T) {
	dim := true
	enc, err := NewEncounter(hailSetup(t, partialSight{dim: &dim}))
	require.NoError(t, err)

	steps := walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})
	require.Equal(t, 2.0, distanceTo(t, enc, "alice", hailGuard), "precondition: inside the approach")
	seen, err := enc.sees(hailGuard, "alice")
	require.NoError(t, err)
	require.True(t, seen, "precondition: he sees her")
	for _, s := range steps {
		require.Empty(t, s.Addressed, "no hail: she could not answer a guard she cannot see, so her walk is never stopped")
	}
	require.Nil(t, enc.interactions[hailGate].address)
	require.Empty(t, beatsNamed(t, enc, BeatAddressed))
}

// A standing address ends when the member loses sight of the guard, and the
// gate's progress stays where it was.
func TestLosingSightOfTheGuardEndsTheAddress(t *testing.T) {
	dim := false
	enc, err := NewEncounter(hailSetup(t, partialSight{dim: &dim}))
	require.NoError(t, err)
	walk(t, enc, "alice", spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1})
	require.NotNil(t, addressOf(t, enc, "alice"), "precondition: hailed")
	warnTheGate(t, enc)
	before := enc.ToData().Interactions[0].Blob

	dim = true
	walk(t, enc, "alice", spatial.Position{X: 4, Y: 2})
	require.LessOrEqual(t, distanceTo(t, enc, "alice", hailGuard), float64(hailReach), "precondition: still in reach")
	require.Nil(t, addressOf(t, enc, "alice"), "she cannot see him: the address is gone")
	ended := beatsNamed(t, enc, BeatAddressEnded)
	require.Len(t, ended, 1)
	require.Equal(t, string(AddressOutOfSight), ended[0]["reason"])
	require.Equal(t, before, enc.ToData().Interactions[0].Blob, "ending touches no progress")
}
