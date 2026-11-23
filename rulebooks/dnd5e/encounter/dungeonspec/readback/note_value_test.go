// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// note_value_test.go is THE READ-BACK SAYS A NOTE'S VALUE (rpg-project#1007,
// rpg-project#1115): where a note is given or asked and carries a value, the
// sentence says it after the topic and the `topic` connection carries it in
// `is`; a note with no value is said exactly as before.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// valueStep is the step both states carry: a checked line that failed, with the
// guard already warned, sends the machine back to the gate.
const valueStep = `          transitions:
            - on: heard
              when:
                all:
                  - { fact: { name: heard.claim, op: eq, value: failed } }
                  - { fact: { name: "noted.as:standing", op: eq, value: warned } }
              to: back-to-gate
              reason: lied twice
`

const valueStates = "        in-hall:\n" + valueStep + "        back-to-gate:\n" + valueStep

const valueOutcome = `    outcomes:
      back-to-gate:
        - give: { topic: standing, as: barred, to: itself, about: the_one_named }
`

const valueLie = "          - id: lie\n" +
	"            say: \"We're expected.\"\n" +
	"            check: [{ ability: persuasion, dc: 10 }]\n" +
	"            requires: { not: { noted: standing } }\n" +
	"            failure:\n" +
	"              - say: Liar.\n" +
	"                give: { topic: standing, as: warned, to: spoken_to, about: speaker }\n"

const valueLieAgain = "          - id: lie-again\n" +
	"            say: \"Truly.\"\n" +
	"            check: [{ ability: persuasion, dc: 10 }]\n" +
	"            requires: { noted: { topic: standing, as: warned } }\n" +
	"            failure:\n" +
	"              - say: Liar again.\n"

// valueSite is slice 3a's two-strikes site, as an author writes it.
func valueSite() string {
	definitions := "behaviorDefinitions:\n  captain-policy:\n    recipe:\n" +
		"      ref: site:behaviors:captain-policy\n      initial: in-hall\n      states:\n" + valueStates + valueOutcome
	gameplay := "    monsterBindings:\n" +
		"      goblin-1:\n        behaviorState: policy\n        behaviors: [{ id: policy, use: captain-policy }]\n" +
		"        options:\n" + valueLie + valueLieAgain +
		"      goblin-2:\n        behaviorState: policy\n        behaviors: [{ id: policy, use: captain-policy }]\n"
	return fmt.Sprintf(`
version: 5
key: refusal-fixture
play: { void: transparent, lighting: bright, standing: centre-covered }
%s
room:
  version: 3
  id: room-1
  name: A Room
  coordinateFrame: { hexRadius: 1 }
  scene:
    version: 1
    id: scene-1
    name: A Room
    items:
      - { id: table, kind: prop, transform: { x: 0, y: 0, z: 2, rotationY: 0 } }
    groups: []
  room:
    implicitRegionId: room-1-region
    regions:
      - id: room-1-region
        name: A Room
        lighting: { intensity: 1 }
        cells:
          - r: 0
            q: [[0, 2]]
          - r: 1
            q: [[0, 1]]
    propDeclarations:
      table: { blocksMovement: false, blocksLineOfSight: false, footprint: { width: 1, depth: 1, offsetX: 0, offsetZ: 0 } }
    arrangementDeclarations:
      nook:
        stamped-chair: { blocksMovement: false, blocksLineOfSight: false, footprint: { width: 1, depth: 1, offsetX: 0, offsetZ: 0 } }
    partyStart: { q: 0, r: 0 }
    monsterDeclarations:
      - { id: goblin-1, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 2, r: 0 } } }
      - { id: goblin-2, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 1, r: 1 } } }
%s
`, definitions, gameplay)
}

// valued is the statement of kind at path, once (a hub repeats it under its
// own kinds).
func valued(t *testing.T, rb *readback.ReadBack, kind readback.Kind, path string) readback.Statement {
	t.Helper()
	var found []readback.Statement
	for _, st := range rb.Statements() {
		if st.Path == path && st.Kind == kind {
			found = append(found, st)
		}
	}
	require.Len(t, found, 1, "one statement at %s", path)
	return found[0]
}

// topicEdges is st's `topic` connections of role.
func topicEdges(st readback.Statement, role string) []readback.Connection {
	var out []readback.Connection
	for _, c := range st.Connects {
		if c.Kind == "topic" && c.Role == role {
			out = append(out, c)
		}
	}
	return out
}

func valueReadBack(t *testing.T) *readback.ReadBack {
	t.Helper()
	rb, err := readback.Of([]byte(valueSite()))
	require.NoError(t, err)
	return rb
}

func TestNoteValue_ALinesGiveSaysItsValue(t *testing.T) {
	rb := valueReadBack(t)
	st := valued(t, rb, readback.KindLineGrant, "room.room.monsterBindings.goblin-1.options[0].failure[0].give")
	require.Contains(t, st.Sentence, "is given the topic `standing` as `warned`, and notes it about", "the sentence says the value")
	require.Equal(t, []readback.Connection{{Kind: "topic", ID: "standing", Role: "grants", Is: "warned"}}, topicEdges(st, "grants"),
		"the connection carries the value")
	require.Contains(t, rb.FoldedEachText(), "notes `standing` as `warned` about", "the folded text says the value")
}

