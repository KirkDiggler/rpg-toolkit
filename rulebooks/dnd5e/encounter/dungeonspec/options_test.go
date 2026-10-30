// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// options_test.go is the authoring surface of the one social verb
// (rpg-project#569, the mad king use case §2, rulings R1–R3 and R9): what an
// author writes on a creature or a prop for a character to SAY, the check that
// line rolls, what it waits on, and what is answered on either verdict — and
// what the compiler hands the host.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// The front room goblin, authored with two options: the threat and the
// appeal that were its two priced verbs, with the tables that were its four
// social keys now living on the option they answer.
func TestTheFrontRoomGoblinCompilesWithTwoOptions(t *testing.T) {
	compiled := loadContent(t, "testdata/world-builder-v4-front-room.yaml")

	goblin := monsterNamed(t, compiled, "front-goblin")
	require.Equal(t, []encounter.Option{
		{
			ID:    "lean-on-it",
			Say:   "Where's the cellar? Tell me or you're next.",
			Check: []encounter.CheckApproach{{Ability: "intimidation", DC: 12}},
			Success: []encounter.Answer{
				{Weight: 70, Say: "Fine! FINE. The cellar door is behind the barrels. Just don't.", Fact: "goblin-cowed"},
				{Weight: 30, Say: "Boss! BOSS!", Flee: true},
			},
			Failure: []encounter.Answer{{Weight: 1, Say: "Big talk, for someone standing in my doorway."}},
		},
		{
			ID:      "talk-it-round",
			Say:     "We're not here for you. Where did the bandits go?",
			Check:   []encounter.CheckApproach{{Ability: "persuasion", DC: 10}},
			Success: []encounter.Answer{{Weight: 1, Say: "Bandits took the cellar. Go left at the rope, and mind the third step."}},
			Failure: []encounter.Answer{{
				Weight: 100, Say: "Cellar's empty, friend. Nothing down there but rats. Straight on through.",
				Fact: "cellar-is-clear",
			}},
		},
	}, goblin.Options, "both lines, in the order the author wrote them, every omitted weight resolved to 1")

	plain := monsterNamed(t, compiled, "front-goblin-2")
	require.Nil(t, plain.Options, "a goblin nobody wrote a line for carries none, and nil is how it says so")
}

// optionsOnGoblin is a v4 room whose first goblin carries the given options
// block, and whose table can be picked up.
func optionsOnGoblin(options string) string {
	return v4With("", "    propBindings:\n      table: { holdable: true }\n"+
		"    monsterBindings:\n      goblin-1:\n        options:\n"+options)
}

// A line the author can write for a creature, with every key.
const gossipOption = `          - id: gossip
            say: "Busy night?"
            requires: { fact: feast-tonight }
            check: [{ ability: persuasion, dc: 10 }]
            success: [{ say: "Feast for the whole court.", fact: king-burned-village }]
            failure: [{ say: "Out of my kitchen." }]
`

// `requires` compiles to the composition's own trigger: `fact` to the
// predicate's fact, `holds` to the read-only holds trigger.
func TestRequiresCompilesToTheTriggerTheEngineReads(t *testing.T) {
	compiled := loadSource(t, optionsOnGoblin(gossipOption+`          - id: bribe
            say: "Ten gold says you never saw us."
            requires: { holds: table }
            check: [{ ability: persuasion, dc: 8 }]
            success: [{ say: "Saw who?" }]
`))
	goblin := monsterNamed(t, compiled, "goblin-1")
	require.Equal(t, encounter.TriggerFact{Fact: "feast-tonight"}, optionNamed(t, goblin.Options, "gossip").Requires)
	bribe := optionNamed(t, goblin.Options, "bribe")
	require.Equal(t, encounter.TriggerHolds{Prop: "table"}, bribe.Requires)
	require.Nil(t, bribe.Failure, "no failure rows written means none")
}

// optionNamed is the compiled option one id names, required to exist so a
// renamed line fails as a missing line rather than as an index out of range.
func optionNamed(t *testing.T, options []encounter.Option, id string) encounter.Option {
	t.Helper()
	for _, o := range options {
		if o.ID == id {
			return o
		}
	}
	require.FailNowf(t, "no such option", "%q is not among the compiled options", id)

	return encounter.Option{}
}

