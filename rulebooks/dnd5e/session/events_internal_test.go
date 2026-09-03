// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// testKindOf is decodeBeat's own Kind half, isolated for the tests below
// that pin kindFor's mapping without caring what bodyFor makes of the rest
// of the payload (rpg-toolkit#941 split decodeBeat's old single-return
// kindOf into kindFor + bodyFor; these pins moved with it).
func testKindOf(payload string) EventKind {
	kind, _ := decodeBeat([]byte(payload))
	return kind
}

// TestAnUnrecognisedBeatStaysUnknown pins the mapper's default arm.
//
// Pinned HERE, against kindFor directly, rather than through a verb — because no
// verb can produce it. Every beat the composition emits today has a case, which
// is the point of rpg-toolkit#1038; the arm exists for the beat a LATER
// composition adds, and there is no honest way to drive that from this side of
// the seam without faking the composition itself.
//
// It is load-bearing rather than defensive. Delivering an uninterpretable beat
// keeps a client's sequence gapless, so it can tell "I do not understand this"
// from "I missed something" — dropping it would manufacture a hole and send the
// client into a resync it never needed. That is also what lets a newer
// composition ship a beat this version has never heard of without older clients
// losing their place.
func TestAnUnrecognisedBeatStaysUnknown(t *testing.T) {
	require.Equal(t, EventUnknown, testKindOf(`{"beat":"transmogrified"}`),
		"a beat this version does not know is delivered, not dropped")
	require.Equal(t, EventUnknown, testKindOf(`{"actor":"alice"}`),
		"and so is a beat with no name at all")
	require.Equal(t, EventUnknown, testKindOf(`not json`),
		"including a payload this package cannot even parse")
}

// TestTheOutcomeBeatsAreTheCompositionsOwnStrings guards the coupling from the
// other side.
//
// The seam-level pins drive real scenes and are the evidence that matters —
// attackevents_test.go for the two swings, death_test.go for the third. This
// asks the narrower question those cannot: does kindFor's literal still equal
// the composition's own constant?
//
// It is BUILT FROM the composition's constants rather than from strings typed
// twice, and that is the whole strength of it. kindFor matches on literals, so a
// rename upstream degrades every event of that kind to EventUnknown with
// nothing failing — the silent failure its own doc warns about. A test that
// also spelled the literal out would keep passing through exactly that change.
// Reading the constant means the rename lands here as a red test instead of in
// a game as a table that has stopped narrating.
func TestTheOutcomeBeatsAreTheCompositionsOwnStrings(t *testing.T) {
	cases := []struct {
		kind encounter.OutcomeKind
		want EventKind
	}{
		{encounter.OutcomeStruck, EventStruck},
		{encounter.OutcomeMissed, EventMissed},
		// The third outcome beat. Unlike the two above, no caller can push it
		// in — the composition writes it itself when it notices a body
		// (rpg-toolkit#1077), which is why the beat carries a member rather
		// than an actor and targets.
		{encounter.OutcomeDown, EventDowned},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			require.Equal(t, tc.want,
				testKindOf(`{"beat":"`+string(tc.kind)+`","member":"goblin"}`))
		})
	}
}

// TestTheSeamSaysDownedWhereTheCompositionSaysDown makes Kirk's ruling
// mechanical (rpg-toolkit#1084).
//
// The two strings are DIFFERENT ON PURPOSE and neither may drift toward the
// other, which is why both halves and the gap between them are asserted rather
// than just the one this package owns.
//
//   - The seam publishes "downed", because a bare "down" also reads as PRONE —
//     a posture the rulebook tracks and this package never gates on. This is
//     the wire value rpg-api adopts, so it is contract, not cosmetics, and
//     asserting the CONSTANT alone would not have noticed it changing.
//   - The composition keeps "down", because that kind is persisted in every
//     stored world. Renaming it there is a migration; translating here is a
//     line in kindFor.
//
// The third assertion is the one that earns its keep. Both single-value rows
// pass happily if somebody aligns the two — that is the tidy-looking change
// this asymmetry invites, and it silently either breaks every stored world or
// hands clients back the ambiguous word.
func TestTheSeamSaysDownedWhereTheCompositionSaysDown(t *testing.T) {
	require.Equal(t, EventKind("downed"), EventDowned,
		"the wire value a client branches on, and rpg-api's vocabulary")
	require.Equal(t, encounter.OutcomeKind("down"), encounter.OutcomeDown,
		"the composition's persisted kind, deliberately left alone")
	require.NotEqual(t, string(EventDowned), string(encounter.OutcomeDown),
		"these two must not be aligned: see kindFor, and rpg-toolkit#1084")
}

