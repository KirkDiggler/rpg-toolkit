// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
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
