// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestTheBreakBeatIsTheCompositionsOwnString guards the coupling from this
// side, the way TestTheOutcomeBeatsAreTheCompositionsOwnStrings does for the
// three outcome kinds.
//
// BUILT FROM the composition's constant rather than from a string typed twice.
// kindFor matches on a literal, so a rename upstream would degrade every break
// to EventUnknown with nothing failing — a client's log would keep its
// sequence and quietly stop saying that anybody lost a spell.
func TestTheBreakBeatIsTheCompositionsOwnString(t *testing.T) {
	require.Equal(t, EventConcentrationEnded,
		testKindOf(`{"beat":"`+encounter.BeatConcentrationEnded+`"}`),
		"the composition's break beat must reach this seam as its own kind")
	require.Equal(t, EventKind(encounter.BeatConcentrationEnded), EventConcentrationEnded,
		"and the wire word is the composition's, unchanged: unlike down/downed there is "+
			"nothing ambiguous here to translate away from")
}

// TestTheBreakBodyCarriesWhoWhatAndWhy pins the decoder against the payload
// the composition actually writes, marshalled from its own beat shape rather
// than typed out here.
func TestTheBreakBodyCarriesWhoWhatAndWhy(t *testing.T) {
	payload := breakPayload(t, "bard-1", "dnd5e:spells:true-strike", "True Strike", "damage")

	kind, body := decodeBeat(payload)
	require.Equal(t, EventConcentrationEnded, kind)
	require.Equal(t, ConcentrationEndedBody{
		Caster: "bard-1",
		Spell:  SpellRef{Ref: "dnd5e:spells:true-strike", Name: "True Strike"},
		Reason: "damage",
	}, body)
}

// TestTheBreakBodyRefusesAnIncompletePayload holds the presence law every
// other body at this seam keeps.
//
// A break that cannot say who lost the spell, what they lost, or why is not an
// older shape to read leniently. The composition refuses all three on the way
// in, so a payload missing one is a beat this build did not write — and a body
// populated with zero values would narrate a nameless caster losing a nameless
// spell for no reason, which is worse than delivering the beat uninterpreted.
func TestTheBreakBodyRefusesAnIncompletePayload(t *testing.T) {
	cases := map[string]string{
		"no caster":     `{"beat":"concentration_ended","caster":"","spell":{"ref":"r","name":"n"},"reason":"damage"}`,
		"no spell ref":  `{"beat":"concentration_ended","caster":"c","spell":{"ref":"","name":"n"},"reason":"damage"}`,
		"no spell name": `{"beat":"concentration_ended","caster":"c","spell":{"ref":"r","name":""},"reason":"damage"}`,
		"no reason":     `{"beat":"concentration_ended","caster":"c","spell":{"ref":"r","name":"n"},"reason":""}`,
		"caster absent": `{"beat":"concentration_ended","spell":{"ref":"r","name":"n"},"reason":"damage"}`,
		"reason null":   `{"beat":"concentration_ended","caster":"c","spell":{"ref":"r","name":"n"},"reason":null}`,
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(payload))
			require.Equal(t, EventConcentrationEnded, kind,
				"the kind still crosses: a client keeps its place in the sequence")
			require.Nil(t, body,
				"but nothing is invented for the field the composition did not write")
		})
	}
}

// TestEveryBreakReasonTheRulebookWritesCrosses is the reason the field is an
// open string, made explicit.
//
// Six words end a concentration and the set is the RULEBOOK'S, growing with
// the rulebook rather than with this seam. Each is asserted crossing unchanged
// so nobody is tempted to close the set here — a closed one would have to be
// widened in three modules the next time a spell ends a new way, and a
// vocabulary this seam did not recognise would reach a client as an empty
// reason on a beat whose whole job is to say why.
func TestEveryBreakReasonTheRulebookWritesCrosses(t *testing.T) {
	// Spelled out rather than imported: the reasons live in the rulebook's
	// conditions package, and this module holds no non-test import of it by
	// the rule TestSessionConstructsNoCheck keeps. A test file may name them.
	for _, reason := range []string{
		"damage", "recast", "duration", "combat_end", "spell_ended", "caster_down",
	} {
		t.Run(reason, func(t *testing.T) {
			_, body := decodeBeat(breakPayload(t, "bard-1", "r", "n", reason))
			ended, ok := body.(ConcentrationEndedBody)
			require.True(t, ok, "a rulebook reason must not stop the body decoding")
			require.Equal(t, reason, ended.Reason, "and it crosses as written")
		})
	}
}

// breakPayload marshals one break beat the way the composition marshals it:
// the beat word from encounter's own constant, the caster, the spell's ref and
// name, and the reason.
func breakPayload(t *testing.T, caster, ref, name, reason string) []byte {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"beat":   encounter.BeatConcentrationEnded,
		"caster": caster,
		"spell":  map[string]string{"ref": ref, "name": name},
		"reason": reason,
	})
	require.NoError(t, err)
	return payload
}
