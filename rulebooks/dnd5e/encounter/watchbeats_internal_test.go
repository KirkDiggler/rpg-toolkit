// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// watchbeats_internal_test.go is what a run's record tells the studio's
// watch (rpg-project#1001): a sighting names its observer, and a second
// role's machine writes its own state beat, while a player's own story
// gains nothing.

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// watchStoryOf is one member's story as decoded beats, by sequence.
func watchStoryOf(t *testing.T, enc *Encounter, who MemberID) map[uint64]map[string]any {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: who})
	require.NoError(t, err)
	out := map[uint64]map[string]any{}
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		out[entry.Seq] = beat
	}
	return out
}

// recordOf is every beat any member was told, by sequence: the record as
// the watch reads it.
func recordOf(t *testing.T, enc *Encounter) []map[string]any {
	t.Helper()
	all := map[uint64]map[string]any{}
	for id := range enc.members {
		for seq, beat := range watchStoryOf(t, enc, id) {
			all[seq] = beat
		}
	}
	seqs := make([]uint64, 0, len(all))
	for seq := range all {
		seqs = append(seqs, seq)
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	out := make([]map[string]any, len(seqs))
	for i, seq := range seqs {
		out[i] = all[seq]
	}
	return out
}

// twoRoles is a guard whose body holds on `time` and whose second role,
// `investigate`, steps from `idle` to `looking` on `time`.
func twoRoles(t *testing.T) MemberInput {
	t.Helper()
	body := recipe(t, "watch", map[string]machine.State{"watch": {Transitions: []machine.Transition{pickTime}}})
	second := recipe(t, "idle", map[string]machine.State{
		"idle":    {Transitions: []machine.Transition{{On: beatTime, To: "looking", Reason: "a noise to look into"}}},
		"looking": {},
	})
	second.Ref.ID = "investigate"
	m := monster("m-guard", 8, 2, holds())
	m.Behaviors = &BehaviorInput{BodyInstance: "patrol", Instances: []BoundBehavior{
		{Instance: behavior.InstanceSpec{ID: "patrol", Recipe: *body, Table: holds()}},
		{Instance: behavior.InstanceSpec{ID: "investigate", Recipe: *second, Table: holds()}},
	}}
	return m
}

// TestASecondRoleWritesItsOwnStateBeat: the guard's `investigate` role
// steps; it writes a role_state beat naming the role and its recipe, told
// to the guard alone. Its body did not change, so no `state` beat is
// written, and a reader that folds only `state` beats (the watch before
// this) still reads the body as it was, never the role's state as the
// body's. Alice's story gains nothing.
func TestASecondRoleWritesItsOwnStateBeat(t *testing.T) {
	enc, _ := wakeHall(t, twoRoles(t))
	require.NoError(t, enc.spendWorldAction("alice"))

	var roles, states []map[string]any
	for _, beat := range recordOf(t, enc) {
		switch beat["beat"] {
		case BeatRoleState:
			roles = append(roles, beat)
		case BeatState:
			states = append(states, beat)
		}
	}
	require.Equal(t, []map[string]any{{
		"beat": BeatRoleState, "member": "m-guard", "role": "investigate", "recipe": "dnd5e:machines:investigate",
		"from": "idle", "to": "looking", "reason": "a noise to look into",
	}}, roles)
	require.Empty(t, states, "an old reader folding `state` beats sees no change to the guard's body, which is true")

	for _, beat := range watchStoryOf(t, enc, "alice") {
		require.NotEqual(t, BeatRoleState, beat["beat"], "a player is never told a creature's other role")
	}
	guard := watchStoryOf(t, enc, "m-guard")
	told := 0
	for _, beat := range guard {
		if beat["beat"] == BeatRoleState {
			told++
		}
	}
	require.Equal(t, 1, told, "the guard is told its own role's step")
}

// TestTheBodysStateBeatIsUnchanged: a body's step still writes exactly the
// `state` beat it wrote, to the same audience.
func TestTheBodysStateBeatIsUnchanged(t *testing.T) {
	guard := listener("m-guard", 8, 2, recipe(t, "watch", map[string]machine.State{
		"watch": {Transitions: []machine.Transition{{On: beatTime, To: "alert", Reason: "the hour turned", Do: []machine.Act{{Pick: string(AnswerTime)}}}}},
		"alert": {Transitions: []machine.Transition{pickTime}},
	}))
	enc, _ := wakeHall(t, guard)
	require.NoError(t, enc.spendWorldAction("alice"))
	var told []map[string]any
	for _, beat := range watchStoryOf(t, enc, "alice") {
		if beat["beat"] == BeatState {
			told = append(told, beat)
		}
	}
	require.Equal(t, []map[string]any{{"beat": BeatState, "member": "m-guard", "from": "watch", "to": "alert", "reason": "the hour turned"}}, told,
		"the body's state beat reaches the roster as before, with nothing added")
}

// TestASightingNamesItsObserver: every sighted beat names its observer, and
// a member's story holds only sightings whose observer is that member, so a
// player is told nothing new.
func TestASightingNamesItsObserver(t *testing.T) {
	in := wakeSetup(BehaviorDriver{}, monster("m-guard", 4, 2, holds()))
	in.Sight = everyoneSeesTheWholeMap{}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	seen := 0
	for _, who := range []MemberID{"alice", "m-guard"} {
		for _, beat := range watchStoryOf(t, enc, who) {
			if beat["beat"] != BeatSighted {
				continue
			}
			require.Equal(t, string(who), beat["observer"], "%s is told only its own sightings, which name it", who)
			seen++
		}
	}
	require.Equal(t, 2, seen, "each saw the other")
}

// TestARunSavedBeforeLoads: a run whose sighted beats name no observer (as
// every run saved before this did) loads, and its story reads as it was.
func TestARunSavedBeforeLoads(t *testing.T) {
	in := wakeSetup(BehaviorDriver{}, twoRoles(t))
	in.Sight = everyoneSeesTheWholeMap{}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)

	// Every story payload, as an older build wrote it: no observer.
	var blob map[string]any
	require.NoError(t, json.Unmarshal(raw, &blob))
	entries := blob["log"].(map[string]any)["entries"].([]any)
	stripped := 0
	for _, e := range entries {
		entry := e.(map[string]any)
		payload, err := base64.StdEncoding.DecodeString(entry["payload"].(string))
		require.NoError(t, err)
		var beat map[string]any
		require.NoError(t, json.Unmarshal(payload, &beat))
		if _, ok := beat["observer"]; ok {
			delete(beat, "observer")
			stripped++
		}
		back, err := json.Marshal(beat)
		require.NoError(t, err)
		entry["payload"] = base64.StdEncoding.EncodeToString(back)
	}
	require.Positive(t, stripped)
	old, err := json.Marshal(blob)
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(old, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: in.Capabilities})
	require.NoError(t, err)
	for _, beat := range watchStoryOf(t, loaded, "alice") {
		if beat["beat"] == BeatSighted {
			require.NotContains(t, beat, "observer", "an old sighting reads as it was written")
		}
	}
	require.NoError(t, loaded.spendWorldAction("alice"), "and the run goes on")
}
