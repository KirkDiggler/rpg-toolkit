// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// not_test.go is THE ONE NEGATION in the dialect (rpg-project#719):
// `{ not: <one predicate> }`, legal wherever a predicate is, scoped by what it
// wraps, and refused wherever it could never fire.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// notFactions is two factions civil to the party, the goblins until round 3.
const notFactions = `factions:
  - { id: goblins }
  - { id: dogs }
dispositions:
  - { between: [goblins, party], stance: neutral, until: { round: 3 } }
`

// twoDoors is a front door and a back one.
const twoDoors = "exits:\n  - { id: front-door, cell: { q: 0, r: 0 } }\n  - { id: back-door, cell: { q: 1, r: 0 } }\n"

// TestNotCompilesToTheEnginesNegation: every place a `not` is legal, it
// reaches the engine as a TriggerNot around the inner form's own trigger.
func TestNotCompilesToTheEnginesNegation(t *testing.T) {
	t.Run("an option's requires, on a fact and on a holding", func(t *testing.T) {
		compiled := loadSource(t, optionsOnGoblin(`          - id: chat
            say: "Smells good in here."
            requires: { not: { fact: caught-in-kitchen } }
            check: [{ ability: persuasion, dc: 10 }]
          - id: beg
            say: "Spare a crust?"
            requires: { not: { holds: table } }
            check: [{ ability: persuasion, dc: 10 }]
`))
		goblin := monsterNamed(t, compiled, "goblin-1")
		require.Equal(t, encounter.TriggerNot{Inner: encounter.TriggerFact{Fact: "caught-in-kitchen"}},
			optionNamed(t, goblin.Options, "chat").Requires)
		require.Equal(t, encounter.TriggerNot{Inner: encounter.TriggerHolds{Prop: "table"}},
			optionNamed(t, goblin.Options, "beg").Requires)
	})

	t.Run("an until, an ending's when and a leave ending's if", func(t *testing.T) {
		compiled, err := dungeonspec.Load([]byte(v4With(notFactions+
			"  - { between: [dogs, party], stance: neutral, until: { not: { stance: { between: [goblins, party], is: neutral } } } }\n"+
			twoDoors+`endings:
  - { id: truce-broken, when: { not: { stance: { between: [goblins, party], is: neutral } } }, outcome: failure }
  - { id: slipped-out, when: { not: { left: front-door } }, outcome: failure }
  - { id: goblin-lives, when: { left: front-door }, if: { not: { down: goblin-1 } }, outcome: failure }
  - { id: goblin-slain, when: { left: front-door }, outcome: success }`, "")))
		require.NoError(t, err)

		goblinsNeutral := encounter.TriggerStance{
			Between: [2]encounter.FactionID{"goblins", encounter.FactionParty}, Stance: encounter.StanceNeutral,
		}
		var dogs *encounter.DispositionInput
		for i := range compiled.Field.Dispositions {
			if compiled.Field.Dispositions[i].Between[0] == "dogs" {
				dogs = &compiled.Field.Dispositions[i]
			}
		}
		require.NotNil(t, dogs)
		require.Equal(t, []encounter.UntilStep{{When: encounter.TriggerNot{Inner: goblinsNeutral}}}, dogs.Until)
		require.Equal(t, []encounter.EndingInput{
			{Key: "truce-broken", Trigger: encounter.TriggerNot{Inner: goblinsNeutral}, Result: encounter.ResultFailure},
			{Key: "slipped-out", Trigger: encounter.TriggerNot{Inner: encounter.TriggerLeft{Exit: "front-door"}},
				Result: encounter.ResultFailure},
			{Key: "goblin-lives", Trigger: encounter.TriggerLeft{Exit: "front-door"},
				If: encounter.TriggerNot{Inner: encounter.TriggerMemberDown{Member: "goblin-1"}}, Result: encounter.ResultFailure},
			{Key: "goblin-slain", Trigger: encounter.TriggerLeft{Exit: "front-door"}, Result: encounter.ResultSuccess},
		}, compiled.Endings)
	})
}