// TestBodyForRefusesAMissingRequiredField pins the fix for Copilot's finding
// on PR #1174: json.Unmarshal does not fail when a struct's fields are
// simply absent from the payload, so bodyFor (and structBody, its own arm
// for struck/missed) must check for the required fields explicitly rather
// than trusting a successful Unmarshal alone — an incomplete beat must
// leave Body nil, not populate it with zero-valued fields that read as
// real answers ("" for a member id, an empty AttackRef).
func TestBodyForRefusesAMissingRequiredField(t *testing.T) {
	cases := []struct {
		name string
		kind EventKind
		json string
	}{
		{"moved with no member", EventMoved, `{"beat":"moved","position":{"x":1,"y":2}}`},
		{"turn-ended with no next", EventTurnEnded, `{"beat":"turn-ended","member":"alice"}`},
		{"turn-ended with no member", EventTurnEnded, `{"beat":"turn-ended","next":"bob"}`},
		{"bubble-formed with an empty order", EventFightStarted, `{"beat":"bubble-formed","order":[]}`},
		{"bubble-dissolved with no cause", EventFightEnded, `{"beat":"bubble-dissolved"}`},
		{"down with no member", EventDowned, `{"beat":"down"}`},
		{"joined with no member", EventJoined, `{"beat":"joined"}`},
		{"exited with no member", EventExited, `{"beat":"exited"}`},
		{"struck with no actor", EventStruck, `{"beat":"struck","targets":["bob"],"attack":{"ref":"longsword"}}`},
		{"struck with no targets", EventStruck, `{"beat":"struck","actor":"alice","attack":{"ref":"longsword"}}`},
		{"struck with two targets", EventStruck, `{"beat":"struck","actor":"alice","targets":["bob","carol"],"attack":{"ref":"longsword"}}`},
		{"struck with no attack ref", EventStruck, `{"beat":"struck","actor":"alice","targets":["bob"]}`},
		{"missed with no actor", EventMissed, `{"beat":"missed","targets":["bob"],"attack":{"ref":"longsword"}}`},
		{"ended with no ending", EventEnded, `{"beat":"ended"}`},
		{"door with no door", EventDoor, `{"beat":"door","state":"open"}`},
		{"door with no state", EventDoor, `{"beat":"door","door":"gate"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(tc.json))
			require.Equal(t, tc.kind, kind, "the KIND is still correctly identified from the declared beat")
			require.Nil(t, body, "but the body is nil rather than populated with zero-valued fields")
		})
	}
}

// TestBodyForAcceptsACompleteBeat is the companion pin: every field the
// case above found missing, present, produces a typed body — the stricter
// check must not have overreached into refusing valid beats.
func TestBodyForAcceptsACompleteBeat(t *testing.T) {
	kind, body := decodeBeat([]byte(
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8,` +
			`"critical":false,"attack":{"ref":"longsword","name":"Longsword","damage_type":"slashing"}}`))
	require.Equal(t, EventStruck, kind)
	require.Equal(t, StruckBody{
		Attacker: "alice", Target: "bob", Roll: 15, Total: 20, Against: 12, Damage: 8,
		Attack: AttackRef{Ref: "longsword", Name: "Longsword", DamageType: DamageSlashing},
	}, body)
}

// TestStruckBodyDecodesReplayDetail pins the second half of the projection:
// the exact primitive payload session authored becomes its own exported body,
// with the sourced roll graph, a present zero modifier, and the supplied
// ordering intact.
func TestStruckBodyDecodesReplayDetail(t *testing.T) {
	kind, body := decodeBeat([]byte(
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8,` +
			`"critical":false,"attack":{"ref":"longsword","name":"Longsword","damage_type":"slashing"},` +
			`"damage_components":[` +
			`{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},` +
			`"dice":{"notation":"d8","die_size":8,"original_rolls":[2],"final_rolls":[4],` +
			`"rerolls":[{"die_index":0,"before":2,"after":4,"source":{"ref":"dnd5e:conditions:fighting_style_great_weapon_fighting","name":"Great Weapon Fighting"}}],"subtotal":4},"modifier":0},` +
			`"damage_type":"slashing"},` +
			`{"source":"monster_trait","roll":{"source":{"ref":"dnd5e:monster_traits:immunity","name":"Immunity"}},"damage_type":"slashing","multiplier":0}],` +
			`"advantage_sources":[{"source_ref":"dnd5e:conditions:hidden","source_id":"alice"}],` +
			`"disadvantage_sources":[{"source_ref":"dnd5e:conditions:dodging","source_id":"bob"}]}`))
	require.Equal(t, EventStruck, kind)

	got, ok := body.(StruckBody)
	require.True(t, ok, "rich struck payload produces StruckBody, got %T", body)
	require.Len(t, got.DamageComponents, 2)
	zero := 0.0
	zeroMod := 0
	require.Equal(t, []DamageComponent{
		{
			Source: "weapon",
			Roll: RollComponent{
				Source: RollSource{Ref: "dnd5e:weapons:longsword", Name: "Longsword"},
				Dice: &DiceTrace{
					Notation: "d8", DieSize: 8,
					OriginalRolls: []int{2}, FinalRolls: []int{4}, Subtotal: 4,
					Rerolls: []DiceReroll{{
						DieIndex: 0, Before: 2, After: 4,
						Source: RollSource{
							Ref:  "dnd5e:conditions:fighting_style_great_weapon_fighting",
							Name: "Great Weapon Fighting",
						},
					}},
				},
				Modifier: &zeroMod,
			},
			DamageType: DamageSlashing,
		},
		{
			Source:     "monster_trait",
			Roll:       RollComponent{Source: RollSource{Ref: "dnd5e:monster_traits:immunity", Name: "Immunity"}},
			DamageType: DamageSlashing, Multiplier: &zero,
		},
	}, got.DamageComponents)
	require.NotNil(t, got.DamageComponents[0].Roll.Modifier)
	require.Zero(t, *got.DamageComponents[0].Roll.Modifier)
	require.NotNil(t, got.DamageComponents[1].Multiplier)
	require.Zero(t, *got.DamageComponents[1].Multiplier)
	require.Equal(t,
		[]AttackModifierSource{{SourceRef: "dnd5e:conditions:hidden", SourceID: "alice"}},
		got.AdvantageSources)
	require.Equal(t,
		[]AttackModifierSource{{SourceRef: "dnd5e:conditions:dodging", SourceID: "bob"}},
		got.DisadvantageSources)
}

// TestStruckBodyDecodesLegacyDamageComponents pins the legacy read fallback:
// a pre-trace struck payload maps its scalar facts into the legacy fields, one
// for one, and the decoder never fabricates a roll graph from them.
func TestStruckBodyDecodesLegacyDamageComponents(t *testing.T) {
	kind, body := decodeBeat([]byte(
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8,` +
			`"critical":false,"attack":{"ref":"longsword","name":"Longsword","damage_type":"slashing"},` +
			`"damage_components":[` +
			`{"source":"weapon","source_ref":"dnd5e:weapons:longsword","dice":"1d8","final_rolls":[5],"flat_bonus":0,"damage_type":"slashing"},` +
			`{"source":"monster_trait","flat_bonus":0,"damage_type":"slashing","multiplier":0}]}`))
	require.Equal(t, EventStruck, kind)

	got, ok := body.(StruckBody)
	require.True(t, ok, "legacy struck payload produces StruckBody, got %T", body)
	zero := 0.0
	require.Equal(t, []DamageComponent{
		{
			Source: "weapon", SourceRef: "dnd5e:weapons:longsword", Dice: "1d8",
			FinalRolls: []int{5}, DamageType: DamageSlashing,
		},
		{
			Source: "monster_trait", DamageType: DamageSlashing, Multiplier: &zero,
		},
	}, got.DamageComponents)
	require.Nil(t, got.DamageComponents[0].Roll.Dice,
		"the legacy scalars are never refabricated into a trace")
	require.Nil(t, got.DamageComponents[0].Roll.Modifier)
	require.Empty(t, got.DamageComponents[0].Roll.Source.Ref)
}

