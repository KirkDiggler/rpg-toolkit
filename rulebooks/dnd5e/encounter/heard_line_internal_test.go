// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// heard_line_internal_test.go is the internal half of a line heard moves an
// NPC's own machine (rpg-project#1000): who a beat names, what a landed
// sound and an NPC's own speech carry, a fighting NPC, and the refusals.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// A beat names the one it is about only where a sense names them: seen,
// walked into, hailed, or a line's speaker caught and seen. A landed sound
// — a noise, an NPC's own speech — names nobody and carries no claim.
func TestABeatNamesOnlyWhatASenseNames(t *testing.T) {
	enc, _ := earsHall(t, listener("guard", 4, 1, postedRecipe(t)))
	m := enc.members["guard"]
	cell := spatial.Position{X: 2, Y: 1}
	for _, tc := range []struct {
		name  string
		beat  wakeBeat
		names bool
	}{
		{"seen", wakeBeat{Beat: beatSeen, Cell: &cell, Subject: "alice", Member: "guard"}, true},
		{"barred", wakeBeat{Beat: beatBarred, Cell: &cell, Subject: "alice", Member: "guard"}, true},
		{"approached", wakeBeat{Beat: beatApproached, Cell: &cell, Subject: "alice", Member: "guard"}, true},
		{"a line caught and seen", wakeBeat{Beat: beatHeard, Kind: SoundVoice, Cell: &cell, Subject: "alice", Member: "guard", Named: true}, true},
		{"a landed voice", wakeBeat{Beat: beatHeard, Kind: SoundVoice, Cell: &cell, Subject: "alice", Member: "guard"}, false},
		{"a deed", wakeBeat{Beat: beatDeed, Cell: &cell, Subject: "alice"}, false},
		{"a fact", wakeBeat{Beat: beatFact, Member: "guard"}, false},
		{"a topic that names its subject", wakeBeat{Beat: beatTopic, Subject: "alice", Member: "guard"}, true},
		{"a topic with no subject", wakeBeat{Beat: beatTopic, Member: "guard"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := enc.evFor("guard", m, tc.beat)
			require.NoError(t, err)
			if tc.names {
				require.Equal(t, MemberID("alice"), ev.Subject)
				return
			}
			require.Empty(t, ev.Subject)
		})
	}
}

// TestAnNPCsSpeechAndANoiseLandUnnamedWithNoClaim: what an NPC says, and a
// clang, land for the next raise as before; neither names its maker, and
// `heard.claim` is unknown on both.
func TestAnNPCsSpeechAndANoiseLandUnnamedWithNoClaim(t *testing.T) {
	enc, _ := earsHall(t, listener("guard", 4, 1, postedRecipe(t)))
	require.NoError(t, enc.makeSound(noiseAttack, "alice", 0))
	require.NoError(t, enc.makeSound(noiseSay, "alice", 0))
	require.Len(t, enc.landed, 2, "both land for the next raise")
	claim := machineVocabulary().Facts[factHeardClaim]
	for _, b := range enc.landed {
		ev, err := enc.evFor("guard", enc.members["guard"], b)
		require.NoError(t, err)
		require.Empty(t, ev.Subject, "%s names nobody", b.Kind)
		_, known := claim.Read(&machineEnv{e: enc, id: "guard", event: ev})
		require.False(t, known, "%s carries no claim", b.Kind)
	}
}

// TestANPCInAFightStillHearsALine pins the plan's section 4: a member in a
// fight still steps on a line it hears — the state-change path asks only
// the seat clock — and is not armed on the world clock by it.
func TestANPCInAFightStillHearsALine(t *testing.T) {
	heard := recipe(t, "quiet", map[string]machine.State{
		"quiet": {Transitions: []machine.Transition{{On: beatHeard, To: "heard", Reason: "heard"}}},
		"heard": {},
	})
	target := monster("goblin", 2, 2, holds())
	target.Options = []Option{{ID: "hello", Say: "Evening.", Success: []Answer{{Weight: 1, Say: "Evening."}}}}
	in := wakeSetup(BehaviorDriver{}, listener("brute", 3, 1, heard), target)
	in.Sheets, in.Sight = earsSheets{}, everyoneSeesTheWholeMap{}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	// Everyone sees everyone, and nobody is anybody's ally: the fight is on.
	on, err := enc.onWorldClock("brute")
	require.NoError(t, err)
	require.False(t, on, "precondition: the brute is in the fight")

	_, err = enc.Speak(context.Background(), &SpeakInput{Range: 3, Actor: "alice", Target: "goblin", OptionID: "hello", Roller: wakeRollsOne{}})
	require.NoError(t, err)
	require.Equal(t, "heard", stateOf(enc, "brute"), "the fighting NPC's machine steps")
	_, armed := enc.dueAt["brute"]
	require.False(t, armed, "and it is not armed on the world clock: it is in the fight")
}

// TestTheGuardWordsAreRefused: every refusal sentence, from the engine's one
// judge, at install and on a standalone recipe alike.
func TestTheGuardWordsAreRefused(t *testing.T) {
	leaf := func(name string, value any) *machine.Guard {
		return &machine.Guard{Fact: &machine.FactLeaf{Name: machine.FactName(name), Op: machine.OpEq, Value: value}}
	}
	for _, tc := range []struct {
		name     string
		on       machine.Beat
		when     *machine.Guard
		sentence string
	}{
		{"a claim that is neither", beatHeard, leaf("heard.claim", "maybe"),
			"`heard.claim` is held or failed, and this names \"maybe\""},
		{"a claim not heard", beatTime, leaf("heard.claim", "failed"),
			"`heard.claim` reads what the creature heard, and `time` is not hearing: it is read on `heard` only"},
		{"a claim nested under a not", beatSeen, &machine.Guard{Not: leaf("heard.claim", "held")},
			"`heard.claim` reads what the creature heard, and `seen` is not hearing: it is read on `heard` only"},
		{"holds on a fact", beatFact, leaf("noted:admitted", true),
			"`noted:admitted` asks about the one this beat is about, and `fact` is about nobody: it is read on heard, seen, topic, barred and approached"},
		{"holds on time", beatTime, leaf("noted:admitted", true),
			"`noted:admitted` asks about the one this beat is about, and `time` is about nobody: it is read on heard, seen, topic, barred and approached"},
		{"holds on a deed", beatDeed, &machine.Guard{All: []machine.Guard{*leaf("noted:admitted", true)}},
			"`noted:admitted` asks about the one this beat is about, and `deed` is about nobody: it is read on heard, seen, topic, barred and approached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.sentence, GuardWordsRefusal(tc.on, tc.when))
			r := recipe(t, "quiet", map[string]machine.State{"quiet": {Transitions: []machine.Transition{{On: tc.on, When: tc.when, To: "quiet"}}}})
			err := ValidateMachine(r)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.sentence)
			_, err = CheckBehaviors(&CheckBehaviorsInput{Behaviors: *fixtureBehavior(*r, nil)})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.sentence)
		})
	}
	for _, on := range []machine.Beat{beatHeard, beatSeen, beatBarred, beatApproached} {
		require.Empty(t, GuardWordsRefusal(on, leaf("noted:admitted", true)), "holds is read on %s", on)
	}
	require.Empty(t, GuardWordsRefusal(beatHeard, leaf("heard.claim", "held")))
}
