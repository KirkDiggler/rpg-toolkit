// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// status_grant.go is A LINE THAT GIVES A STATUS (rpg-project#1024): a line's
// row may give its speaker a status effect an author wrote — "Well fed" after
// helping the cook.
//
// THE ENCOUNTER DOES NOT APPLY IT, AND CANNOT (C1): a status is a rules
// object, and this module may not import the rules. The grant carries the
// status in the AUTHOR's words as opaque JSON that nothing here reads inside,
// and when its row lands the encounter returns it — "this member was given
// this status" — on the verb's output ([SpeakOutput.Statuses],
// [InteractOutput.Statuses]). The session hands it to the rules to compile and
// give, and saves what that changed. Returned, not acted on: there is nothing
// in the encounter's own state for a status to change.

// StatusGrant is a status a line's row gives its speaker: the status's id in
// its site and its authored block, carried unread.
type StatusGrant struct {
	// ID is the status's id in its site's `statuses:`; the same id is the
	// same status.
	ID string `json:"id"`

	// Status is the authored block — {name, changes, maintenance, again[, when_given]} —
	// as canonical JSON. The encounter never reads inside it.
	Status json.RawMessage `json:"status"`
}

// GrantedStatus is a status a landed row gave: who was given it, and what.
type GrantedStatus struct {
	// Member is who was given it: the line's speaker.
	Member MemberID

	// Grant is the status given, as authored.
	Grant StatusGrant
}

// validateStatusGrant refuses a status grant nobody could author on purpose:
// no id, or a status that is not a JSON object. What the object holds is the
// rules' to judge, not this module's.
func validateStatusGrant(what string, g StatusGrant) error {
	if g.ID == "" {
		return fmt.Errorf("%s gives a status with no id", what)
	}
	trimmed := bytes.TrimSpace(g.Status)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("%s gives status %q, whose authored block is not a JSON object", what, g.ID)
	}
	return nil
}
