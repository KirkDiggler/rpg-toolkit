// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package scenarios_test

// holdout_test.go is the hold-out's own refusals (rpg-project#375, design
// §2): the dungeon allows an until fact no record reveals; this scenario is
// where "a hold-out nobody can win" is refused, in the form's own words.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/scenarios"
)

// campFacts is the goblin camp's field, narrowed — with the mind declared,
// the disposition hostile until the fact, and a record that reveals it —
// then edited one fact at a time by the scenes below.
func campFacts(edit func(*encounter.FieldInput), monsters ...encounter.FactionID) *scenarios.DungeonFacts {
	field := encounter.FieldInput{
		Factions: []encounter.FactionInput{{ID: factionID, Mind: "chief"}},
		Dispositions: []encounter.DispositionInput{{
			Between: [2]encounter.FactionID{factionID, encounter.FactionParty},
			Stance:  encounter.StanceHostile,
			Until:   encounter.TriggerFact{Fact: factID},
		}},
		Intel: []encounter.IntelRecord{{ID: "letter", Reveals: encounter.RevealTargets{Fact: factID}}},
	}
	if edit != nil {
		edit(&field)
	}
	return scenarios.FactsFrom(field, monsters...)
}

func TestTheHoldOutDeclaresAStanceEnding(t *testing.T) {
	s, ok := scenarios.Lookup(scenarios.HoldOutID)
	require.True(t, ok)

	declared, err := s.New(map[string]string{scenarios.FieldConvince: factionID}, campFacts(nil, factionID, factionID))
	require.NoError(t, err)
	require.Equal(t, encounter.FactionID(factionID), declared.Convince)
	require.Equal(t, []encounter.EndingInput{{
		Key: scenarios.HoldOutID,
		Trigger: encounter.TriggerStance{
			Between: [2]encounter.FactionID{factionID, encounter.FactionParty},
			Stance:  encounter.StanceNeutral,
		},
	}}, declared.Endings, "convince is sugar for a stance ending (R10)")
}

func TestTheHoldOutRefusesAHoldOutNobodyCanWin(t *testing.T) {
	s, ok := scenarios.Lookup(scenarios.HoldOutID)
	require.True(t, ok)
	bind := func(faction string) map[string]string {
		return map[string]string{scenarios.FieldConvince: faction}
	}

	scenes := []struct {
		name  string
		cfg   map[string]string
		facts *scenarios.DungeonFacts
		want  string
	}{
		{
			name: "a faction the dungeon does not have",
			cfg:  bind("kobolds"), facts: campFacts(nil),
			want: "is not a faction this dungeon has",
		},
		{
			name: "a faction with no mind",
			cfg:  bind(factionID),
			facts: campFacts(func(f *encounter.FieldInput) {
				f.Factions = []encounter.FactionInput{{ID: factionID}}
			}, factionID, factionID),
			want: "has no mind",
		},
		{
			name: "the party itself",
			cfg:  bind(encounter.FactionParty), facts: campFacts(nil),
			want: "has no mind",
		},
		{
			name: "no hostile disposition with an until toward the party",
			cfg:  bind(factionID),
			facts: campFacts(func(f *encounter.FieldInput) {
				f.Dispositions = nil
			}),
			want: "is not hostile to the party until a fact",
		},
		{
			name: "nothing reveals the fact",
			cfg:  bind(factionID),
			facts: campFacts(func(f *encounter.FieldInput) {
				f.Intel = nil
			}),
			want: "a hold-out nobody can win",
		},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			_, err := s.New(sc.cfg, sc.facts)
			require.Error(t, err)
			require.Contains(t, err.Error(), scenarios.FieldConvince, "the refusal names the field")
			require.Contains(t, err.Error(), sc.want)
		})
	}

	t.Run("a faction of one needs no declared mind", func(t *testing.T) {
		facts := campFacts(func(f *encounter.FieldInput) {
			f.Factions = []encounter.FactionInput{{ID: factionID}}
		}, factionID)
		_, err := s.New(bind(factionID), facts)
		require.NoError(t, err)
	})
}
