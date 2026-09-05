// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// factions_test.go is the hold-out authoring slice, step A (rpg-project#375,
// design §2): factions, minds, dispositions, the predicate grammar and the
// fact a record reveals — what the goblin camp compiles to, and every way
// the file can get one of those lines wrong.
//
// Each refusal scene starts from the goblin camp, changes ONE line, and
// asserts the author is told about that line in words a form-filler can act
// on — heirloom_test.go's discipline, one slice on.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

const goblinCampPath = "testdata/reference-goblin-camp.yaml"

// campSource is the fixture's bytes, read once per scene.
func campSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(goblinCampPath)
	require.NoError(t, err)
	return string(raw)
}

// edited is the camp with exactly one line changed — refused when the line
// is not there once, so a scene cannot silently test the unedited file.
func edited(t *testing.T, old, replacement string) string {
	t.Helper()
	source := campSource(t)
	require.Equal(t, 1, strings.Count(source, old), "the line to edit appears once: %q", old)
	return strings.Replace(source, old, replacement, 1)
}

// campLine constants: the lines the scenes below edit, spelled once.
const (
	chiefLine   = `  - { id: chief,  ref: "dnd5e:monsters:goblin-boss", at: [12,4], faction: goblins }`
	scoutLine   = `  - { id: scout,  ref: "dnd5e:monsters:goblin",      at: [4,2],  faction: goblins }`
	factionLine = `  - { id: goblins, mind: chief }`
	dispoLine   = `  - { between: [goblins, party], stance: hostile, until: { fact: saved-wiseman } }`
	intelLine   = `  - { id: wisemans-letter, reveals: { fact: saved-wiseman } }`
	letterHolds = `      holds: [wisemans-letter] }`
)

// TestTheGoblinCampCompiles is the fixture's own gate: the camp is a legal
// dungeon, and it carries exactly what design §1 says it carries.
func TestTheGoblinCampCompiles(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(campSource(t)))
	require.NoError(t, err)

	t.Run("one faction, and the chief is its mind", func(t *testing.T) {
		require.Equal(t, []encounter.FactionInput{{ID: "goblins", Mind: "chief"}}, compiled.Factions)
		require.Equal(t, compiled.Factions, compiled.Field.Factions, "the field carries the same list")
	})

	t.Run("hostile to the party until the chief knows the fact", func(t *testing.T) {
		require.Equal(t, []encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"goblins", "party"},
			Stance:  encounter.StanceHostile,
			Until:   encounter.TriggerFact{Fact: "saved-wiseman"},
		}}, compiled.Dispositions)
		require.Equal(t, compiled.Dispositions, compiled.Field.Dispositions)
	})

	t.Run("both goblins are placed in the faction, by the author's word", func(t *testing.T) {
		require.Len(t, compiled.Monsters, 2)
		for _, m := range compiled.Monsters {
			require.Equal(t, "goblins", m.Faction, m.ID)
		}
		require.Equal(t, "chief", compiled.Monsters[0].ID)
		require.Equal(t, "hut", compiled.Monsters[0].Region)
		require.Equal(t, "scout", compiled.Monsters[1].ID)
		require.Equal(t, "yard", compiled.Monsters[1].Region)
	})

	t.Run("the letter reveals the fact, and its id is compiled like a door's", func(t *testing.T) {
		require.Equal(t, []encounter.IntelRecord{{
			ID: "reference-goblin-camp/wisemans-letter", Reveals: encounter.RevealTargets{Fact: "saved-wiseman"},
		}}, compiled.Intel)
		var letter encounter.PropInput
		for _, p := range compiled.Field.Props {
			if p.ID == "letter" {
				letter = p
			}
		}
		require.True(t, letter.Holdable, "the letter can be picked up")
		require.Equal(t, []encounter.IntelID{"reference-goblin-camp/wisemans-letter"}, letter.Holds)
	})

	t.Run("the front gate stands where the letter lies, and the party faces the yard", func(t *testing.T) {
		require.Len(t, compiled.Field.Exits, 1)
		require.Equal(t, "front-gate", compiled.Field.Exits[0].ID)
		var letterAt encounter.PropInput
		for _, p := range compiled.Field.Props {
			if p.ID == "letter" {
				letterAt = p
			}
		}
		require.Equal(t, letterAt.At, compiled.Field.Exits[0].At)
		require.Equal(t, "e", compiled.StartFacing)
		require.Equal(t, "gate", compiled.PartyStart[0].Region)
	})

	t.Run("the scenario binds the faction", func(t *testing.T) {
		require.Equal(t, map[string]map[string]string{"hold-out": {"convince": "goblins"}}, compiled.Scenarios)
	})
}