func TestNoteValue_AStatesGiveSaysItsValue(t *testing.T) {
	rb := valueReadBack(t)
	st := valued(t, rb, readback.KindRoutineOutcome, "behaviorDefinitions.captain-policy.outcomes.back-to-gate[0]")
	require.Contains(t, st.Sentence, "it notes `standing` as `barred` about", "the sentence says the value")
	require.Equal(t, []readback.Connection{{Kind: "topic", ID: "standing", Role: "grants", Is: "barred"}}, topicEdges(st, "grants"))
	require.Contains(t, rb.FoldedEachText(), "it notes `standing` as `barred` about", "the folded text says the value")
	var cause []readback.Connection
	for _, c := range st.Connects {
		if c.Kind == "state" {
			cause = append(cause, c)
		}
	}
	require.Equal(t, []readback.Connection{{Kind: "state", ID: "back-to-gate", Role: "enters"}}, cause, "the cause is still the state entered")
}

func TestNoteValue_ALinesAskSaysItsValueAndAPlainAskSaysNone(t *testing.T) {
	rb := valueReadBack(t)
	asks := func(path string) readback.Statement {
		var out []readback.Statement
		for _, st := range rb.Statements() {
			if !notePointer(st) && strings.HasPrefix(st.Path, path) && len(topicEdges(st, "asks")) > 0 {
				out = append(out, st)
			}
		}
		require.NotEmpty(t, out, "a statement under %s asks the topic", path)
		return out[0]
	}
	valuedAsk := asks("room.room.monsterBindings.goblin-1.options[1]")
	require.Contains(t, valuedAsk.Sentence, "has noted `standing` as `warned` about", "the sentence says the value")
	require.Equal(t, []readback.Connection{{Kind: "topic", ID: "standing", Role: "asks", Is: "warned"}}, topicEdges(valuedAsk, "asks"),
		"the connection carries the value and is not negated")

	plain := asks("room.room.monsterBindings.goblin-1.options[0]")
	require.Contains(t, plain.Sentence, "has NOT noted `standing` about", "a plain ask is said as today")
	require.NotContains(t, plain.Sentence, " as `", "a plain ask says no value")
	require.Equal(t, []readback.Connection{{Kind: "topic", ID: "standing", Role: "asks", Not: true}}, topicEdges(plain, "asks"),
		"a plain ask carries no `is`")
}

func TestNoteValue_APostsPassSaysItsValue(t *testing.T) {
	raw, err := os.ReadFile("../testdata/world-builder-v4-castle.yaml")
	require.NoError(t, err)
	source := string(raw)
	const give = "            topic: caught-lying\n            to: itself\n            about: the_one_named\n"
	const pass = "              - not:\n                  noted: caught-lying\n            at:\n"
	require.Equal(t, 1, strings.Count(source, give))
	require.Equal(t, 1, strings.Count(source, pass))
	source = strings.Replace(source, give, give+"            as: barred\n", 1)
	source = strings.Replace(source, pass, "              - not:\n                  noted: { topic: caught-lying, as: barred }\n            at:\n", 1)
	rb, err := readback.Of([]byte(source))
	require.NoError(t, err)

	var found []readback.Statement
	for _, st := range rb.Statements() {
		for _, c := range st.Connects {
			if !notePointer(st) && c.Kind == "topic" && c.ID == "caught-lying" && c.Role == "asks" && c.Not && c.Is == "barred" {
				found = append(found, st)
				break
			}
		}
	}
	require.NotEmpty(t, found, "a post's pass asks not-barred, negated, with the value")
	for _, st := range found {
		require.Contains(t, st.Sentence, "has NOT noted `caught-lying` as `barred` about", "the sentence says the value")
	}
}

func TestNoteValue_AStepsAskSaysItsValue(t *testing.T) {
	rb := valueReadBack(t)
	var found []readback.Statement
	for _, st := range rb.Statements() {
		for _, c := range st.Connects {
			if !notePointer(st) && c.Kind == "topic" && c.ID == "standing" && c.Role == "asks" && st.Path != "" && strings.HasPrefix(st.Path, "behaviorDefinitions.") {
				found = append(found, st)
				break
			}
		}
	}
	require.NotEmpty(t, found, "a step's `noted.as:standing` asks the topic")
	st := found[0]
	require.Contains(t, st.Sentence, "has noted `standing` as `warned` about", "the sentence says the value")
	require.Equal(t, []readback.Connection{{Kind: "topic", ID: "standing", Role: "asks", Is: "warned"}}, topicEdges(st, "asks"))
	var about []string
	for _, c := range st.Connects {
		if c.Kind == "who" && c.Role == "about" && c.At == "standing" {
			about = append(about, c.ID)
		}
	}
	require.Equal(t, []string{"the_one_named"}, about, "whom it is about")
}

func TestNoteValue_NothingAboutAsIsNotYetSaid(t *testing.T) {
	rb := valueReadBack(t)
	for _, un := range rb.NotYetSaid {
		require.False(t, strings.HasSuffix(un.Path, ".as"), "a value is said, never left unsaid: %v", un)
	}
}