// TestStruckBodyDecodesLegacyMultiplierOnlyComponent pins the legacy read
// fallback for the one pre-trace shape that carries neither a scalar roll fact
// nor a flat bonus: a multiplier-only trait component. Its present non-null
// multiplier is the legacy marker — the component maps into the legacy fields
// alone, with the multiplier carried as a present zero and no roll graph
// fabricated from it.
func TestStruckBodyDecodesLegacyMultiplierOnlyComponent(t *testing.T) {
	kind, body := decodeBeat([]byte(
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8,` +
			`"critical":false,"attack":{"ref":"longsword","name":"Longsword","damage_type":"slashing"},` +
			`"damage_components":[` +
			`{"source":"monster_trait","damage_type":"slashing","multiplier":0}]}`))
	require.Equal(t, EventStruck, kind)

	got, ok := body.(StruckBody)
	require.True(t, ok, "legacy multiplier-only payload produces StruckBody, got %T", body)
	require.Len(t, got.DamageComponents, 1)
	zero := 0.0
	require.Equal(t, DamageComponent{
		Source: "monster_trait", DamageType: DamageSlashing, Multiplier: &zero,
	}, got.DamageComponents[0], "legacy fields only: no source_ref, dice, final_rolls, or flat bonus")
	require.NotNil(t, got.DamageComponents[0].Multiplier)
	require.Zero(t, *got.DamageComponents[0].Multiplier)
	require.Nil(t, got.DamageComponents[0].Roll.Dice,
		"the legacy scalars are never refabricated into a trace")
	require.Nil(t, got.DamageComponents[0].Roll.Modifier)
	require.Empty(t, got.DamageComponents[0].Roll.Source.Ref)
}

// TestStruckBodyDecodesMultiplierOnlyNewRoll pins the new representation's
// multiplier-only shape: a sourced identity beside a multiplier, with neither
// dice nor a modifier — exactly the trait components the rulebook's own
// producers emit. The identity is validated (canonical ref, name) even though
// there is no trace to replay.
func TestStruckBodyDecodesMultiplierOnlyNewRoll(t *testing.T) {
	kind, body := decodeBeat([]byte(
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8,` +
			`"critical":false,"attack":{"ref":"longsword","name":"Longsword","damage_type":"slashing"},` +
			`"damage_components":[` +
			`{"source":"monster_trait","roll":{"source":{"ref":"dnd5e:monster_traits:immunity","name":"Immunity"}},"damage_type":"slashing","multiplier":0}]}`))
	require.Equal(t, EventStruck, kind)

	got, ok := body.(StruckBody)
	require.True(t, ok, "multiplier-only new payload produces StruckBody, got %T", body)
	require.Len(t, got.DamageComponents, 1)
	zero := 0.0
	require.Equal(t, DamageComponent{
		Source:     "monster_trait",
		Roll:       RollComponent{Source: RollSource{Ref: "dnd5e:monster_traits:immunity", Name: "Immunity"}},
		DamageType: DamageSlashing, Multiplier: &zero,
	}, got.DamageComponents[0])
	require.Nil(t, got.DamageComponents[0].Roll.Dice)
	require.Nil(t, got.DamageComponents[0].Roll.Modifier)
	require.NotNil(t, got.DamageComponents[0].Multiplier)
	require.Zero(t, *got.DamageComponents[0].Multiplier)
}

