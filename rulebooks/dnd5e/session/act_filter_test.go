// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// act_filter_test.go is E6's castle check (rpg-project#631, event-stream
// R35): the mad king's gatehouse played through the session seam, and each
// recipient receiving only the acts they perceive while the record keeps
// everything.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// actSubject is the member an act is about, "" for one about nobody.
func actSubject(act session.Act) string {
	switch b := act.Body.(type) {
	case session.WalkAct:
		return b.Mover.ID
	case session.AttackAct:
		return b.By.ID
	case session.SpeakAct:
		return b.By.ID
	case session.StayAct:
		return b.Member.ID
	case session.TableAnswer:
		return b.Who.ID
	case session.StateChange:
		return b.Party.ID
	case session.ClockEntry:
		if b.Arrival != nil {
			return b.Arrival.ID
		}
		return b.Member.ID
	default:
		return ""
	}
}

// recordHolds is the record's beats named beat whose payload names member
// under key — what the record keeps whoever its act stream reached (R35).
func recordHolds(t *testing.T, mgr *session.Manager, beat, key, member string) []session.RecordEvent {
	t.Helper()
	read, err := mgr.Record(context.Background(), &session.RecordReadInput{Session: "sess"})
	require.NoError(t, err)
	var out []session.RecordEvent
	for _, e := range read.Events {
		if e.Beat != beat {
			continue
		}
		var fields map[string]any
		require.NoError(t, json.Unmarshal(e.Fields, &fields))
		if fields[key] == member {
			out = append(out, e)
		}
	}
	return out
}

// keepPatrol is the gatehouse after alice's first line: the world takes its
// round, and keep-1 walks its keep patrol out of her sight.
func keepPatrol(t *testing.T) *gatehouse {
	t.Helper()
	g := openTheGatehouse(14, 5, 15)
	_, err := g.mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside",
	})
	require.NoError(t, err)
	return g
}

// movedBy is the record's moved beats by one member.
func movedBy(t *testing.T, g *gatehouse, member string) []session.RecordEvent {
	t.Helper()
	read, err := g.mgr.Record(context.Background(), &session.RecordReadInput{Session: "sess"})
	require.NoError(t, err)
	var out []session.RecordEvent
	for _, e := range read.Events {
		if e.Beat != encounter.BeatMoved {
			continue
		}
		var p struct {
			Member string `json:"member"`
		}
		require.NoError(t, json.Unmarshal(e.Fields, &p))
		if p.Member == member {
			out = append(out, e)
		}
	}
	return out
}

// TestFilterDropsAnUnseenGuardsWalk: keep-1 walks its keep patrol while
// alice stands at the gate. Alice receives no act about keep-1, live or on
// catch-up — and keep-1, folded for itself, receives its own walk.
//
// Mutant: a filter that delivers everything gives alice keep-1's walk.
func TestFilterDropsAnUnseenGuardsWalk(t *testing.T) {
	g := keepPatrol(t)
	require.NotEmpty(t, movedBy(t, g, "keep-1"), "keep-1 walked, or this proves nothing")

	caught := actsOf(t, g.mgr, "alice", "").Acts
	for _, acts := range [][]session.Act{g.live.of("alice"), caught} {
		require.NotEmpty(t, acts)
		for _, act := range acts {
			assert.NotEqual(t, "keep-1", actSubject(act), "alice cannot see the keep: %+v", act)
		}
	}

	var own []session.WalkAct
	for _, act := range actsOf(t, g.mgr, "keep-1", "").Acts {
		if w, ok := act.Body.(session.WalkAct); ok && w.Mover.ID == "keep-1" {
			own = append(own, w)
		}
	}
	assert.NotEmpty(t, own, "keep-1 receives its own walk")
}

// TestFilterRecordIsUnfiltered is R35's other half: the record keeps every
// beat — keep-1's steps that alice never receives among them — whoever
// reads it.
//
// Mutant: filtering the record by a viewer loses keep-1's steps.
func TestFilterRecordIsUnfiltered(t *testing.T) {
	g := keepPatrol(t)

	steps := movedBy(t, g, "keep-1")
	require.NotEmpty(t, steps, "the record holds keep-1's walk")

	acts := map[string]bool{}
	for _, act := range actsOf(t, g.mgr, "alice", "").Acts {
		acts[act.ID] = true
	}
	for _, step := range steps {
		assert.False(t, acts[step.ID], "keep-1's step %s is the record's, not alice's", step.ID)
	}
}

// TestFilterCatchUpEqualsLive is #541 R2 with the filter on: the gatehouse
// route — two lines, the walk in, the cut, the end of turn — for alice at
// the gate and keep-1 in the keep, two recipients in different rooms. What
// each received live is exactly what a catch-up folds, whole and after
// every act's id.
//
// Mutant: seeding the view from nothing on catch-up fails a suffix — the
// catch-up after a mid-route id forgets who alice saw at first light.
func TestFilterCatchUpEqualsLive(t *testing.T) {
	ctx := context.Background()
	g := openTheGatehouse(14, 5, 15)
	for range 2 {
		_, err := g.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
		require.NoError(t, err)
	}
	_, err := g.mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(t, g.mgr, "sess", "alice"),
	})
	require.NoError(t, err)

	for _, member := range []string{"alice", "keep-1"} {
		published := g.live.of(member)
		require.NotEmpty(t, published, member)

		caught := actsOf(t, g.mgr, member, "")
		require.False(t, caught.Restarted, "the route fits the retention window, or this proves nothing")
		assert.Equal(t, published, caught.Acts, "%s: live and catch-up fold the same acts", member)

		for i, act := range published {
			last := i
			for j := i; j < len(published); j++ {
				if published[j].ID == act.ID {
					last = j
				}
			}
			want := published[last+1:]
			got := actsOf(t, g.mgr, member, act.ID).Acts
			if len(want) == 0 {
				assert.Empty(t, got, "%s after %s", member, act.ID)
				continue
			}
			assert.Equal(t, want, got, "%s after %s", member, act.ID)
		}
	}
}
