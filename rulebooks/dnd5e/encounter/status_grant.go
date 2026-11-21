// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// status_grant.go is A LINE THAT GIVES A STATUS (rpg-project#1024): a line's
// row may give a status effect an author wrote — "Warmed up" to the speaker
// after helping the cook, "Held" to the speaker's party a guard sees
// (rpg-project#990).
//
// THE ENCOUNTER DOES NOT APPLY IT, AND CANNOT (C1): a status is a rules
// object, and this module may not import the rules. The grant carries the
// status in the AUTHOR's words as opaque JSON that nothing here reads inside,
// and when its row lands the encounter returns it — "this member was given
// this status" — on the verb's output ([SpeakOutput.Statuses],
// [InteractOutput.Statuses]). The session hands it to the rules to compile and
// give, and saves what that changed. Returned, not acted on: there is nothing
// in the encounter's own state for a status to change.

// StatusGrant is a status a line's row gives: the status's id in its site,
// its authored block, carried unread, and who receives it.
//
// WHO RECEIVES IT is a who-word written at [WhoInApplyStatusTo], resolved when
// the row lands by the same resolver `give` uses ([Encounter.whoNow]):
//
//   - `speaker` is the line's speaker, UNCONDITIONALLY — seen or not, on a
//     thing's row as on a creature's;
//   - `party_in_sight` is the speaker's party the creature spoken to sees at
//     that moment, the speaker included only if seen — `give`'s reading;
//   - `spoken_to` is the creature the line was said to.
//
// A row of a thing has nobody spoken to, so its `party_in_sight` and
// `spoken_to` resolve to nobody. Each member the word resolves to is returned
// once ([GrantedStatus]).
//
// `spoken_to` ALWAYS HAS A SHEET TODAY because only authored creatures carry
// lines — a world NPC is placed with none — so no line is ever said to one; if
// world NPCs ever get authored lines, that change must refuse `to: spoken_to`
// on them.
//
// EVERY MEMBER THE WORD RESOLVES TO IS RETURNED, sheet or no sheet: the
// encounter does not know which members the rules hold a sheet for, and does
// not guess. A receiver the rules cannot give the status to fails that give,
// and the whole line is refused, as any failed give is — all or nothing.
// Never add a silent skip here: a status dropped without a word is a rule
// that did not happen.
type StatusGrant struct {
	// ID is the status's id in its site's `statuses:`; the same id is the
	// same status.
	ID string `json:"id"`

	// Status is the authored block — {name, changes, maintenance, again[, when_given]} —
	// as canonical JSON. The encounter never reads inside it.
	Status json.RawMessage `json:"status"`

	// To is who receives it: a word of [WhoInApplyStatusTo]. REQUIRED, and
	// never defaulted (rpg-project#990): a grant with none — a run saved by
	// an engine from before this field — is refused when it is set up or
	// loaded, saying the run must be started again.
	To string `json:"to,omitzero"`

	// Until is how long the status lasts when it is given for a time
	// (rpg-project#1080): a number of rounds of world time, written where
	// the status is given. Nil is a status with no time of its own. Read here
	// and never inside [StatusGrant.Status]: the number is on the giving,
	// which is this module's content, so the encounter can read it without
	// reading inside a status.
	Until *StatusUntil `json:"until,omitzero"`
}

// StatusUntil is how long a status given here lasts, in rounds of world
// time, in or out of a fight; any of the status's own endings may end it
// sooner. Immutable once built, so a copy of a pointer to it is a copy.
type StatusUntil struct {
	// Rounds is how many rounds, 1 to 1000.
	Rounds int `json:"rounds"`
}

// GrantedStatus is a status a landed row gave: who was given it, and what.
type GrantedStatus struct {
	// Member is who was given it: one member the grant's [StatusGrant.To]
	// resolved to when the row landed.
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
	if g.To == "" {
		// A PLAYER'S REFUSAL, BY NAME: the one way to get here is a game
		// saved before the receiver was always stated — an author's site is
		// refused first by the compiler, at their path, with what to write.
		return fmt.Errorf("%s gives status %q with no receiver: %w", what, g.ID, ErrSavedByOlderVersion)
	}
	if _, ok := WhoWordAt(WhoInApplyStatusTo, g.To); !ok {
		return fmt.Errorf("%s gives status %q to %q: who receives it is one of %q",
			what, g.ID, g.To, WhoWordsIn(WhoInApplyStatusTo))
	}
	if g.Until != nil {
		if why := StatusUntilRefusal(g.Until.Rounds); why != "" {
			return fmt.Errorf("%s gives status %q: %s: %w", what, g.ID, why, ErrInvalidData)
		}
	}
	return nil
}