// A prop binding's options compile onto the placed footprint the host builds
// the field from.
func TestAPropsOptionsCompileOntoItsPlacement(t *testing.T) {
	compiled := loadSource(t, v4With("", `    propBindings:
      table:
        options:
          - id: examine
            say: "Examine the table."
            check: [{ ability: investigation, dc: 12 }]
            success: [{ say: "Scratches. Somebody dragged it.", fact: table-moved }]
`))
	var table *encounter.PlacedPropInput
	for i := range compiled.Field.Placed {
		if compiled.Field.Placed[i].ID == "table" {
			table = &compiled.Field.Placed[i]
		}
	}
	require.NotNil(t, table, "the declared table is placed")
	require.Equal(t, []encounter.Option{{
		ID: "examine", Say: "Examine the table.",
		Check:   []encounter.CheckApproach{{Ability: "investigation", DC: 12}},
		Success: []encounter.Answer{{Weight: 1, Say: "Scratches. Somebody dragged it.", Fact: "table-moved"}},
	}}, table.Options)
}

// Every refusal an option earns, at the v4 path, in the exact sentence the
// World Builder draws on the row.
func TestAnOptionIsRefusedByName(t *testing.T) {
	const at = "room.room.monsterBindings.goblin-1.options"
	tests := []struct {
		name    string
		options string
		path    string
		message string
	}{
		{
			name:    "no id",
			options: "          - { say: \"Hi.\", check: [{ ability: persuasion, dc: 10 }] }\n",
			path:    at + "[0].id",
			message: "the option has no id",
		},
		{
			name:    "no line",
			options: "          - { id: hi, check: [{ ability: persuasion, dc: 10 }] }\n",
			path:    at + "[0].say",
			message: "the option does not say what the character says",
		},
		{
			name: "two options under one id",
			options: "          - { id: hi, say: \"Hi.\", check: [{ ability: persuasion, dc: 10 }] }\n" +
				"          - { id: hi, say: \"Hello.\", check: [{ ability: persuasion, dc: 10 }] }\n",
			path:    at + "[1].id",
			message: `option "hi" is already declared at options[0]`,
		},
		{
			name:    "no check",
			options: "          - { id: hi, say: \"Hi.\" }\n",
			path:    at + "[0].check",
			message: "this option needs at least one way to make its check — an ability and a DC",
		},
		{
			name:    "an empty check",
			options: "          - { id: hi, say: \"Hi.\", check: [] }\n",
			path:    at + "[0].check",
			message: "this option needs at least one way to make its check — an ability and a DC",
		},
		{
			name: "a requirement that is neither form",
			options: "          - { id: hi, say: \"Hi.\", requires: { round: 2 }, " +
				"check: [{ ability: persuasion, dc: 10 }] }\n",
			path: at + "[0].requires",
			message: "an option requires { fact: <id> }, { holds: <prop id> }, or { not: } one of them, " +
				"and `round` is none of them",
		},
		{
			name: "holding a prop this room does not place",
			options: "          - { id: hi, say: \"Hi.\", requires: { holds: lantern }, " +
				"check: [{ ability: persuasion, dc: 10 }] }\n",
			path:    at + "[0].requires.holds",
			message: `"lantern" is not a prop in this dungeon, so nobody can hold it`,
		},
		{
			name: "an empty verdict",
			options: "          - { id: hi, say: \"Hi.\", check: [{ ability: persuasion, dc: 10 }], " +
				"success: [] }\n",
			path:    at + "[0].success",
			message: "`success` lists nothing — omit it, or write what is answered",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, optionsOnGoblin(test.options)), test.path, test.message)
		})
	}

	t.Run("holding a prop nobody can pick up", func(t *testing.T) {
		errs := refusals(t, v4With("", "    monsterBindings:\n      goblin-1:\n        options:\n"+
			"          - { id: hi, say: \"Hi.\", requires: { holds: table }, "+
			"check: [{ ability: persuasion, dc: 10 }] }\n"))
		requireExactDefect(t, errs, at+"[0].requires.holds",
			`"table" is a prop nobody can pick up, so nobody can hold it`)
	})
}

// The answer rows on an option are the table's rows, judged by the same
// grammar: one word, a weight that can be rolled, and only the words a
// verdict may answer with.
func TestAnOptionsRowsAreJudgedByTheTableGrammar(t *testing.T) {
	const at = "room.room.monsterBindings.goblin-1.options[0]"

	t.Run("two words in one row", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, optionsOnGoblin(
			"          - { id: hi, say: \"Hi.\", check: [{ ability: persuasion, dc: 10 }], "+
				"success: [{ fact: cowed, flee: {} }] }\n")),
			at+".success[0]", "an entry does one thing")
	})

	t.Run("a time word on a verdict", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, optionsOnGoblin(
			"          - { id: hi, say: \"Hi.\", check: [{ ability: persuasion, dc: 10 }], "+
				"failure: [{ hold: {} }] }\n")),
			at+".failure[0].hold", "`hold` is what a creature does with time, and `failure` is an outcome")
	})

	t.Run("a condition under a verdict", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, optionsOnGoblin(
			"          - { id: hi, say: \"Hi.\", check: [{ ability: persuasion, dc: 10 }], "+
				"success: [{ say: \"ok\", when: { enemy: seen } }] }\n")),
			at+".success[0].when", "`success` is already the condition")
	})
}