// TestAMissedBeatCarriesNoDamageComponents pins the miss contract at the
// decoder: damage_components belongs to the STRUCK shape alone, so its mere
// presence on a missed beat — well-formed or malformed — is a shape this
// decoder does not recognise. Both retain the KNOWN EventMissed kind with a
// nil body; neither is decoded-and-dropped.
func TestAMissedBeatCarriesNoDamageComponents(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{
			name: "well-formed components on a miss",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":{"notation":"d8","die_size":8,"original_rolls":[4],"final_rolls":[4],"subtotal":4}},"damage_type":"slashing"}]}`,
		},
		{
			name: "malformed components on a miss",
			json: `"damage_components":[{"note":"x"}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(
				`{"beat":"missed","actor":"alice","targets":["bob"],"roll":2,"total":7,"against":12,` +
					`"attack":{"ref":"longsword","name":"Longsword","damage_type":"slashing"},` + tc.json))
			require.Equal(t, EventMissed, kind, "the KNOWN kind is never demoted")
			require.Nil(t, body, "a miss carries no damage components, however they arrive")
		})
	}
}

// TestStrictJSONObjectRefusesTrailingContent pins the walker's own boundary:
// one JSON object and nothing after it.
func TestStrictJSONObjectRefusesTrailingContent(t *testing.T) {
	_, ok := strictJSONObject([]byte(`{"beat":"struck"}`))
	require.True(t, ok, "the well-formed object itself still parses")
	_, ok = strictJSONObject([]byte(`{"beat":"struck"} 42`))
	require.False(t, ok, "a trailing JSON value is trailing content")
	_, ok = strictJSONObject([]byte(`{"beat":"struck"} garbage`))
	require.False(t, ok, "and so is trailing garbage")
	_, ok = strictJSONObject([]byte(`{"beat":"struck"} {"beat":"moved"}`))
	require.False(t, ok, "a trailing object is still trailing content")
}

// TestKnownBeatPayloadsRefuseNullResultAndRoll pins the two forbidden-null
// shapes the decoders own directly: a null result on an activation beat and a
// null roll on a damage component. Both keep their kind with a nil body.
func TestKnownBeatPayloadsRefuseNullResultAndRoll(t *testing.T) {
	kind, body := decodeBeat([]byte(`{"beat":"activation-result","actor":"alice","result":null}`))
	require.Equal(t, EventActivationResult, kind)
	require.Nil(t, body)

	kind, body = decodeBeat([]byte(
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8,` +
			`"critical":false,"attack":{"ref":"longsword","name":"Longsword","damage_type":"slashing"},` +
			`"damage_components":[{"source":"weapon","roll":null,"damage_type":"slashing"}]}`))
	require.Equal(t, EventStruck, kind)
	require.Nil(t, body)
}

