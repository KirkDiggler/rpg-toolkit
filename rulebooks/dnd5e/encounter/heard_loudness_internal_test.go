// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// TestARaisedVoiceIsAGuardFact: a transition's `heard.loudness:<word>` fact
// (rpg-project#933) is registered only for a loudness this rulebook names,
// and reads true while the heard occurrence was at or above it — a clang,
// which states no loudness, is at or above nothing.
func TestARaisedVoiceIsAGuardFact(t *testing.T) {
	recipe := &machine.Recipe{States: map[string]machine.State{"posted": {Transitions: []machine.Transition{
		{On: beatHeard, When: &machine.Guard{Any: []machine.Guard{
			{Fact: &machine.FactLeaf{Name: "heard.loudness:shout", Op: machine.OpEq, Value: true}},
			{Fact: &machine.FactLeaf{Name: "heard.loudness:yell", Op: machine.OpEq, Value: true}},
		}}},
	}}}}
	v := vocabularyFor(recipe)
	spec, ok := v.Facts["heard.loudness:shout"]
	require.True(t, ok, "a loudness this rulebook names is a fact")
	_, ok = v.Facts["heard.loudness:yell"]
	require.False(t, ok, "a misspelt one is not, so the vocabulary refuses it")

	for _, tc := range []struct {
		heard Loudness
		want  bool
	}{{LoudnessScream, true}, {LoudnessShout, true}, {LoudnessSpeak, false}, {LoudnessWhisper, false}, {"", false}} {
		got, ok := spec.Read(&machineEnv{event: wakeEv{Kind: SoundVoice, Loudness: tc.heard}})
		require.True(t, ok)
		require.Equal(t, tc.want, got, "heard at %q", tc.heard)
	}
}
