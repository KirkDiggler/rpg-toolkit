// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// interactions_test.go is the authoring surface of the entrance that
// remembers (rpg-project#922, Gate S2 of #769): a root `interactions:` entry,
// an option requiring its state, and an option whose verdict advances it.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// frontGateSource is the root interaction every case here names.
const frontGateSource = `interactions:
  front-gate:
    recipe:
      ref: site:interactions:front-gate
      initial: fresh
      states:
        fresh:
          transitions:
            - { on: guest-failed, to: warned-guest, reason: the guest claim failed }
            - { on: admitted, to: admitted }
        warned-guest:
          transitions:
            - { on: troupe-failed, to: arrested }
            - { on: admitted, to: admitted }
        admitted: { terminal: true }
        arrested: { terminal: true }
`

// gateWith is a v4 room with root and the first goblin carrying options.
func gateWith(root, options string) string {
	return v4With(root, "    propBindings:\n      table: { holdable: true }\n"+
		"    monsterBindings:\n      goblin-1:\n        options:\n"+options)
}

// guestClaim is the guest claim, gated on the gate being fresh and advancing
// it on either verdict.
const guestClaim = `          - id: guest
            say: "We're invited."
            check: [{ ability: persuasion, dc: 11 }]
            requires:
              - { fact: feast-tonight }
              - { interaction: { id: front-gate, in: [fresh] } }
            advance: { interaction: front-gate, success: admitted, failure: guest-failed }
            success: [{ say: "Through you go." }]
            failure: [{ say: "What's going on here?" }]
`

// The root interaction lowers onto the field the host launches, and the
// option's requirement and advance compile to the engine's own shapes.
func TestAnInteractionCompilesOntoTheField(t *testing.T) {
	compiled := loadSource(t, gateWith(frontGateSource, guestClaim))

	require.Len(t, compiled.Field.Interactions, 1)
	gate := compiled.Field.Interactions[0]
	require.Equal(t, encounter.InteractionID("front-gate"), gate.ID)
	require.Equal(t, core.Ref{Module: "site", Type: "interactions", ID: "front-gate"}, gate.Recipe.Ref)
	require.Equal(t, "fresh", gate.Recipe.Initial)
	require.Equal(t, []machine.Transition{
		{On: "guest-failed", To: "warned-guest", Reason: "the guest claim failed"},
		{On: "admitted", To: "admitted"},
	}, gate.Recipe.States["fresh"].Transitions)

	guest := optionNamed(t, monsterNamed(t, compiled, "goblin-1").Options, "guest")
	require.Equal(t, []encounter.Trigger{
		encounter.TriggerFact{Fact: "feast-tonight"},
		encounter.TriggerInteraction{Interaction: "front-gate", In: []string{"fresh"}},
	}, guest.Requires)
	require.Equal(t, &encounter.Advance{Interaction: "front-gate", Success: "admitted", Failure: "guest-failed"}, guest.Advance)
}

// A room with no `interactions:` lowers none.
func TestNoInteractionsLowersNone(t *testing.T) {
	compiled := loadSource(t, optionsOnGoblin(gossipOption))
	require.Nil(t, compiled.Field.Interactions)
}

// A `not` of an interaction is legal on an option's requires.
func TestANotOfAnInteractionIsLegalOnRequires(t *testing.T) {
	compiled := loadSource(t, gateWith(frontGateSource, `          - id: bluff
            say: "Again?"
            requires: { not: { interaction: { id: front-gate, in: [arrested] } } }
            success: [{ say: "Again." }]
`))
	require.Equal(t, []encounter.Trigger{encounter.TriggerNot{Inner: encounter.TriggerInteraction{
		Interaction: "front-gate", In: []string{"arrested"},
	}}}, optionNamed(t, monsterNamed(t, compiled, "goblin-1").Options, "bluff").Requires)
}

// What an option names of an interaction must exist: the interaction, each
// state, and each beat an advance steps — refused at the author's path.
func TestAnOptionNamingWhatTheInteractionLacksIsRefused(t *testing.T) {
	const at = "room.room.monsterBindings.goblin-1.options[0]"
	for name, tc := range map[string]struct{ option, path, says string }{
		"an unknown interaction": {
			`{ id: hi, say: "Hi.", requires: { interaction: { id: nope, in: [fresh] } }, success: [{ say: "Hi." }] }`,
			at + ".requires.interaction.id", `"nope" is not an interaction in this dungeon`,
		},
		"an unknown state": {
			`{ id: hi, say: "Hi.", requires: { interaction: { id: front-gate, in: [fresh, bribed] } }, success: [{ say: "Hi." }] }`,
			at + ".requires.interaction.in[1]", `"bribed" is not a state of interaction "front-gate"`,
		},
		"an advance on a beat nobody hears": {
			`{ id: hi, say: "Hi.", check: [{ ability: persuasion, dc: 10 }], advance: { interaction: front-gate, failure: bribed }, success: [{ say: "Hi." }] }`,
			at + ".advance.failure", `no state of interaction "front-gate" listens for "bribed"`,
		},
		"an advance of an unknown interaction": {
			`{ id: hi, say: "Hi.", advance: { interaction: back-gate, success: admitted }, success: [{ say: "Hi." }] }`,
			at + ".advance.interaction", `"back-gate" is not an interaction in this dungeon`,
		},
		"an advance with no beat": {
			`{ id: hi, say: "Hi.", advance: { interaction: front-gate }, success: [{ say: "Hi." }] }`,
			at + ".advance", "names neither",
		},
		"a failure beat on a plain line": {
			`{ id: hi, say: "Hi.", advance: { interaction: front-gate, failure: guest-failed }, success: [{ say: "Hi." }] }`,
			at + ".advance.failure", "cannot be missed",
		},
	} {
		t.Run(name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, gateWith(frontGateSource, "          - "+tc.option+"\n")), tc.path, tc.says)
		})
	}
}

