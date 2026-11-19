// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// notes_test.go is "trace one note" (rpg-project#1001): from connections
// alone, what gives a topic, who comes to note it and how, about whom, and
// what reads it.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// notePointer reports whether a statement repeats another's under a hub or
// as a way past a holder, rather than giving or reading a note itself.
func notePointer(st readback.Statement) bool {
	k := string(st.Kind)
	return k == "topic" || strings.HasPrefix(k, "topic.") || strings.HasPrefix(k, "fact.") || strings.HasPrefix(k, "place.way")
}

// edgesAt is st's connections of kind and role at topic.
func edgesAt(st readback.Statement, kind, role, topic string) []readback.Connection {
	var out []readback.Connection
	for _, c := range st.Connects {
		if c.Kind == kind && c.Role == role && c.At == topic {
			out = append(out, c)
		}
	}
	return out
}

// TestEveryNoteGivenOrReadSaysWho: over every content file, every statement
// that gives a topic says whom it is about, which taker words take it (with
// their way) and which creatures can come to note it; every statement that
// reads one says whom it asks about.
func TestEveryNoteGivenOrReadSaysWho(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.yaml")
	require.NoError(t, err)
	more, err := filepath.Glob("testdata/*.yaml")
	require.NoError(t, err)
	gives, reads := 0, 0
	for _, f := range append(files, more...) {
		var rb *readback.ReadBack
		if filepath.Base(f) == filepath.Base(castlePath) {
			_, rb = theCastle(t)
		} else {
			raw, err := os.ReadFile(f)
			require.NoError(t, err)
			if rb, err = readback.Of(raw); err != nil {
				continue
			}
		}
		for _, st := range rb.Statements() {
			for _, c := range st.Connects {
				if c.Kind != "topic" {
					continue
				}
				where := fmt.Sprintf("%s %s %s", filepath.Base(f), st.Kind, st.Path)
				switch c.Role {
				case "grants":
					gives++
					require.Len(t, edgesAt(st, readback.ConnWho, "about", c.ID), 1, "%s: whom %s is about", where, c.ID)
					require.NotEmpty(t, edgesAt(st, readback.ConnWho, "takes", c.ID), "%s: who takes %s", where, c.ID)
					for _, n := range edgesAt(st, "creature", readback.RoleNotes, c.ID) {
						require.NotEmpty(t, n.Is, "%s: the way %s comes by %s", where, n.ID, c.ID)
					}
				case "asks", "wakes-on":
					reads++
					require.Len(t, edgesAt(st, readback.ConnWho, "about", c.ID), 1, "%s: whom the read of %s asks about", where, c.ID)
				}
			}
		}
	}
	require.Positive(t, gives)
	require.Positive(t, reads)
}

// TestTheCastleTracesAdmitted builds the trace of `admitted` from the
// castle's connections alone: each give (what gave it, about whom, who
// notes it and how) and each read (about whom, whose note, negated).
func TestTheCastleTracesAdmitted(t *testing.T) {
	_, rb := theCastle(t)
	var gives, reads []string
	for _, st := range rb.Statements() {
		if notePointer(st) {
			continue
		}
		for _, c := range st.Connects {
			if c.Kind != "topic" || c.ID != "admitted" {
				continue
			}
			about := ""
			for _, w := range edgesAt(st, readback.ConnWho, "about", "admitted") {
				about = w.ID
			}
			switch c.Role {
			case "grants":
				var takers, cause []string
				for _, n := range edgesAt(st, "creature", readback.RoleNotes, "admitted") {
					takers = append(takers, n.ID+" by "+n.Is)
				}
				sort.Strings(takers)
				for _, x := range st.Connects {
					if x.Role == "on" || x.Role == "enters" {
						cause = append(cause, x.Kind+" "+x.ID+" "+x.Is)
					}
				}
				gives = append(gives, fmt.Sprintf("%s | %s | about %s | %s", st.Path, strings.Join(cause, ", "), about, strings.Join(takers, ", ")))
			case "asks", "wakes-on":
				whose := "its host"
				for _, w := range edgesAt(st, "creature", readback.RoleWhose, "admitted") {
					whose = w.ID
				}
				not := ""
				if c.Not {
					not = " NOT"
				}
				reads = append(reads, fmt.Sprintf("%s | about %s | %s%s", st.Path, about, whose, not))
			}
		}
	}
	sort.Strings(gives)
	sort.Strings(reads)
	require.Equal(t, []string{
		"interactions.front-gate.outcomes.admitted[0].give | conversation front-gate admitted | about party_in_sight | " +
			"captain by heard, captain by judgement, gate-1 by heard, gate-1 by judgement, gate-2 by heard, gate-2 by judgement",
		"room.room.monsterBindings.gate-1.options[0].success[0].give | option vouch said | about party_in_sight | captain by heard, gate-1 by judgement, gate-2 by heard",
		"room.room.monsterBindings.gate-1.options[1].success[0].give | option straggler win | about speaker | captain by heard, gate-1 by judgement, gate-2 by heard",
		"room.room.monsterBindings.gate-2.options[0].success[0].give | option vouch said | about party_in_sight | captain by heard, gate-1 by heard, gate-2 by judgement",
		"room.room.monsterBindings.gate-2.options[1].success[0].give | option straggler win | about speaker | captain by heard, gate-1 by heard, gate-2 by judgement",
	}, gives)
	require.Equal(t, []string{
		"behaviorDefinitions.captain-policy.recipe.states.in-hall.transitions[1] | about the_one_named | its host",
		"behaviorDefinitions.captain-policy.recipe.states.ready.transitions[2] | about anyone | its host",
		"room.room.monsterBindings.captain.options[8].requires | about the_speaker | captain",
		"room.room.monsterBindings.captain.posts.back-to-gate.pass | about the_walker | captain",
		"room.room.monsterBindings.captain.posts.ready.pass | about the_walker | captain",
		"room.room.monsterBindings.gate-1.options[0].requires[1] | about the_speaker | gate-1",
		"room.room.monsterBindings.gate-1.options[1].requires[1] | about the_speaker | gate-1 NOT",
		"room.room.monsterBindings.gate-1.options[2].requires[1] | about the_speaker | gate-1 NOT",
		"room.room.monsterBindings.gate-1.posts.alert.pass | about the_walker | gate-1",
		"room.room.monsterBindings.gate-1.posts.challenged.pass | about the_walker | gate-1",
		"room.room.monsterBindings.gate-1.posts.posted.pass | about the_walker | gate-1",
		"room.room.monsterBindings.gate-2.options[0].requires[1] | about the_speaker | gate-2",
		"room.room.monsterBindings.gate-2.options[1].requires[1] | about the_speaker | gate-2 NOT",
		"room.room.monsterBindings.gate-2.options[2].requires[1] | about the_speaker | gate-2 NOT",
		"room.room.monsterBindings.gate-2.posts.ready.pass | about the_walker | gate-2",
	}, reads)
}