// TestOuterActivationDuplicatesKeepTheirKind pins the outer duplicate-key
// refusal on the activation-result beat: a repeated actor — or a repeated
// beat, which the tolerant kind peek resolves by last value — is ambiguous
// and refused while the KNOWN kind is retained.
func TestOuterActivationDuplicatesKeepTheirKind(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{
			name: "duplicate actor",
			json: `"actor":"alice","actor":"bob","result":{"kind":"capacity-granted","target":"alice","description":"30ft movement"}`,
		},
		{
			name: "duplicate beat",
			json: `"beat":"activation-result","actor":"alice","result":{"kind":"capacity-granted","target":"alice","description":"30ft movement"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(`{"beat":"activation-result",` + tc.json + `}`))
			require.Equal(t, EventActivationResult, kind)
			require.Nil(t, body)
		})
	}
}

// TestStruckBodyRejectsAmbiguousAndCorruptRollDetail pins the strict decoder's
// refusals: mixed representations, duplicate keys at every depth, forbidden
// nulls, unknown keys inside known roll bodies, and rolls whose arithmetic
// does not replay. Every refusal keeps the KNOWN kind with a nil body.
func TestStruckBodyRejectsAmbiguousAndCorruptRollDetail(t *testing.T) {
	base := `"actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8,"critical":false,` +
		`"attack":{"ref":"longsword","name":"Longsword","damage_type":"slashing"}`
	cases := []struct {
		name string
		json string
		kind EventKind
	}{
		{
			name: "component carries both representations",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"modifier":0},"source_ref":"dnd5e:weapons:longsword","damage_type":"slashing"}]`,
		},
		{
			name: "roll trace alongside legacy final rolls",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"}},"final_rolls":[5],"damage_type":"slashing"}]`,
		},
		{
			name: "component with neither representation",
			json: `"damage_components":[{"source":"weapon","damage_type":"slashing"}]`,
		},
		{
			name: "new source-only roll without a multiplier",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"}},"damage_type":"slashing"}]`,
		},
		{
			name: "multiplier-only roll missing its ref",
			json: `"damage_components":[{"source":"monster_trait","roll":{"source":{"name":"Immunity"}},"damage_type":"slashing","multiplier":0}]`,
		},
		{
			name: "multiplier-only roll with a noncanonical ref",
			json: `"damage_components":[{"source":"monster_trait","roll":{"source":{"ref":"monster_traits:immunity","name":"Immunity"}},"damage_type":"slashing","multiplier":0}]`,
		},
		{
			name: "multiplier-only roll with a blank name",
			json: `"damage_components":[{"source":"monster_trait","roll":{"source":{"ref":"dnd5e:monster_traits:immunity","name":"   "}},"damage_type":"slashing","multiplier":0}]`,
		},
		{
			name: "unknown key inside the component body",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"}},"note":"x","damage_type":"slashing"}]`,
		},
		{
			name: "unknown key inside the roll body",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"note":"x"},"damage_type":"slashing"}]`,
		},
		{
			name: "unknown key inside the dice trace",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":{"notation":"d8","die_size":8,"original_rolls":[4],"final_rolls":[4],"subtotal":4,"faces":[1]}},"damage_type":"slashing"}]`,
		},
		{
			name: "unknown key inside a reroll",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":{"notation":"d8","die_size":8,"original_rolls":[2],"final_rolls":[4],"rerolls":[{"die_index":0,"before":2,"after":4,"source":{"ref":"dnd5e:conditions:fighting_style_great_weapon_fighting","name":"Great Weapon Fighting"},"why":1}],"subtotal":4}},"damage_type":"slashing"}]`,
		},
		{
			name: "duplicate key at the payload level",
			json: `"critical":false,"critical":true`,
		},
		{
			name: "duplicate key inside the component",
			json: `"damage_components":[{"source":"weapon","source":"monster_trait","damage_type":"slashing","roll":{"source":{"ref":"dnd5e:monster_traits:immunity","name":"Immunity"}}}]`,
		},
		{
			name: "duplicate key inside the roll source",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","ref":"dnd5e:weapons:dagger","name":"Longsword"}},"damage_type":"slashing"}]`,
		},
		{
			name: "duplicate key inside the dice trace",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":{"notation":"d8","die_size":8,"original_rolls":[4],"original_rolls":[5],"final_rolls":[5],"subtotal":5}},"damage_type":"slashing"}]`,
		},
		{
			name: "null multiplier",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"}},"damage_type":"slashing","multiplier":null}]`,
		},
		{
			name: "null dice trace",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":null},"damage_type":"slashing"}]`,
		},
		{
			name: "null modifier",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"modifier":null},"damage_type":"slashing"}]`,
		},
		{
			name: "null source",
			json: `"damage_components":[{"source":"weapon","roll":{"source":null},"damage_type":"slashing"}]`,
		},
		{
			name: "missing source name in the roll",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword"},"modifier":0},"damage_type":"slashing"}]`,
		},
		{
			name: "subtotal contradicts the final faces",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":{"notation":"2d6","die_size":6,"original_rolls":[2,2],"final_rolls":[2,2],"subtotal":5}},"damage_type":"slashing"}]`,
		},
		{
			name: "reroll before contradicts the current face",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":{"notation":"d8","die_size":8,"original_rolls":[3],"final_rolls":[4],"rerolls":[{"die_index":0,"before":2,"after":4,"source":{"ref":"dnd5e:conditions:fighting_style_great_weapon_fighting","name":"Great Weapon Fighting"}}],"subtotal":4}},"damage_type":"slashing"}]`,
		},
		{
			name: "reroll after outside the die",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":{"notation":"d8","die_size":8,"original_rolls":[2],"final_rolls":[9],"rerolls":[{"die_index":0,"before":2,"after":9,"source":{"ref":"dnd5e:conditions:fighting_style_great_weapon_fighting","name":"Great Weapon Fighting"}}],"subtotal":9}},"damage_type":"slashing"}]`,
		},
		{
			name: "kept indices contradict the subtotal",
			json: `"damage_components":[{"source":"weapon","roll":{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword"},"dice":{"notation":"2d10","die_size":10,"original_rolls":[3,7],"final_rolls":[3,7],"kept_indices":[0],"subtotal":10}},"damage_type":"slashing"}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(`{"beat":"struck",` + base + `,` + tc.json + `}`))
			require.Equal(t, EventStruck, kind, "a malformed known payload keeps its kind")
			require.Nil(t, body)
		})
	}
	// The miss twin: the same refusals without demoting the kind, on the one
	// shared-field arm a miss carries.
	kind, body := decodeBeat([]byte(`{"beat":"missed",` + base + `,"critical":false,"critical":true}`))
	require.Equal(t, EventMissed, kind)
	require.Nil(t, body)
}

// TestActivationResultsMapEveryProviderFieldInOrder guards the persistence
// adapter rather than one currently-produced ability at a time. Resolution's
// effect values are authoritative; Session copies every field — including the
// healing's sourced roll calculation, cloned so no captured trace aliases the
// provider's graph — and preserves publication order without parsing refs or
// redoing arithmetic.
func TestActivationResultsMapEveryProviderFieldInOrder(t *testing.T) {
	three := 3
	effects := []resolution.ActivationEffect{
		{
			Kind: resolution.EffectHealingApplied, TargetID: "alice",
			Ref: "dnd5e:features:second_wind", Name: "Second Wind",
			Amount: 2, Requested: 11, Before: 28, After: 30,
			Calculation: &dnd5eEvents.RollCalculation{
				Components: []dnd5eEvents.RollComponent{
					{
						Source: dnd5eEvents.RollSource{Ref: refs.Features.SecondWind(), Name: "Second Wind"},
						Dice: &dnd5eEvents.DiceTrace{
							Notation: "1d10", DieSize: 10,
							OriginalRolls: []int{8}, FinalRolls: []int{8}, Subtotal: 8,
						},
					},
					{
						Source: dnd5eEvents.RollSource{
							Ref: refs.Classes.Fighter(), Name: "Fighter", Label: "Fighter level",
						},
						Modifier: &three,
					},
				},
				Total: 11,
			},
		},
		{
			Kind: resolution.EffectConditionApplied, TargetID: "bob",
			Ref: "dnd5e:conditions:raging", Name: "Raging",
		},
		{
			Kind: resolution.EffectConditionRemoved, TargetID: "carol",
			Ref: "dnd5e:conditions:hidden", Name: "Hidden", Reason: "revealed",
		},
		{
			Kind: resolution.EffectCapacityGranted, TargetID: "dave",
			Description: "30ft movement",
		},
	}

	results := activationResults(effects)
	require.Len(t, results, 4)
	require.Equal(t, encounter.ResultHealingApplied, results[0].Kind)
	require.Equal(t, encounter.MemberID("alice"), results[0].Target)
	require.Equal(t, 2, results[0].Amount)
	require.Equal(t, 11, results[0].Requested)
	require.Equal(t, 28, results[0].Before)
	require.Equal(t, 30, results[0].After)
	require.NotNil(t, results[0].Calculation)
	require.Equal(t, 11, results[0].Calculation.Total)
	require.Len(t, results[0].Calculation.Components, 2)
	require.Equal(t, "dnd5e:features:second_wind", results[0].Calculation.Components[0].Source.Ref)
	require.Equal(t, "Second Wind", results[0].Calculation.Components[0].Source.Name)
	require.Equal(t, []int{8}, results[0].Calculation.Components[0].Dice.OriginalRolls)
	require.Equal(t, 8, results[0].Calculation.Components[0].Dice.Subtotal)
	require.Equal(t, "dnd5e:classes:fighter", results[0].Calculation.Components[1].Source.Ref)
	require.Equal(t, "Fighter level", results[0].Calculation.Components[1].Source.Label)
	require.NotNil(t, results[0].Calculation.Components[1].Modifier)
	require.Equal(t, 3, *results[0].Calculation.Components[1].Modifier)

	require.Equal(t, encounter.ActivationResult{
		Kind: encounter.ResultConditionApplied, Target: "bob",
		Ref: "dnd5e:conditions:raging", Name: "Raging",
		Calculation: nil, // the non-healing kinds carry no calculation
	}, results[1])
	require.Equal(t, encounter.ActivationResult{
		Kind: encounter.ResultConditionRemoved, Target: "carol",
		Ref: "dnd5e:conditions:hidden", Name: "Hidden", Reason: "revealed",
	}, results[2])
	require.Equal(t, encounter.ActivationResult{
		Kind: encounter.ResultCapacityGranted, Target: "dave",
		Description: "30ft movement",
	}, results[3])

	// No aliasing: mutating the provider's captured graph after projection
	// cannot rewrite what persistence recorded.
	effects[0].Calculation.Components[0].Dice.OriginalRolls[0] = 99
	effects[0].Calculation.Components[1].Modifier = nil
	require.Equal(t, []int{8}, results[0].Calculation.Components[0].Dice.OriginalRolls)
	require.NotNil(t, results[0].Calculation.Components[1].Modifier)
	require.Equal(t, 3, *results[0].Calculation.Components[1].Modifier)
	require.NotSame(t, effects[0].Calculation, results[0].Calculation)
	require.NotSame(t, effects[0].Calculation.Components[0].Dice,
		results[0].Calculation.Components[0].Dice)
}

// TestActivationResultBodiesDecodeExactlyOneVariant exercises each closed
// result shape, including condition-removed (which no current activation emits)
// and both healing representations. Every accepted payload populates exactly one
// pointer on the shared body.
func TestActivationResultBodiesDecodeExactlyOneVariant(t *testing.T) {
	three := 3
	cases := []struct {
		name string
		json string
		want ActivationResultBody
	}{
		{
			name: "healing applied with the legacy scalar representation",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"healing-applied","target":"alice","amount":0,"requested":10,"roll":7,"modifier":3,"before":30,"after":30,"ref":"dnd5e:features:second_wind","name":"Second Wind"}}`,
			want: ActivationResultBody{Actor: "alice", HealingApplied: &HealingAppliedBody{
				Target: "alice", Amount: 0, Requested: 10, Roll: 7, Modifier: 3,
				SourceRef: "dnd5e:features:second_wind", SourceName: "Second Wind",
				HPBefore: 30, HPAfter: 30,
			}},
		},
		{
			name: "healing applied with a sourced calculation",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"dice":{"notation":"1d10","die_size":10,"original_rolls":[8],"final_rolls":[8],"subtotal":8}},{"source":{"ref":"dnd5e:classes:fighter","name":"Fighter","label":"Fighter level"},"modifier":3}],"total":11},"ref":"dnd5e:features:second_wind","name":"Second Wind"}}`,
			want: ActivationResultBody{Actor: "alice", HealingApplied: &HealingAppliedBody{
				Target: "alice", Amount: 2, Requested: 11,
				SourceRef: "dnd5e:features:second_wind", SourceName: "Second Wind",
				HPBefore: 28, HPAfter: 30,
				Calculation: &RollCalculation{
					Components: []RollComponent{
						{
							Source: RollSource{Ref: "dnd5e:features:second_wind", Name: "Second Wind"},
							Dice: &DiceTrace{
								Notation: "1d10", DieSize: 10,
								OriginalRolls: []int{8}, FinalRolls: []int{8}, Subtotal: 8,
							},
						},
						{
							Source:   RollSource{Ref: "dnd5e:classes:fighter", Name: "Fighter", Label: "Fighter level"},
							Modifier: &three,
						},
					},
					Total: 11,
				},
			}},
		},
		{
			name: "condition applied",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"condition-applied","target":"alice","ref":"dnd5e:conditions:raging","name":"Raging"}}`,
			want: ActivationResultBody{Actor: "alice", ConditionApplied: &ConditionAppliedBody{
				Target: "alice", Ref: "dnd5e:conditions:raging", Name: "Raging",
			}},
		},
		{
			name: "condition removed",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"condition-removed","target":"bob","ref":"dnd5e:conditions:hidden","name":"Hidden","reason":"revealed"}}`,
			want: ActivationResultBody{Actor: "alice", ConditionRemoved: &ConditionRemovedBody{
				Target: "bob", Ref: "dnd5e:conditions:hidden", Name: "Hidden", Reason: "revealed",
			}},
		},
		{
			name: "capacity granted",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"capacity-granted","target":"alice","description":"30ft movement"}}`,
			want: ActivationResultBody{Actor: "alice", CapacityGranted: &CapacityGrantedBody{
				Member: "alice", Description: "30ft movement",
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(tc.json))
			require.Equal(t, EventActivationResult, kind)
			require.Equal(t, tc.want, body)

			got := body.(ActivationResultBody)
			populated := 0
			for _, present := range []bool{
				got.HealingApplied != nil, got.ConditionApplied != nil,
				got.ConditionRemoved != nil, got.CapacityGranted != nil,
			} {
				if present {
					populated++
				}
			}
			require.Equal(t, 1, populated, "one payload must produce exactly one result body")
		})
	}
}

