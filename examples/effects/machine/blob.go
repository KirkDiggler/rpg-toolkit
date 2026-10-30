// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Blob is what a condition persists (design E1, E4): the header a loader
// routes on, the current state, and the bag's keys beside them in the same
// flat JSON object, so a key a consumer duck-types (turns_active) sits where
// it does today. Validate refuses a bag key that collides with the header.
type Blob struct {
	Ref      core.Ref  `json:"ref"`
	MemberID string    `json:"member_id"`
	Source   *core.Ref `json:"source,omitzero"`
	State    string    `json:"state"`
	Bag      Bag       `json:",embed"`
}

// headerKeys are the blob keys a bag key may not take.
var headerKeys = map[string]bool{"ref": true, "member_id": true, "source": true, "state": true}

// UnmarshalJSONFrom decodes a blob, landing whole JSON numbers in the bag as
// int. It returns the decoder's error for malformed JSON or a bad ref.
func (b *Blob) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain Blob
	return json.UnmarshalDecode(dec, (*plain)(b), wholeNumbers)
}

// Marshal writes the blob as one JSON object with keys in a stable order.
func (b *Blob) Marshal() ([]byte, error) {
	return json.Marshal(b, json.Deterministic(true))
}
