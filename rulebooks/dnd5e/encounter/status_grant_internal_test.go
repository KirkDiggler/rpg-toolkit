// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// A STATUS GRANT IS A LINE ROW'S, CARRIED UNREAD (status_grant.go): its shape
// is refused only for having no id or no object; inside it nothing is read;
// an interaction's outcome gives none; one grant is one kind.
func TestAStatusGrantIsRefusedWhereNobodyCouldMeanIt(t *testing.T) {
	ok := &StatusGrant{ID: "warmed-up", Status: json.RawMessage(`{"name":"Warmed up","changes":["anything"]}`)}
	require.NoError(t, validateConsequence("row", Consequence{ApplyStatus: ok}, false), "the inside is the rules' to judge")
	for name, g := range map[string]*StatusGrant{
		"no id":         {Status: json.RawMessage(`{}`)},
		"no status":     {ID: "warmed-up"},
		"not an object": {ID: "warmed-up", Status: json.RawMessage(`["name"]`)},
		"not JSON":      {ID: "warmed-up", Status: json.RawMessage(`{name`)},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, validateConsequence("row", Consequence{ApplyStatus: g}, false))
		})
	}
	require.ErrorContains(t, validateConsequence("row", Consequence{ApplyStatus: ok, Tells: "map"}, false), "one grant is one kind")
	err := validateOutcomes("gate", map[string]bool{"in": true}, map[string][]Consequence{"in": {{ApplyStatus: ok}}})
	require.ErrorContains(t, err, "an outcome gives nobody one")
}