// TestTheCampRefusesEachWrongLine is design §2's refusal list, one scene per
// row, each at the field it names.
func TestTheCampRefusesEachWrongLine(t *testing.T) {
	scenes := []struct {
		name, old, replacement string
		want                   []string
	}{
		{
			name: "an unknown faction on a placement",
			old:  scoutLine, replacement: strings.Replace(scoutLine, "faction: goblins", "faction: kobolds", 1),
			want: []string{"place[1].faction", "no faction in this dungeon has that id"},
		},
		{
			name: "party on a placement",
			old:  scoutLine, replacement: strings.Replace(scoutLine, "faction: goblins", "faction: party", 1),
			want: []string{"place[1].faction", "players' side"},
		},
		{
			name: "a faction on a prop",
			old:  letterHolds, replacement: `      holds: [wisemans-letter], faction: goblins }`,
			want: []string{"place[2].faction", "is not a monster"},
		},
		{
			name: "a mind outside its faction",
			old:  chiefLine, replacement: strings.Replace(chiefLine, "faction: goblins", "faction: monsters", 1),
			want: []string{"factions[0].mind", "in its own faction"},
		},
		{
			name: "a mind that is a prop",
			old:  factionLine, replacement: `  - { id: goblins, mind: letter }`,
			want: []string{"factions[0].mind", "is a prop"},
		},
		{
			name: "a mind that names nothing",
			old:  factionLine, replacement: `  - { id: goblins, mind: nobody }`,
			want: []string{"factions[0].mind", "no placement in this dungeon has that id"},
		},
		{
			name: "party declared",
			old:  factionLine, replacement: factionLine + "\n  - { id: party }",
			want: []string{"factions[1].id", "never declared"},
		},
		{
			name: "a faction declared twice",
			old:  factionLine, replacement: factionLine + "\n  - { id: goblins }",
			want: []string{"factions[1].id", "already declared at factions[0]"},
		},
		{
			name: "two dispositions for one pair, in either order",
			old:  dispoLine, replacement: dispoLine + "\n  - { between: [party, goblins], stance: neutral }",
			want: []string{"dispositions[1].between", "already have a disposition at dispositions[0]"},
		},
		{
			name: "an until on a stance that is not hostile",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "stance: hostile", "stance: neutral", 1),
			want: []string{"dispositions[0].until", "only a hostile pair"},
		},
		{
			name: "an unknown faction in a disposition",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "[goblins, party]", "[kobolds, party]", 1),
			want: []string{"dispositions[0].between[0]", "not a faction in this dungeon"},
		},
		{
			name: "a disposition between a faction and itself",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "[goblins, party]", "[goblins, goblins]", 1),
			want: []string{"dispositions[0].between", "names \"goblins\" twice"},
		},
		{
			name: "a stance outside the closed set",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "stance: hostile", "stance: angry", 1),
			want: []string{"dispositions[0].stance", "not a stance"},
		},
		{
			name: "an unknown placement in down",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "{ fact: saved-wiseman }", "{ down: nobody }", 1),
			want: []string{"dispositions[0].until.down", "not a placement in this dungeon"},
		},
		{
			name: "a prop in down",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "{ fact: saved-wiseman }", "{ down: letter }", 1),
			want: []string{"dispositions[0].until.down", "only a monster can be down"},
		},
		{
			name: "a round that never starts",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "{ fact: saved-wiseman }", "{ round: 0 }", 1),
			want: []string{"dispositions[0].until.round", "a round starts at 1"},
		},
		{
			name: "a faction of many waiting for a fact with no mind",
			old:  factionLine, replacement: `  - { id: goblins }`,
			want: []string{"dispositions[0].until", "name a mind, or the faction cannot learn"},
		},
		{
			name: "a record revealing both a door and a fact",
			old:  intelLine, replacement: `  - { id: wisemans-letter, reveals: { door: gate-yard, fact: saved-wiseman } }`,
			want: []string{"intel[0].reveals", "exactly one thing"},
		},
		{
			name: "a record revealing nothing",
			old:  intelLine, replacement: `  - { id: wisemans-letter, reveals: {} }`,
			want: []string{"intel[0].reveals", "does not say what it reveals"},
		},
		{
			name: "a stance a pair can never reach",
			old:  dispoLine, replacement: dispoLine +
				"\n  - { between: [monsters, party], stance: hostile, until: { stance: { between: [goblins, party], is: allied } } }",
			want: []string{"dispositions[1].until.stance", "can never be allied"},
		},
		{
			name: "a stance a pair holds from the start",
			old:  dispoLine, replacement: dispoLine +
				"\n  - { between: [monsters, party], stance: hostile, until: { stance: { between: [goblins, party], is: hostile } } }",
			want: []string{"dispositions[1].until.stance", "from the start"},
		},
		{
			name: "a disposition waiting on its own stance",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "{ fact: saved-wiseman }",
				"{ stance: { between: [party, goblins], is: neutral } }", 1),
			want: []string{"dispositions[0].until.stance", "its own stance"},
		},
		{
			name: "dispositions waiting on each other in a ring",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "{ fact: saved-wiseman }",
				"{ stance: { between: [monsters, party], is: neutral } }", 1) +
				"\n  - { between: [monsters, party], stance: hostile, until: { stance: { between: [goblins, party], is: neutral } } }",
			want: []string{"dispositions[0].until.stance", "in a ring"},
		},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			requireDefect(t, defectsIn(t, edited(t, sc.old, sc.replacement)), sc.want...)
		})
	}
}

