// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// party_test.go is the root `party` block (rpg-project#935): the sharing
// dial, authored as one word, compiled onto the field, round-tripped, and
// every mistake refused at its own path in a sentence the studio can show.

const partyAnchor = "key: front-room-site"

// withParty is the v4 site fixture with a `party` block written after its key.
func withParty(t *testing.T, block string) []byte {
	t.Helper()
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	return swapOneIn(t, raw, partyAnchor, partyAnchor+"\n"+block)
}

// TestEachSharingWordCompilesOntoTheField: own, view and knowledge each
// compile onto the field as authored — knowledge too, because launch is where
// it is refused, not the compiler.
func TestEachSharingWordCompilesOntoTheField(t *testing.T) {
	for _, word := range encounter.SharingWords() {
		t.Run(word, func(t *testing.T) {
			compiled, err := Load(withParty(t, "party: { sharing: "+word+" }"))
			require.NoError(t, err)
			require.Equal(t, encounter.Sharing(word), compiled.Field.Sharing)
		})
	}
}

// TestAnUnmarkedSiteCompilesNoSharing: a site that says nothing — and one
// whose party block says nothing — compiles the dial empty, which the
// encounter runs as shared view; the compiled picture of every earlier site
// is unchanged.
func TestAnUnmarkedSiteCompilesNoSharing(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	compiled, err := Load(raw)
	require.NoError(t, err)
	require.Empty(t, compiled.Field.Sharing)

	compiled, err = Load(withParty(t, "party: {}"))
	require.NoError(t, err)
	require.Empty(t, compiled.Field.Sharing)
}

// TestThePartyBlockRefusals: a word the dial does not have, a key the block
// does not take, and an authored nothing are each refused at their own path.
func TestThePartyBlockRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, block string
		want        FieldError
	}{
		{
			name:  "a word the dial does not have",
			block: "party: { sharing: everything }",
			want: FieldError{Path: "party.sharing",
				Message: `"everything" is not a sharing: write own, view or knowledge, or omit it for view`},
		},
		{
			name:  "a key the block does not take",
			block: "party: { sharing: own, size: 4 }",
			want: FieldError{Path: "party.size",
				Message: `"size" is not a key this build reads: they are sharing`},
		},
		{
			name:  "the block authored as nothing",
			block: "party: null",
			want:  FieldError{Path: "party", Message: errNotNull},
		},
		{
			name:  "the word authored as nothing",
			block: "party: { sharing: null }",
			want:  FieldError{Path: "party.sharing", Message: errNotNull},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(withParty(t, tc.block))
			require.ErrorIs(t, err, ErrBadSpec)
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Contains(t, validation.Errors, tc.want)
		})
	}
}

// TestThePartyBlockRoundTrips: an authored dial survives a decode and a
// re-encode; an unauthored one writes no `party` key.
func TestThePartyBlockRoundTrips(t *testing.T) {
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: withParty(t, "party: { sharing: own }")})
	require.NoError(t, err)
	require.Equal(t, &PartySpec{Sharing: "own"}, decoded.Spec.Party)
	encoded, err := yaml.Marshal(decoded.Spec)
	require.NoError(t, err)
	// Read back as plain YAML, not re-decoded: the fixture's `on:` tables
	// do not survive a marshal (the pre-existing asymmetry
	// TestTheV4FixtureRoundTripsItsSiteKeys names), and nothing here is
	// about them.
	var again struct {
		Party *PartySpec `yaml:"party"`
	}
	require.NoError(t, yaml.Unmarshal(encoded, &again))
	require.Equal(t, decoded.Spec.Party, again.Party)

	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	bare, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)
	require.Nil(t, bare.Spec.Party)
	encoded, err = yaml.Marshal(bare.Spec)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "party:", "a site that authored no party block writes none")
}
