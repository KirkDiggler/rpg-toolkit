// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// arrivals_test.go is the hold-out authoring slice, step B (rpg-project#375,
// design §2, R6, R10): `arrives` on a placement and `endings` in the file —
// what the shipped camp compiles them to, every way a line can be wrong, and
// the pin that a scenario's own field is sugar for an authored ending.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/scenarios"
)

// The step-B lines of the shipped camp, spelled once.
const (
	letterArrivesLine = `      holds: [wisemans-letter], arrives: { round: 6 } }`
	zombie1Line       = `  - { id: reinforcement-1, ref: "dnd5e:monsters:zombie", at: [1,4], faction: raiders, arrives: { down: chief } }`
	zombie2Line       = `  - { id: reinforcement-2, ref: "dnd5e:monsters:zombie", at: [2,4], faction: raiders, arrives: { down: chief } }`
	scenariosLine     = "scenarios:\n"
)

// refusals runs one edited camp through Load and returns the paths and
// messages it was refused with — empty when it compiled.
func refusals(t *testing.T, source string) []dungeonspec.FieldError {
	t.Helper()
	_, err := dungeonspec.Load([]byte(source))
	if err == nil {
		return nil
	}
	var verr *dungeonspec.ValidationError
	require.ErrorAs(t, err, &verr, "a validation failure, not a decode one: %v", err)
	return verr.Errors
}

// requireRefusedAt asserts one refusal landed at the path, in the words.
func requireRefusedAt(t *testing.T, errs []dungeonspec.FieldError, path string, words ...string) {
	t.Helper()
	for _, e := range errs {
		if e.Path != path {
			continue
		}
		for _, w := range words {
			require.Contains(t, e.Message, w, "at %s", path)
		}
		return
	}
	require.Failf(t, "no refusal at the path", "%s not in %v", path, errs)
}

// TestArrivesRefusesEachWrongLine is design §2's refusal list for
// `arrives`, one scene per row, each at `place[i].arrives` and the sub-path
// of the form it takes.
func TestArrivesRefusesEachWrongLine(t *testing.T) {
	scenes := []struct {
		name, old, replacement, path string
		want                         []string
	}{
		{
			name: "a round counted from zero",
			old:  letterArrivesLine, replacement: strings.Replace(letterArrivesLine, "round: 6", "round: 0", 1),
			path: "place[2].arrives.round", want: []string{"a round is counted from 1"},
		},
		{
			name: "a fall of nobody",
			old:  zombie1Line, replacement: strings.Replace(zombie1Line, "down: chief", "down: nobody", 1),
			path: "place[3].arrives.down", want: []string{`"nobody" is not a placement`},
		},
		{
			name: "a fall of a prop",
			old:  zombie1Line, replacement: strings.Replace(zombie1Line, "down: chief", "down: letter", 1),
			path: "place[3].arrives.down", want: []string{"is a prop, and only a monster can be down"},
		},
		{
			name: "its own fall",
			old:  zombie1Line, replacement: strings.Replace(zombie1Line, "down: chief", "down: reinforcement-1", 1),
			path: "place[3].arrives.down", want: []string{"cannot wait for its own fall"},
		},
		{
			name: "a stance the pair holds from the start",
			old:  zombie1Line,
			replacement: strings.Replace(zombie1Line, "arrives: { down: chief }",
				"arrives: { stance: { between: [raiders, party], is: hostile } }", 1),
			path: "place[3].arrives.stance", want: []string{"from the start"},
		},
		{
			name: "a stance the pair can never reach",
			old:  zombie1Line,
			replacement: strings.Replace(zombie1Line, "arrives: { down: chief }",
				"arrives: { stance: { between: [raiders, party], is: allied } }", 1),
			path: "place[3].arrives.stance", want: []string{"can never be"},
		},
		{
			name: "a stance between a faction nobody declared",
			old:  zombie1Line,
			replacement: strings.Replace(zombie1Line, "arrives: { down: chief }",
				"arrives: { stance: { between: [kobolds, party], is: neutral } }", 1),
			path: "place[3].arrives.stance.between[0]", want: []string{"is not a faction in this dungeon"},
		},
		{
			name: "a stance that is not a word",
			old:  zombie1Line,
			replacement: strings.Replace(zombie1Line, "arrives: { down: chief }",
				"arrives: { stance: { between: [raiders, party], is: cranky } }", 1),
			path: "place[3].arrives.stance.is", want: []string{`"cranky" is not a stance`},
		},
		{
			name:        "a prop that arrives and has no name",
			old:         `  - { id: letter, ref: "dnd5e:props:scroll", at: [1,3], holdable: true,`,
			replacement: `  - { ref: "dnd5e:props:scroll", at: [1,3],`,
			path:        "place[2].id", want: []string{"arrives on a predicate and has no id"},
		},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			errs := refusals(t, edited(t, sc.old, sc.replacement))
			requireRefusedAt(t, errs, sc.path, sc.want...)
		})
	}

	t.Run("a ring of falls never arrives, and every member of it is told", func(t *testing.T) {
		source := edited(t, zombie1Line, strings.Replace(zombie1Line, "down: chief", "down: reinforcement-2", 1))
		source = strings.Replace(source, zombie2Line, strings.Replace(zombie2Line, "down: chief", "down: reinforcement-1", 1), 1)
		errs := refusals(t, source)
		requireRefusedAt(t, errs, "place[3].arrives.down", "none of them can ever arrive")
		requireRefusedAt(t, errs, "place[4].arrives.down", "none of them can ever arrive")
		require.Len(t, errs, 2, "%v", errs)
	})
}