// TestTheCampAllowsWhatTheDesignAllows is the other half of §2: the lines
// that look like refusals and are not.
func TestTheCampAllowsWhatTheDesignAllows(t *testing.T) {
	t.Run("a faction of one needs no mind — its member is the mind", func(t *testing.T) {
		source := edited(t, factionLine, `  - { id: goblins }`)
		source = strings.Replace(source, scoutLine, strings.Replace(scoutLine, "faction: goblins", "faction: monsters", 1), 1)
		require.Empty(t, defectsIn(t, source))
	})
	t.Run("an until fact no record reveals — the dungeon allows, the scenario refuses", func(t *testing.T) {
		source := edited(t, intelLine, `  - { id: wisemans-letter, reveals: { door: gate-yard } }`)
		require.Empty(t, defectsIn(t, source))
	})
	t.Run("monsters may be declared, which is how the unauthored side gets a mind", func(t *testing.T) {
		source := edited(t, factionLine, factionLine+"\n  - { id: monsters, mind: scout }")
		source = strings.Replace(source, scoutLine, strings.Replace(scoutLine, "faction: goblins", "faction: monsters", 1), 1)
		require.Empty(t, defectsIn(t, source))
	})
	t.Run("a stance predicate on a pair that can reach it", func(t *testing.T) {
		source := edited(t, dispoLine, dispoLine+
			"\n  - { between: [monsters, party], stance: hostile, until: { stance: { between: [goblins, party], is: neutral } } }")
		require.Empty(t, defectsIn(t, source))
	})
	t.Run("a scenario may bind a faction", func(t *testing.T) {
		require.Empty(t, defectsIn(t, campSource(t)))
	})
}

// TestThePredicateDecodesStrictly is the grammar's own gate: exactly one
// form, no unknown key, and each refusal names the line.
func TestThePredicateDecodesStrictly(t *testing.T) {
	scenes := []struct {
		name, predicate, want string
	}{
		{"two forms", "{ fact: saved-wiseman, round: 6 }", "says both `fact` and `round`"},
		{"no form", "{}", "says nothing"},
		{"an empty form", "{ fact: }", "`fact` says nothing"},
		{"an unknown key", "{ facts: saved-wiseman }", "field facts not found in type dungeonspec.PredicateSpec"},
		{"is outside the stance form", "{ fact: saved-wiseman, is: neutral }", "field is not found"},
		{"a stance with no is", "{ stance: { between: [goblins, party] } }", "does not say which stance"},
		{"a stance with no between", "{ stance: { is: neutral } }", "does not say which pair"},
		{"a stance with an unknown key", "{ stance: { between: [goblins, party], is: neutral, was: hostile } }",
			"field was not found in type dungeonspec.StancePredicateSpec"},
		{"a scalar", "round", "a predicate is exactly one of"},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			source := edited(t, dispoLine, strings.Replace(dispoLine, "{ fact: saved-wiseman }", sc.predicate, 1))
			_, err := dungeonspec.Decode([]byte(source))
			require.Error(t, err)
			require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
			require.Contains(t, err.Error(), sc.want)
		})
	}
}
