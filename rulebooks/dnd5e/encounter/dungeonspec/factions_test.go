// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// factions_test.go is the hold-out authoring slice, step A (rpg-project#375,
// design §2): factions, minds, dispositions, the predicate grammar and the
// fact a record reveals — what the raider camp compiles to, and every way
// the file can get one of those lines wrong.
//
// Each refusal scene starts from the raider camp, changes ONE line, and
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

const raiderCampPath = "testdata/reference-raider-camp.yaml"

// campSource is the fixture's bytes, read once per scene.
func campSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(raiderCampPath)
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

// The lines the scenes below edit, spelled once.
const (
	chiefLine   = `  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders }`
	scoutLine   = `  - { id: scout,  ref: "dnd5e:monsters:skeleton",         at: [4,2],  faction: raiders }`
	factionLine = `  - { id: raiders, mind: chief }`
	dispoLine   = `  - { between: [raiders, party], stance: hostile, until: { fact: saved-wiseman } }`
	intelLine   = `  - { id: wisemans-letter, reveals: { fact: saved-wiseman } }`
	letterHolds = `      holds: [wisemans-letter] }`
	notBuilt    = "in this version a disposition turns only on a fact"
)

// TestTheRaiderCampCompiles is the fixture's own gate: the camp is a legal
// dungeon, and it carries exactly what design §1 says it carries.
func TestTheRaiderCampCompiles(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(campSource(t)))
	require.NoError(t, err)

	t.Run("one faction, and the chief is its mind", func(t *testing.T) {
		require.Equal(t, []encounter.FactionInput{{ID: "raiders", Mind: "chief"}}, compiled.Factions)
		require.Equal(t, compiled.Factions, compiled.Field.Factions, "the field carries the same list")
	})

	t.Run("hostile to the party until the chief knows the fact", func(t *testing.T) {
		require.Equal(t, []encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"raiders", "party"},
			Stance:  encounter.StanceHostile,
			Until:   encounter.TriggerFact{Fact: "saved-wiseman"},
		}}, compiled.Dispositions)
		require.Equal(t, compiled.Dispositions, compiled.Field.Dispositions)
	})

	t.Run("both skeletons are placed in the faction, by the author's word", func(t *testing.T) {
		require.Len(t, compiled.Monsters, 2)
		for _, m := range compiled.Monsters {
			require.Equal(t, "raiders", m.Faction, m.ID)
		}
		require.Equal(t, "chief", compiled.Monsters[0].ID)
		require.Equal(t, "dnd5e:monsters:skeleton-captain", compiled.Monsters[0].Ref)
		require.Equal(t, "hut", compiled.Monsters[0].Region)
		require.Equal(t, "scout", compiled.Monsters[1].ID)
		require.Equal(t, "dnd5e:monsters:skeleton", compiled.Monsters[1].Ref)
		require.Equal(t, "yard", compiled.Monsters[1].Region)
	})

	t.Run("the letter reveals the fact, and its id is compiled like a door's", func(t *testing.T) {
		require.Equal(t, []encounter.IntelRecord{{
			ID: "reference-raider-camp/wisemans-letter", Reveals: encounter.RevealTargets{Fact: "saved-wiseman"},
		}}, compiled.Intel)
		letter := propByID(compiled.Field, "letter")
		require.True(t, letter.Holdable, "the letter can be picked up")
		require.Equal(t, []encounter.IntelID{"reference-raider-camp/wisemans-letter"}, letter.Holds)
	})

	t.Run("the front gate stands where the letter lies, and the party faces the yard", func(t *testing.T) {
		require.Len(t, compiled.Field.Exits, 1)
		require.Equal(t, "front-gate", compiled.Field.Exits[0].ID)
		require.Equal(t, propByID(compiled.Field, "letter").At, compiled.Field.Exits[0].At)
		require.Equal(t, "e", compiled.StartFacing)
		require.Equal(t, "gate", compiled.PartyStart[0].Region)
	})

	t.Run("the scenario binds the faction", func(t *testing.T) {
		require.Equal(t, map[string]map[string]string{"hold-out": {"convince": "raiders"}}, compiled.Scenarios)
	})
}