// TestArrivesDecodesStrictly is the grammar's own refusals, reached through
// `arrives`: a predicate that says nothing, one that says two things, and a
// key this build does not know.
func TestArrivesDecodesStrictly(t *testing.T) {
	scenes := []struct{ name, replacement, want string }{
		{"says nothing", "arrives: { }", "says nothing"},
		{"says two things", "arrives: { round: 6, down: chief }", "says both"},
		{"an unknown key", "arrives: { when: 6 }", "field when not found"},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			_, err := dungeonspec.Load([]byte(edited(t, zombie1Line, strings.Replace(zombie1Line, "arrives: { down: chief }", sc.replacement, 1))))
			require.Error(t, err)
			require.Contains(t, err.Error(), sc.want)
		})
	}
}

// TestArrivesAllowsWhatTheDesignAllows: a `{ fact }` no record reveals is the
// dungeon's to allow (R8 — the scenario is where "nobody can win" is refused),
// and a monster that arrives needs no id — the host names the member it
// spawns.
func TestArrivesAllowsWhatTheDesignAllows(t *testing.T) {
	t.Run("a fact nothing reveals", func(t *testing.T) {
		compiled, err := dungeonspec.Load([]byte(edited(t, zombie1Line,
			strings.Replace(zombie1Line, "arrives: { down: chief }", "arrives: { fact: rumour }", 1))))
		require.NoError(t, err)
		require.Equal(t, encounter.TriggerFact{Fact: "rumour"}, compiled.Monsters[2].Arrives)
	})
	t.Run("a nameless monster", func(t *testing.T) {
		compiled, err := dungeonspec.Load([]byte(edited(t, zombie1Line,
			strings.Replace(zombie1Line, "id: reinforcement-1, ", "", 1))))
		require.NoError(t, err)
		require.Equal(t, "", compiled.Monsters[2].ID)
		require.Equal(t, encounter.TriggerMemberDown{Member: "chief"}, compiled.Monsters[2].Arrives)
	})
}

// withEndings is the shipped camp with an `endings:` block written in before
// `scenarios:`.
func withEndings(t *testing.T, entries string) string {
	t.Helper()
	return edited(t, scenariosLine, "endings:\n"+entries+"\n"+scenariosLine)
}