// TestNotIsRefusedByName: each wrong `not` at its own path, in the inner
// form's own words where the scope is the inner form's.
func TestNotIsRefusedByName(t *testing.T) {
	const at = "room.room.monsterBindings.goblin-1.options[0].requires"
	option := func(requires string) string {
		return optionsOnGoblin("          - { id: hi, say: \"Hi.\", requires: " + requires +
			", check: [{ ability: persuasion, dc: 10 }] }\n")
	}
	for _, tc := range []struct {
		name, source, path string
		says               []string
	}{
		{name: "holds under a not on an ending",
			source: v4With("endings:\n  - { id: rich, when: { not: { holds: table } } }", ""),
			path:   "endings[0].when.not.holds", says: []string{"`holds` is only something an option requires"}},
		{name: "holds under a not on an until",
			source: v4With(notFactions+"  - { between: [dogs, party], stance: neutral, until: { not: { holds: table } } }", ""),
			path:   "dispositions[1].until.not.holds", says: []string{"`holds` is only something an option requires"}},
		{name: "holds under a not on an arrival",
			source: v4With("", "    monsterBindings:\n      goblin-2: { arrives: { not: { holds: table } } }"),
			path:   "room.room.monsterBindings.goblin-2.arrives.not.holds", says: []string{"`holds` is only something an option requires"}},
		{name: "left under a not on an until",
			source: v4With(twoDoors+notFactions+"  - { between: [dogs, party], stance: neutral, until: { not: { left: front-door } } }", ""),
			path:   "dispositions[1].until.not.left", says: []string{"only an ending waits for that"}},
		{name: "left under a not on an option's requires",
			source: v4With(twoDoors, "    monsterBindings:\n      goblin-1:\n        options:\n"+
				"          - { id: hi, say: \"Hi.\", requires: { not: { left: front-door } }, check: [{ ability: persuasion, dc: 10 }] }\n"),
			path: at + ".not", says: []string{"`left` is none of them"}},
		{name: "left under a not in a leave ending's if",
			source: v4With(twoDoors+"endings:\n  - { id: out, when: { left: front-door }, if: { not: { left: back-door } }, outcome: success }", ""),
			path:   "endings[0].if.not", says: []string{"`left` is none of them"}},
		{name: "a not of a not on an option",
			source: option("{ not: { not: { fact: x } } }"),
			path:   at + ".not", says: []string{"a not of a not is the predicate itself"}},
		{name: "a not of a not on an ending",
			source: v4With(notFactions+"endings:\n  - { id: e, when: { not: { not: { round: 2 } } } }", ""),
			path:   "endings[0].when.not", says: []string{"a not of a not is the predicate itself"}},
		{name: "a not of a round on an option",
			source: option("{ not: { round: 2 } }"),
			path:   at + ".not", says: []string{"`round` is none of them"}},
		{name: "a not of a fact waited for on an ending",
			source: v4With("endings:\n  - { id: e, when: { not: { fact: x } } }", ""),
			path:   "endings[0].when.not", says: []string{"holds from the start and can only stop holding"}},
		{name: "a not of a round waited for on an until",
			source: v4With(notFactions+"  - { between: [dogs, party], stance: neutral, until: { not: { round: 2 } } }", ""),
			path:   "dispositions[1].until.not", says: []string{"holds from the start and can only stop holding"}},
		{name: "a not of a fall waited for on an arrival",
			source: v4With("", "    monsterBindings:\n      goblin-2: { arrives: { not: { down: goblin-1 } } }"),
			path:   "room.room.monsterBindings.goblin-2.arrives.not", says: []string{"holds from the start"}},
		{name: "a not of a fall of nobody is that first",
			source: v4With("endings:\n  - { id: e, when: { not: { down: king } } }", ""),
			path:   "endings[0].when.not.down", says: []string{`"king" is not a placement in this dungeon`}},
		{name: "a not of a stance the pair is not in from the start",
			source: v4With(notFactions+"endings:\n  - { id: e, when: { not: { stance: { between: [goblins, party], is: hostile } } } }", ""),
			path:   "endings[0].when.not.stance", says: []string{"neutral from the start, not hostile"}},
		{name: "a not of a stance nothing turns",
			source: v4With(notFactions+"  - { between: [dogs, goblins], stance: allied }\n"+
				"endings:\n  - { id: e, when: { not: { stance: { between: [dogs, goblins], is: allied } } } }", ""),
			path: "endings[0].when.not.stance", says: []string{"nothing turns them, so they can never stop being allied"}},
		{name: "a not of a left on the only way out",
			source: v4With("exits:\n  - { id: front-door, cell: { q: 0, r: 0 } }\n"+
				"endings:\n  - { id: e, when: { not: { left: front-door } }, outcome: failure }", ""),
			path: "endings[0].when.not.left", says: []string{`"front-door" is the only way out`}},
		{name: "a not of a left with no outcome",
			source: v4With(twoDoors+"endings:\n  - { id: e, when: { not: { left: front-door } } }", ""),
			path:   "endings[0].outcome", says: []string{"`outcome: success` or `outcome: failure`"}},
		{name: "a not in a list of until steps",
			source: v4With(notFactions+"  - { between: [dogs, party], stance: neutral, until: [{ fact: a, then: hostile }, "+
				"{ not: { stance: { between: [goblins, party], is: neutral } }, then: allied }] }", ""),
			path: "dispositions[1].until[1]", says: []string{"a list of until steps turns on facts only"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, tc.source), tc.path, tc.says...)
		})
	}
}

// TestNotIsReadByHand: a `not` that says nothing, two things, or a typo is
// refused at decode by the predicate's own rules, one level down.
func TestNotIsReadByHand(t *testing.T) {
	for name, tc := range map[string]struct{ requires, says string }{
		"nothing inside":    {"{ not: {} }", "this predicate says nothing"},
		"a null inside":     {"{ not: }", "says nothing"},
		"two things inside": {"{ not: { fact: a, round: 2 } }", "this predicate says both `fact` and `round`"},
		"a typo inside":     {"{ not: { facts: a } }", "requires.not.facts: \"facts\" is not a key this build reads"},
		"beside another":    {"{ not: { fact: a }, fact: b }", "this predicate says both `not` and `fact`"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := dungeonspec.Load([]byte(optionsOnGoblin("          - { id: hi, say: \"Hi.\", requires: " +
				tc.requires + ", check: [{ ability: persuasion, dc: 10 }] }\n")))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.says)
		})
	}
}

// TestNotQuotesBack: a refusal that quotes a `not` back spells it as the
// file does.
func TestNotQuotesBack(t *testing.T) {
	p := dungeonspec.PredicateSpec{Not: &dungeonspec.PredicateSpec{Fact: "caught-in-kitchen"}}
	require.Equal(t, "not", p.Form())
	require.Equal(t, "{ not: { fact: caught-in-kitchen } }", p.String())
}