// TestHealingBodiesRejectMixedAndCorruptRollFacts pins the strict decoder's
// refusals for the healing representation: both representations at once,
// a calculation on a non-healing kind, forbidden nulls, duplicate keys at every
// nested depth, unknown keys inside known roll bodies, arithmetic that does not
// replay, and a calculation whose total contradicts the requested heal. Every
// refusal keeps the KNOWN kind with a nil body.
func TestHealingBodiesRejectMixedAndCorruptRollFacts(t *testing.T) {
	identity := `"ref":"dnd5e:features:second_wind","name":"Second Wind"`
	cases := []struct {
		name string
		json string
		kind EventKind
	}{
		{
			name: "legacy scalars beside a calculation",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"roll":8,"modifier":3,"before":28,"after":30,"calculation":{"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"modifier":11}],"total":11},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "healing with neither representation",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "calculation on a condition-applied result",
			json: `{"kind":"condition-applied","target":"alice","ref":"dnd5e:conditions:raging","name":"Raging","calculation":null}`,
			kind: EventActivationResult,
		},
		{
			name: "null calculation on a healing result",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":null,` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "duplicate key inside the calculation",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"modifier":11}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "duplicate key inside a calculation component",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"modifier":11,"modifier":11}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "duplicate key inside a reroll source",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"dice":{"notation":"d10","die_size":10,"original_rolls":[2],"final_rolls":[11],"rerolls":[{"die_index":0,"before":2,"after":11,"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind","name":"Second Wind"}}],"subtotal":11}}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "unknown key inside the calculation",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"modifier":11}],"note":"x"},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "unknown key inside a calculation component",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"modifier":11,"note":"x"}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "unknown key inside the dice trace",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"dice":{"notation":"d10","die_size":10,"original_rolls":[11],"final_rolls":[11],"subtotal":11,"faces":[1]}}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "calculation total contradicts its components",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":12,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"modifier":11}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "dice subtotal contradicts the faces",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"dice":{"notation":"d10","die_size":10,"original_rolls":[11],"final_rolls":[11],"subtotal":9}}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "calculation total contradicts the requested heal",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":10,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"modifier":10}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "null dice inside the calculation",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind"},"dice":null,"modifier":11}]},` + identity + `}`,
			kind: EventActivationResult,
		},
		{
			name: "missing component source name",
			json: `{"kind":"healing-applied","target":"alice","amount":2,"requested":11,"before":28,"after":30,"calculation":{"total":11,"components":[{"source":{"ref":"dnd5e:features:second_wind"},"modifier":11}]},` + identity + `}`,
			kind: EventActivationResult,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(`{"beat":"activation-result","actor":"alice","result":` + tc.json + `}`))
			require.Equal(t, tc.kind, kind, "a malformed known payload keeps its kind")
			require.Nil(t, body)
		})
	}
}

// TestMalformedKnownActivationPayloadsKeepTheirKind rejects incomplete,
// multi-shape, raw-present forbidden, and duplicate-key bodies without
// demoting a recognized outer beat to unknown.
func TestMalformedKnownActivationPayloadsKeepTheirKind(t *testing.T) {
	cases := []struct {
		name string
		json string
		kind EventKind
	}{
		{
			name: "activated missing authored ability name",
			json: `{"beat":"activated","actor":"alice","ability":{"ref":"dnd5e:features:rage"}}`,
			kind: EventActivated,
		},
		{
			name: "result missing actor",
			json: `{"beat":"activation-result","result":{"kind":"capacity-granted","target":"alice","description":"30ft movement"}}`,
			kind: EventActivationResult,
		},
		{
			name: "unknown result shape",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"teleported","target":"alice"}}`,
			kind: EventActivationResult,
		},
		{
			name: "condition and capacity fields form multiple result shapes",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"condition-applied","target":"alice","ref":"dnd5e:conditions:raging","name":"Raging","description":"30ft movement"}}`,
			kind: EventActivationResult,
		},
		{
			name: "healing omits a zero-valued required fact",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"healing-applied","target":"alice","requested":10,"roll":7,"modifier":3,"before":30,"after":30,"ref":"dnd5e:features:second_wind","name":"Second Wind"}}`,
			kind: EventActivationResult,
		},
		{
			name: "condition forbidden description is present as null",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"condition-applied","target":"alice","ref":"dnd5e:conditions:raging","name":"Raging","description":null}}`,
			kind: EventActivationResult,
		},
		{
			name: "capacity forbidden identity is present as null",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"capacity-granted","target":"alice","description":"30ft movement","ref":null}}`,
			kind: EventActivationResult,
		},
		{
			name: "duplicate result with valid object last",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"teleported","target":"alice"},"result":{"kind":"capacity-granted","target":"alice","description":"30ft movement"}}`,
			kind: EventActivationResult,
		},
		{
			name: "duplicate result with valid object first",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"capacity-granted","target":"alice","description":"30ft movement"},"result":{"kind":"teleported","target":"alice"}}`,
			kind: EventActivationResult,
		},
		{
			name: "duplicate result with identical objects",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"capacity-granted","target":"alice","description":"30ft movement"},"result":{"kind":"capacity-granted","target":"alice","description":"30ft movement"}}`,
			kind: EventActivationResult,
		},
		{
			name: "duplicate nested key with identical values",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"condition-applied","target":"alice","target":"alice","ref":"dnd5e:conditions:raging","name":"Raging"}}`,
			kind: EventActivationResult,
		},
		{
			name: "duplicate nested kind with valid value last",
			json: `{"beat":"activation-result","actor":"alice","result":{"kind":"teleported","kind":"capacity-granted","target":"alice","description":"30ft movement"}}`,
			kind: EventActivationResult,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(tc.json))
			require.Equal(t, tc.kind, kind)
			require.Nil(t, body)
		})
	}
}