// TestEndingsCompileToTheEnginesTriggers is R10: every form of the grammar,
// as an ending, compiles to the same sealed trigger an until or an arrives
// does, in authored order, under the author's own id.
func TestEndingsCompileToTheEnginesTriggers(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(withEndings(t,
		"  - { id: held-out, when: { round: 6 } }\n"+
			"  - { id: chief-slain, when: { down: chief } }\n"+
			"  - { id: word-is-out, when: { fact: saved-wiseman } }\n"+
			"  - { id: turned, when: { stance: { between: [raiders, party], is: neutral } } }\n")))
	require.NoError(t, err)
	require.Equal(t, []encounter.EndingInput{
		{Key: "held-out", Trigger: encounter.TriggerRound{Round: 6}},
		{Key: "chief-slain", Trigger: encounter.TriggerMemberDown{Member: "chief"}},
		{Key: "word-is-out", Trigger: encounter.TriggerFact{Fact: "saved-wiseman"}},
		{Key: "turned", Trigger: encounter.TriggerStance{
			Between: [2]encounter.FactionID{"raiders", "party"}, Stance: encounter.StanceNeutral,
		}},
	}, compiled.Endings)
}

// TestConvinceIsSugarForAnAuthoredEnding is R10's pin: the scenario's own
// field and an ending written in the file declare the SAME thing.
func TestConvinceIsSugarForAnAuthoredEnding(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(withEndings(t,
		"  - { id: hold-out, when: { stance: { between: [raiders, party], is: neutral } } }\n")))
	require.NoError(t, err)
	require.Len(t, compiled.Endings, 1)

	scenario, ok := scenarios.Lookup(scenarios.HoldOutID)
	require.True(t, ok)
	declared, err := scenario.New(compiled.Scenarios[scenarios.HoldOutID], scenarios.FactsFrom(compiled.Field))
	require.NoError(t, err)
	require.Equal(t, declared.Endings, compiled.Endings,
		"`scenarios.hold-out.convince: raiders` is sugar for `endings: [{ id: hold-out, when: { stance: ... } }]`")
}

// TestEndingsRefuseEachWrongLine is "an ending nobody can reach", in the
// file's own paths.
func TestEndingsRefuseEachWrongLine(t *testing.T) {
	scenes := []struct {
		name, entries, path string
		want                []string
	}{
		{
			name: "no id", entries: "  - { when: { round: 6 } }\n",
			path: "endings[0].id", want: []string{"has no id"},
		},
		{
			name: "an id twice", entries: "  - { id: same, when: { round: 6 } }\n  - { id: same, when: { round: 7 } }\n",
			path: "endings[1].id", want: []string{"already declared at endings[0]"},
		},
		{
			name: "no when", entries: "  - { id: never }\n",
			path: "endings[0].when", want: []string{"does not say when it fires"},
		},
		{
			name: "a round counted from zero", entries: "  - { id: never, when: { round: 0 } }\n",
			path: "endings[0].when.round", want: []string{"a round is counted from 1"},
		},
		{
			name: "a fall of nobody", entries: "  - { id: never, when: { down: nobody } }\n",
			path: "endings[0].when.down", want: []string{"is not a placement"},
		},
		{
			name: "a stance the pair can never reach", entries: "  - { id: never, when: { stance: { between: [raiders, party], is: allied } } }\n",
			path: "endings[0].when.stance", want: []string{"can never be"},
		},
		{
			name: "a stance the pair holds from the start", entries: "  - { id: never, when: { stance: { between: [raiders, party], is: hostile } } }\n",
			path: "endings[0].when.stance", want: []string{"from the start"},
		},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			errs := refusals(t, withEndings(t, sc.entries))
			requireRefusedAt(t, errs, sc.path, sc.want...)
		})
	}

	t.Run("a key this build does not know", func(t *testing.T) {
		_, err := dungeonspec.Load([]byte(withEndings(t, "  - { id: x, when: { round: 6 }, then: party }\n")))
		require.Error(t, err)
		require.Contains(t, err.Error(), "field then not found")
	})
}
