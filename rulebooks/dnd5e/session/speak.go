// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// speak.go carries THE ONE SOCIAL VERB across the seam (rpg-project#569, the
// mad king use case, rulings R2, R3 and R9). A character says one authored
// line — an OPTION — to a creature or a prop. The option names the check the
// line rolls and what the world answers on either verdict.
//
// It replaced Intimidate and Persuade. Those were one machine under two names,
// differing only in which skill they rolled and which deed they landed. An
// option carries its own check, so the skill is data on the line. The deed is
// the composition's to decide from that skill (R3), never this seam's.
//
// THE MACHINE IS IN social.go. What lives here is this verb's vocabulary: its
// input, its output and its doc.

import (
	"context"
	"fmt"
)

// SpeakInput names who speaks, to what, and which line.
type SpeakInput struct {
	// Session is the session to act in.
	Session string

	// Member is who speaks — whose nerve and whose sheet make the check.
	//
	// THE HOST MUST BIND Member TO THE AUTHENTICATED CALLER, as every verb's
	// acting-as gate requires.
	Member string

	// Target is the creature spoken to, or empty when the line is said to a
	// prop. EXACTLY ONE of Target and Prop is set ([ErrNoMemberID]
	// otherwise). A creature must be able to SEE Member ([ErrUnwitnessed]).
	Target string

	// Prop is the prop spoken to — the dungeon file's `place[].id`, the id
	// [SpeakOptionRow.Prop] carries — or empty when the line is said to a
	// creature. The probe law holds: a prop Member cannot be shown is
	// [ErrNoProp], the same answer an id that names nothing earns.
	//
	// Asked after the two above: a creature must stand within
	// [InteractionReach] of Member ([ErrOutOfRange]); a prop must be on
	// Member's menu now — carried, or on a cell within reach that Member sees
	// now — or the row is no longer offered ([ErrStaleDeclaration]).
	Prop string

	// OptionID names the line, verbatim from [SpeakOptionRow.OptionID]. It
	// must be one the target offers THIS member right now ([ErrNoOption]):
	// an unknown id, and an option whose `requires` the member does not
	// meet, are one refusal.
	OptionID string
}

// SpeakOutput is the attempt, in the open: the roll is public down to the
// number (full data until v1.0), and the `spoke` beat every witness hears
// carries the same facts.
//
// Field for field the shape Intimidate and Persuade answered, plus the line
// and the target kind, because a client that learned that shape must not have
// to learn a second.
type SpeakOutput struct {
	// Beaten is whether the check beat the applied route's DC. The session
	// rolled it; the composition was told this verdict and nothing else.
	Beaten bool `json:"beaten"`

	// Total is what the check totalled. No omitempty: zero is an answer.
	//
	// CARRIED WHILE PAUSED TOO, and it is the PRE-OFFER total then — see
	// [SpeakOutput.Paused].
	Total int `json:"total"`

	// DC is what it had to reach — the APPLIED route's own difficulty, which
	// is always a number the option's `check` authored.
	DC int `json:"dc"`

	// Applied is the route the attempt actually took — chosen as the
	// member's best listed approach, exactly as a lock's is. Its Ability is
	// the skill the composition lands a deed by (R3).
	Applied DoorApproach `json:"applied"`

	// Target and Prop echo what was spoken to; exactly one is set.
	Target string `json:"target,omitempty"`
	Prop   string `json:"prop,omitempty"`

	// OptionID echoes the line that was said.
	OptionID string `json:"option_id"`

	// Paused is true when the attempt stopped to ask the member whether to
	// spend a held offer — Bardic Inspiration — before the verdict is
	// settled. Answer it with [Manager.React].
	//
	// # While it is true: Roll and Total, and nothing else
	//
	// Roll carries the d20 and Total the PRE-OFFER total. Beaten, DC and
	// Applied are the zero value: there is no verdict yet, only a question.
	// This is Unlock's rule, shared: the total alone is safe to show BECAUSE
	// the DC is withheld, so a player weighing whether the die is worth
	// spending cannot read off whether it would close the gap.
	//
	// THE ACTION IS ALREADY SPENT when this is true, on the turn clock. It
	// is charged before anything is rolled, so a member who pauses cannot
	// answer the question and then say something else with the same action.
	Paused bool `json:"paused,omitempty"`

	// Roll is the d20 as rolled, present only when Paused. The DC is
	// deliberately not surfaced beside it.
	Roll *int `json:"roll,omitempty"`

	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// Speak says one authored line as Member: a real ability check against the
// option's own approaches, resolved through the same path Unlock's lock checks
// take, and answered by the rows the author wrote on that option.
//
// # The offer comes from the target
//
// The line is the option, and the DC is the option's `check` and nothing
// else. A creature or prop with no options cannot be spoken to, and an option
// whose `requires` the member does not meet is not one they are offered (R9:
// judged on the member's OWN holdings). Both are refused with ErrNoOption
// before anything is charged or rolled, through the same composition read
// Afford lists them by — so a stale client echoing a line that has since
// stopped being offered gets the panel's own answer.
//
// # The deed is the skill
//
// A beaten check lands the deed of the skill it rolled (R3): Intimidation
// lands a threat on every witness whatever the line said, Persuasion an
// appeal, and every other skill nothing. That is the composition's rule; this
// seam passes the applied route's skill and decides nothing. A prop holds no
// deed.
//
// # Everybody may try, and the roll is what differs
//
// A character with no proficiency in the skill the applied route rolls takes
// disadvantage (rpg-project#457 R2), named in the result's own sources. That
// rule lives in resolution, attached to the skill.
//
// # It works on both clocks, and costs an action on only one
//
// On the turn clock it must be the member's turn and it costs the standard
// action, charged BEFORE the check is staged so a line that pauses for a
// Bardic Inspiration cannot be answered and then spent on somebody else. On
// the WORLD clock it costs nothing, exactly as Move does there: the world
// clock has no economy (R3, rpg-project#457).
//
// # A missed line is an outcome, not an error
//
// No deed lands, and the option's `failure` rows are rolled — which is what
// makes trying worse than not trying when the author wrote one.
//
// Errors: ErrNilInput, ErrNoMemberID, ErrNoSession, ErrNoEncounter,
// ErrNotYourTurn, ErrDowned, ErrNoMember, ErrNoProp, ErrNoOption,
// ErrCannotAfford, ErrUnwitnessed, ErrOutOfRange (a creature beyond
// [InteractionReach]: walk up first), ErrStaleDeclaration (a prop no longer
// on the menu: refresh Afford).
func (m *Manager) Speak(ctx context.Context, in *SpeakInput) (*SpeakOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("speak: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	return m.speak(ctx, in)
}