// TestJoinedAndExitedBodiesCarryTheMember pins bodyFor's newest arms
// (rpg-project#260 slice 4, item 2): the encounter composition's join/exit
// beats already carry "member" (encounter.go's Join and Exit), so this is
// the same decode-from-payload shape every other typed body uses — no new
// field on the wire, only a typed name for what was already there.
func TestJoinedAndExitedBodiesCarryTheMember(t *testing.T) {
	cases := []struct {
		name string
		json string
		kind EventKind
		want EventBody
	}{
		{"joined", `{"beat":"joined","member":"erin"}`, EventJoined, JoinedBody{Member: "erin"}},
		{"exited", `{"beat":"exited","member":"erin"}`, EventExited, ExitedBody{Member: "erin"}},
		// rpg-project#268: the close and the door are narrated from typed
		// facts too. An ended beat carries its declared key; a door beat
		// carries the door, its state, and — on an unlock attempt — the
		// actor and the numbers (full data until v1.0).
		{"ended", `{"beat":"ended","ending":"boss-down"}`, EventEnded, EndedBody{Ending: "boss-down"}},
		{"door opened", `{"beat":"door","door":"gate","state":"open","actor":"erin"}`,
			EventDoor, DoorBody{Door: "gate", State: "open", Actor: "erin"}},
		{"door attempt", `{"beat":"door","door":"gate","state":"locked","actor":"erin","dc":12,"total":9,"beaten":false}`,
			EventDoor, DoorBody{Door: "gate", State: "locked", Actor: "erin", DC: 12, Total: 9, Beaten: false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(tc.json))
			require.Equal(t, tc.kind, kind)
			require.Equal(t, tc.want, body)
		})
	}
}