// A prop cannot run, so `flee` on a prop's option is refused by name.
func TestAPropsOptionCannotFlee(t *testing.T) {
	errs := refusals(t, v4With("", `    propBindings:
      table:
        options:
          - { id: kick, say: "Kick it.", check: [{ ability: athletics, dc: 10 }], success: [{ flee: {} }] }
`))
	requireExactDefect(t, errs, "room.room.propBindings.table.options[0].success[0].flee",
		"a prop cannot run: `flee` is a creature's answer")
}

// `holds` is a read of what the SPEAKER carries, so only an option may
// require it. An ending, an arrival or an until has no speaker, and each is
// refused by name rather than as an unknown form.
func TestHoldsIsOnlyAnOptionsRequirement(t *testing.T) {
	const sentence = "`holds` is only something an option requires: it reads what the speaker carries, " +
		"and nothing here speaks"

	t.Run("an ending", func(t *testing.T) {
		requireExactDefect(t, refusals(t, v4With("endings:\n  - { id: rich, when: { holds: table } }", "")),
			"endings[0].when.holds", sentence)
	})

	t.Run("an arrival", func(t *testing.T) {
		requireExactDefect(t, refusals(t, v4With("",
			"    monsterBindings:\n      goblin-1:\n        arrives: { holds: table }")),
			"room.room.monsterBindings.goblin-1.arrives.holds", sentence)
	})

	t.Run("an until", func(t *testing.T) {
		requireExactDefect(t, refusals(t, v4With(
			"factions:\n  - { id: goblins }\ndispositions:\n"+
				"  - { between: [goblins, party], stance: hostile, until: { holds: table } }",
			"    monsterBindings:\n      goblin-1: { faction: goblins }")),
			"dispositions[0].until.holds", sentence)
	})
}

// The four retired table keys are refused by name with where they went —
// never as a bare unknown trigger, which would tell an author with an old
// file that the word never existed.
func TestTheRetiredSocialKeysPointAtOptions(t *testing.T) {
	for _, key := range []string{"intimidated", "intimidate_failed", "persuaded", "persuade_failed"} {
		t.Run(key, func(t *testing.T) {
			errs := refusals(t, v4With("", "    monsterBindings:\n      goblin-1:\n        on:\n          "+
				key+": [{ say: \"hm\" }]"))
			requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.on."+key,
				"`"+key+"` is no longer a table key: author an option")
		})
	}
}

// The two retired binding keys are refused by name too, in both dialects.
func TestTheRetiredCheckKeysPointAtOptions(t *testing.T) {
	t.Run("v4", func(t *testing.T) {
		errs := refusals(t, v4With("",
			"    monsterBindings:\n      goblin-1:\n        intimidate: [{ ability: intimidation, dc: 12 }]"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.intimidate",
			"`intimidate` is no longer a key: author an option whose check rolls it")
	})

	t.Run("v2", func(t *testing.T) {
		requireDefect(t, decodeDefects(t, edited(t, chiefLine,
			`  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, `+
				`persuade: [{ ability: persuasion, dc: 10 }] }`)),
			"place[0].persuade", "`persuade` is no longer a key: author an option whose check rolls it")
	})
}

// # The v2 dialect

// talkativeChief is the chief with one option of each kind, in v2.
const talkativeChief = `  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, ` +
	`options: [ { id: cow, say: "Kneel.", check: [{ ability: intimidation, dc: 12 }, { ability: str, dc: 15 }], ` +
	`success: [ { weight: 70, say: "Fine, fine!", fact: sergeant-cowed }, { weight: 30, say: "BOSS!", flee: {} } ], ` +
	`failure: [ { say: "Big talk." } ] } ] }`

// A v2 placement carries options exactly as a v4 binding does.
func TestAV2PlacementCarriesOptions(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(edited(t, chiefLine, talkativeChief)))
	require.NoError(t, err)

	chief := compiled.Monsters[0]
	require.Equal(t, "chief", chief.ID)
	require.Equal(t, []encounter.Option{{
		ID: "cow", Say: "Kneel.",
		Check: []encounter.CheckApproach{{Ability: "intimidation", DC: 12}, {Ability: "str", DC: 15}},
		Success: []encounter.Answer{
			{Weight: 70, Say: "Fine, fine!", Fact: "sergeant-cowed"},
			{Weight: 30, Say: "BOSS!", Flee: true},
		},
		Failure: []encounter.Answer{{Weight: 1, Say: "Big talk."}},
	}}, chief.Options)
	require.Nil(t, chief.Table, "the line's answers live on the line, not on a table")
}