func propByID(field encounter.FieldInput, id encounter.PropID) encounter.PropInput {
	for _, p := range field.Props {
		if p.ID == id {
			return p
		}
	}
	return encounter.PropInput{}
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
			old:  scoutLine, replacement: strings.Replace(scoutLine, "faction: raiders", "faction: kobolds", 1),
			want: []string{"place[1].faction", "no faction in this dungeon has that id"},
		},
		{
			name: "party on a placement",
			old:  scoutLine, replacement: strings.Replace(scoutLine, "faction: raiders", "faction: party", 1),
			want: []string{"place[1].faction", "players' side"},
		},
		{
			name: "a faction on a prop",
			old:  letterHolds, replacement: `      holds: [wisemans-letter], faction: raiders }`,
			want: []string{"place[2].faction", "is not a monster"},
		},
		{
			name: "a mind outside its faction",
			old:  chiefLine, replacement: strings.Replace(chiefLine, "faction: raiders", "faction: monsters", 1),
			want: []string{"factions[0].mind", "in its own faction"},
		},
		{
			name: "a mind that is a prop",
			old:  factionLine, replacement: `  - { id: raiders, mind: letter }`,
			want: []string{"factions[0].mind", "is a prop"},
		},
		{
			name: "a mind that names nothing",
			old:  factionLine, replacement: `  - { id: raiders, mind: nobody }`,
			want: []string{"factions[0].mind", "no placement in this dungeon has that id"},
		},
		{
			name: "party declared",
			old:  factionLine, replacement: factionLine + "\n  - { id: party }",
			want: []string{"factions[1].id", "never declared"},
		},
		{
			name: "a faction declared twice",
			old:  factionLine, replacement: factionLine + "\n  - { id: raiders }",
			want: []string{"factions[1].id", "already declared at factions[0]"},
		},
		{
			name: "two dispositions for one pair, in either order",
			old:  dispoLine, replacement: dispoLine + "\n  - { between: [party, raiders], stance: neutral }",
			want: []string{"dispositions[1].between", "already have a disposition at dispositions[0]"},
		},
		{
			name: "an until on a stance that is not hostile",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "stance: hostile", "stance: neutral", 1),
			want: []string{"dispositions[0].until", "only a hostile pair"},
		},
		{
			name: "an unknown faction in a disposition",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "[raiders, party]", "[kobolds, party]", 1),
			want: []string{"dispositions[0].between[0]", "not a faction in this dungeon"},
		},
		{
			name: "a disposition between a faction and itself",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "[raiders, party]", "[raiders, raiders]", 1),
			want: []string{"dispositions[0].between", "names \"raiders\" twice"},
		},
		{
			name: "a stance outside the closed set",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "stance: hostile", "stance: angry", 1),
			want: []string{"dispositions[0].stance", "not a stance"},
		},
		{
			name: "an until on a fall is not built yet",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "{ fact: saved-wiseman }", "{ down: chief }", 1),
			want: []string{"dispositions[0].until", notBuilt},
		},
		{
			name: "an until on a round is not built yet",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "{ fact: saved-wiseman }", "{ round: 6 }", 1),
			want: []string{"dispositions[0].until", notBuilt},
		},
		{
			name: "an until on another stance is not built yet",
			old:  dispoLine, replacement: strings.Replace(dispoLine, "{ fact: saved-wiseman }",
				"{ stance: { between: [monsters, party], is: neutral } }", 1),
			want: []string{"dispositions[0].until", notBuilt},
		},
		{
			name: "a faction of many waiting for a fact with no mind",
			old:  factionLine, replacement: `  - { id: raiders }`,
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
	t.Run("a faction of one needs no mind — the compiler declares its member", func(t *testing.T) {
		source := edited(t, factionLine, `  - { id: raiders }`)
		source = strings.Replace(source, scoutLine, strings.Replace(scoutLine, "faction: raiders", "faction: monsters", 1), 1)
		require.Empty(t, defectsIn(t, source))
		compiled, err := dungeonspec.Load([]byte(source))
		require.NoError(t, err)
		require.Equal(t, []encounter.FactionInput{{ID: "raiders", Mind: "chief"}}, compiled.Factions,
			"the singleton default is declared at compile, never inferred by the run")
	})
	t.Run("an until fact no record reveals — the dungeon allows, the scenario refuses", func(t *testing.T) {
		source := edited(t, intelLine, `  - { id: wisemans-letter, reveals: { door: gate-yard } }`)
		require.Empty(t, defectsIn(t, source))
	})
	t.Run("monsters and party may be named in a disposition", func(t *testing.T) {
		source := edited(t, dispoLine, dispoLine+"\n  - { between: [monsters, party], stance: neutral }")
		require.Empty(t, defectsIn(t, source))
	})
	t.Run("monsters may be declared, which is how the unauthored side gets a mind", func(t *testing.T) {
		source := edited(t, factionLine, factionLine+"\n  - { id: monsters, mind: scout }")
		source = strings.Replace(source, scoutLine, strings.Replace(scoutLine, "faction: raiders", "faction: monsters", 1), 1)
		require.Empty(t, defectsIn(t, source))
		compiled, err := dungeonspec.Load([]byte(source))
		require.NoError(t, err)
		require.Equal(t, []encounter.FactionInput{{ID: "raiders", Mind: "chief"}, {ID: "monsters", Mind: "scout"}}, compiled.Factions)
	})
	t.Run("the undeclared monsters side cannot be waited on for a fact", func(t *testing.T) {
		source := edited(t, dispoLine, dispoLine+
			"\n  - { between: [monsters, party], stance: hostile, until: { fact: saved-wiseman } }")
		requireDefect(t, defectsIn(t, source), "dispositions[1].until", "is not declared, so it has no mind")
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
		{"a stance with no is", "{ stance: { between: [raiders, party] } }", "does not say which stance"},
		{"a stance with no between", "{ stance: { is: neutral } }", "does not say which pair"},
		{"a stance with an unknown key", "{ stance: { between: [raiders, party], is: neutral, was: hostile } }",
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

// TestThePredicateFormsAreCheckedAtTheirPath is the per-form check the
// grammar keeps for the consumers step B adds (arrives, endings): a round is
// counted from 1, a fall names a monster placement. Reached through the
// validator directly, since no step-A field accepts these forms on an until.
func TestThePredicateFormsAreCheckedAtTheirPath(t *testing.T) {
	zero := 0
	t.Run("round", func(t *testing.T) {
		p := &dungeonspec.PredicateSpec{Round: &zero}
		require.Equal(t, "round", p.Form())
		require.Equal(t, "{ round: 0 }", p.String())
	})
	t.Run("stance renders as the one nested key", func(t *testing.T) {
		p := &dungeonspec.PredicateSpec{Stance: &dungeonspec.StancePredicateSpec{Between: [2]string{"raiders", "party"}, Is: "neutral"}}
		require.Equal(t, "{ stance: { between: [raiders, party], is: neutral } }", p.String())
	})
}