// TestRegionRevealedCarriesTheRoomsWallsAndSealedCells is the session half of
// rpg-toolkit#1480.
//
// A client draws walls from segments now, so a reveal that carried a room's
// boundaries and not its lines would open the secret onto a room with no walls.
// The payload below is the shape the composition emits — its own atlas answer,
// sliced, with the field names this package's atlas types already use — so the
// decode is a straight unmarshal and this test is the pin that the two agree
// about the names.
//
// THE COMPOSITION'S SIDE IS PINNED IN THE COMPOSITION, where a real reveal is
// driven end to end and the beat is checked against the recipient's own
// AtlasFor byte for byte. This module cannot run that test until its pin moves
// to the encounter tag that carries it; what it can and must own is that the
// bytes arriving in this shape land in the right fields.
func TestRegionRevealedCarriesTheRoomsWallsAndSealedCells(t *testing.T) {
	payload := `{
		"beat": "region_revealed",
		"region": {
			"id": "vault", "name": "Vault",
			"cells": [{"x": 4, "y": 0}, {"x": 4, "y": 1}],
			"archetype": "crypt",
			"lighting": {"intensity": 0.2}
		},
		"props": [],
		"boundaries": [
			{"from": {"x": 3, "y": 1}, "to": {"x": 4, "y": 1},
			 "blocks_movement": true, "blocks_line_of_sight": true, "height": 2}
		],
		"segments": [
			{"from": {"q": 6, "r": 0.5}, "to": {"q": 5, "r": 2.5}, "height": 3},
			{"from": {"q": 6, "r": 3.5}, "to": {"q": 5, "r": 5.5}}
		],
		"sealed": [{"x": 6, "y": 2}]
	}`

	kind, body := decodeBeat([]byte(payload))
	require.Equal(t, EventRegionRevealed, kind)

	region, ok := body.(RegionRevealedBody)
	require.True(t, ok, "the reveal decodes to its own typed body")
	require.Equal(t, "vault", region.Region.ID)

	require.Len(t, region.Segments, 2, "the walls inside the room being revealed")
	require.Equal(t, AxialPointF{Q: 6, R: 0.5}, region.Segments[0].From,
		"fractional axial, halves and all")
	require.Equal(t, AxialPointF{Q: 5, R: 2.5}, region.Segments[0].To)
	require.Equal(t, 3.0, region.Segments[0].Height, "a raised wall is drawn raised")
	require.Zero(t, region.Segments[1].Height, "and one that authored no height keeps the standard")

	require.Equal(t, []spatial.Position{{X: 6, Y: 2}}, region.Sealed,
		"the cells of the room nobody stands on")
}

// TestRegionRevealedWithoutWallsIsStillARoom — a room with no walls of its own
// inside it reveals with no segments, and that is an ordinary answer rather
// than a malformed beat. The empty-is-the-ordinary-case rule the door reveal
// already follows.
func TestRegionRevealedWithoutWallsIsStillARoom(t *testing.T) {
	payload := `{
		"beat": "region_revealed",
		"region": {"id": "closet", "name": "Closet", "cells": [{"x": 2, "y": 0}],
		           "archetype": "crypt", "lighting": {"intensity": 1}},
		"props": [], "boundaries": []
	}`

	kind, body := decodeBeat([]byte(payload))
	require.Equal(t, EventRegionRevealed, kind)

	region, ok := body.(RegionRevealedBody)
	require.True(t, ok)
	require.Equal(t, "closet", region.Region.ID)
	require.Empty(t, region.Segments, "no walls inside it is not a defect")
	require.Empty(t, region.Sealed, "and nothing sealed is the ordinary case")
}

func TestDeathSaveBodyPreservesAuthoritativeTypedFacts(t *testing.T) {
	payload := []byte(`{"beat":"death_save","actor":"alice","death_save":{"roll":20,"outcome":"recovered","successes_added":0,"failures_added":0,"successes":0,"failures":0,"successes_needed":3,"failures_remaining":3,"stabilized":false,"dead":false,"recovered":true,"hp_restored":1,"continuation":"keep_turn","presentation_id":"opaque-save"}}`)
	kind, body := decodeBeat(payload)
	require.Equal(t, EventDeathSave, kind)
	require.Equal(t, DeathSaveBody{
		Actor: "alice", Roll: 20, Outcome: DeathSaveOutcomeRecovered,
		SuccessesNeeded: 3, FailuresRemaining: 3, Recovered: true,
		HPRestored: 1, Continuation: DeathSaveContinuationKeepTurn,
		PresentationID: "opaque-save",
	}, body)

	kind, body = decodeBeat([]byte(`{"beat":"death_save","actor":"alice","death_save":{"outcome":"success","continuation":"end_turn"}}`))
	require.Equal(t, EventDeathSave, kind)
	require.Nil(t, body, "missing opaque correlation is not a complete typed body")
}