// A v2 prop carries options onto the cell prop the host builds.
func TestAV2PropCarriesOptions(t *testing.T) {
	const altar = `  - { id: altar, ref: "dnd5e:props:barrel", at: [11,4], blocks_movement: true, blocks_los: false, ` +
		`options: [ { id: examine, say: "Examine it.", check: [{ ability: religion, dc: 12 }], ` +
		`success: [ { say: "The Pale One's.", fact: king-is-cultist } ] } ] }`

	compiled, err := dungeonspec.Load([]byte(edited(t, chiefLine, chiefLine+"\n"+altar)))
	require.NoError(t, err)

	var found []encounter.Option
	for _, p := range compiled.Field.Props {
		if p.ID == "altar" {
			found = p.Options
		}
	}
	require.Equal(t, []encounter.Option{{
		ID: "examine", Say: "Examine it.",
		Check:   []encounter.CheckApproach{{Ability: "religion", DC: 12}},
		Success: []encounter.Answer{{Weight: 1, Say: "The Pale One's.", Fact: "king-is-cultist"}},
	}}, found)
}

// A v2 refusal lands at `place[i].options[j]`, by the same grammar.
func TestAV2OptionIsRefusedAtItsPlacementPath(t *testing.T) {
	requireDefect(t, defectsIn(t, edited(t, chiefLine,
		strings.Replace(talkativeChief, `say: "Kneel.", `, "", 1))),
		"place[0].options[0].say", "the option does not say what the character says")
}

// onOption is the chief with one option whose success rows are the body, so
// a scene can name exactly the row it is about.
func onOption(t *testing.T, rows string) string {
	t.Helper()

	return edited(t, chiefLine,
		`  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, `+
			`options: [ { id: hi, say: "Hi.", check: [{ ability: persuasion, dc: 10 }], success: `+rows+` } ] }`)
}

// A word the design NAMES and this build does not land is refused BY NAME on
// an option's row exactly as on a table's — the row decoder is the same one.
func TestADesignedButUnbuiltWordIsRefusedByNameOnAnOption(t *testing.T) {
	for word, want := range map[string]string{
		"alarm":   "designed as the Alarm slice and not built yet",
		"lure":    "designed and not built yet",
		"pretend": "designed as the Insight slice and not built yet",
		"tell":    "not a word: a fact taught to whoever was there is `fact`",
	} {
		t.Run(word, func(t *testing.T) {
			_, err := dungeonspec.Load([]byte(onOption(t, `[ { `+word+`: { toward: bandit-hall } } ]`)))
			require.Error(t, err)
			require.Contains(t, err.Error(), "`"+word+"` is "+want)
		})
	}
}

// A key that is neither an outcome word nor a designed one is a plain
// unknown key at the row's own path.
func TestATypoInAnOptionsRowIsAnUnknownField(t *testing.T) {
	requireDefect(t, decodeDefects(t, onOption(t, `[ { sez: "hi" } ]`)),
		"place[0].options[0].success[0].sez", `"sez" is not a key this build reads`)
}

// A typo on the option itself is an unknown key at the option's path,
// listing the keys an option takes.
func TestATypoOnAnOptionIsAnUnknownField(t *testing.T) {
	requireDefect(t, decodeDefects(t, edited(t, chiefLine,
		`  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, `+
			`options: [ { id: hi, say: "Hi.", chek: [] } ] }`)),
		"place[0].options[0].chek", `"chek" is not a key this build reads`)
}

// A row that does nothing and says nothing, and a weight that can never
// fire, are the table's refusals on an option's row too.
func TestAnOptionsEmptyRowAndZeroWeightAreRefused(t *testing.T) {
	requireDefect(t, defectsIn(t, onOption(t, `[ { weight: 3 } ]`)),
		"place[0].options[0].success[0]", "this entry does nothing and says nothing")
	requireDefect(t, defectsIn(t, onOption(t, `[ { weight: 0, say: "never" } ]`)),
		"place[0].options[0].success[0].weight", "a weight of 0 can never be rolled")
	requireDefect(t, defectsIn(t, onOption(t, `[ { fact: "" } ]`)),
		"place[0].options[0].success[0].fact", "does not say what")
}