// An interaction's state is read by an option and never waited for: an
// until, an arrival and an ending's `when` each refuse it by name.
func TestAnInteractionIsRefusedWhereItWouldBeWaitedFor(t *testing.T) {
	const gate = "{ interaction: { id: front-gate, in: [admitted] } }"
	for name, tc := range map[string]struct{ root, path string }{
		"an until": {
			frontGateSource + "factions:\n  - { id: goblins }\ndispositions:\n" +
				"  - { between: [goblins, party], stance: neutral, until: " + gate + " }\n",
			"dispositions[0].until.interaction",
		},
		"an ending's when": {
			frontGateSource + "endings:\n  - { id: in, when: " + gate + ", outcome: success }\n",
			"endings[0].when.interaction",
		},
		"a not on an ending's when": {
			frontGateSource + "endings:\n  - { id: in, when: { not: " + gate + " }, outcome: success }\n",
			"endings[0].when.not.interaction",
		},
	} {
		t.Run(name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, v4With(tc.root, "")), tc.path, "only something an option requires")
		})
	}
}

// The recipe is judged by the engine's own door at its path: a pick, a
// wake, a fact it would read, a missing ref and an unknown member are each
// refused.
func TestAnInteractionRecipeIsJudgedAtItsPath(t *testing.T) {
	recipe := func(body string) string {
		return "interactions:\n  front-gate:\n    recipe:\n" + body
	}
	for name, tc := range map[string]struct{ root, path, says string }{
		"a pick": {recipe(`      ref: site:interactions:front-gate
      initial: fresh
      states:
        fresh: { transitions: [{ on: knock, do: [{ pick: time }] }] }
`), "interactions.front-gate.recipe", "picks nothing"},
		"a wake": {recipe(`      ref: site:interactions:front-gate
      initial: fresh
      states:
        fresh: { transitions: [{ on: knock, do: [{ wake_in: 2 }] }] }
`), "interactions.front-gate.recipe", "nothing wakes an interaction"},
		"a fact": {recipe(`      ref: site:interactions:front-gate
      initial: fresh
      states:
        fresh: { transitions: [{ on: knock, when: { fact: { name: enemy, op: eq, value: true } } }] }
`), "interactions.front-gate.recipe", "enemy"},
		"no ref": {recipe(`      initial: fresh
      states:
        fresh: {}
`), "interactions.front-gate.recipe", ""},
		"an unknown member": {recipe(`      ref: site:interactions:front-gate
      initial: fresh
      states:
        fresh: { transitoins: [] }
`), "interactions.front-gate.recipe", "transitoins"},
	} {
		t.Run(name, func(t *testing.T) {
			errs := refusals(t, v4With(tc.root, ""))
			if tc.says == "" {
				requireRefusedAt(t, errs, tc.path+".ref")
				return
			}
			requireRefusedAt(t, errs, tc.path, tc.says)
		})
	}
	_, err := dungeonspec.Load([]byte(v4With(recipe("      ref: site:interactions:front-gate\n      initial: fresh\n      states: { fresh: {} }\n    guards: [captain]\n"), "")))
	require.ErrorContains(t, err, "guards", "an interaction's own keys are its recipe and nothing else")
}

// withApproach is frontGateSource with an `approach:` body under the
// interaction, beside its recipe.
func withApproach(body string) string {
	return frontGateSource + "    approach: " + body + "\n"
}

// An interaction's approach distance lowers onto its definition
// (rpg-project#923), and one with none lowers none.
func TestAnInteractionApproachLowersOntoItsDefinition(t *testing.T) {
	compiled := loadSource(t, gateWith(withApproach("{ within: 2 }"), guestClaim))
	require.Len(t, compiled.Field.Interactions, 1)
	require.Equal(t, &encounter.InteractionApproach{Within: 2}, compiled.Field.Interactions[0].Approach)

	plain := loadSource(t, gateWith(frontGateSource, guestClaim))
	require.Nil(t, plain.Field.Interactions[0].Approach, "no approach, nobody is hailed")
}

// A malformed approach is refused at compile, fail closed: no distance, a
// zero or negative one, and an unknown key.
func TestAMalformedApproachIsRefused(t *testing.T) {
	for name, tc := range map[string]struct{ body, path, says string }{
		"no distance": {"{}", "interactions.front-gate.approach.within", "at least 1"},
		"zero":        {"{ within: 0 }", "interactions.front-gate.approach.within", "at least 1"},
		"negative":    {"{ within: -2 }", "interactions.front-gate.approach.within", "at least 1"},
	} {
		t.Run(name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, gateWith(withApproach(tc.body), guestClaim)), tc.path, tc.says)
		})
	}
	_, err := dungeonspec.Load([]byte(gateWith(withApproach("{ within: 2, shout: 9 }"), guestClaim)))
	require.ErrorContains(t, err, "shout", "an approach's one key is its distance")
}
