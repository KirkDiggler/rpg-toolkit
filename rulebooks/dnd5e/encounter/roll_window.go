// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// BeatRollWindowOpened is the "beat" value of the story beat this composition
// appends when a roll stops to ask somebody whether they spend something on it.
//
// EXPORTED FOR [BeatWindowOpened]'s reason: a decoder is being written against
// it in the same wave, so it arrives as a name rather than as a string literal
// every host copies.
//
// It is a SECOND beat rather than a widening of [BeatWindowOpened], and that is
// the whole reason this file exists. That beat is movement-shaped — mover,
// from, to, all load-bearing — and a roll window has no mover and no cells.
// Widening it would put three fields whose zero values lie on every post-roll
// beat.
const BeatRollWindowOpened = "roll_window_opened"

// RollWindowInput narrates one roll window opening: who is being asked, what
// they hold, and the numbers they are deciding about.
//
// THE TARGET'S AC IS DELIBERATELY ABSENT and there is no field for it. The
// client learns an AC only when the server has shown it, and a beat that
// leaked it would tell a player whether their swing lands before they choose,
// which is the whole decision. The struck beat says what it was against
// afterwards, as it does today.
type RollWindowInput struct {
	// Audience is the member being asked. Must be a member of this encounter.
	Audience MemberID

	// Offer names what they hold — the same identity shape a reaction carries,
	// because it is the same question in the story: "you may use this now".
	Offer ReactionIdentity

	// Roll is the d20 as rolled and Total the number the offer would join.
	Roll  int
	Total int

	// PresentationID is the caller's existing opaque token for this d20,
	// shared with the attack response, visual throw and eventual outcome.
	// Empty remains valid for legacy callers; never derive it from Seq.
	PresentationID string
}

// RollWindowOutput reports where the beat landed.
type RollWindowOutput struct {
	// Seq is the story sequence of the appended beat.
	Seq uint64
}

type rollWindowPayload struct {
	Beat           string                  `json:"beat"`
	Audience       MemberID                `json:"audience"`
	Offer          reactionIdentityPayload `json:"offer"`
	Roll           int                     `json:"roll"`
	Total          int                     `json:"total"`
	PresentationID string                  `json:"presentation_id,omitempty"`
}

type reactionIdentityPayload struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

// RecordRollWindow appends the beat that says a roll is waiting on an answer.
//
// # It is exported, unlike the paused walk's own window beat
//
// appendWindowOpenedBeat is private because the composition itself is what
// pauses a driven walk: it holds the paused turn and narrates it in the same
// breath. A post-roll window is posed by a verb ABOVE this composition — an
// attack that stopped inside a rules machine — and this composition never sees
// the machine. So the narration has to be reachable, and it is reachable
// through a verb with a validated input rather than through an exported
// payload struct the caller marshals itself.
//
// # It narrates and does nothing else
//
// No paused turn is stored, no drive is stopped, and Paused() is untouched.
// The encounter's PausedTurn is a driven-turn remainder — member, cells, the
// rest of a path — and validatePausedTurn refuses one with no remaining steps.
// A player's own attack is not a driven turn and has no path, so the pause
// lives entirely in the session's own ledger and this composition is told about
// it only so the table can read what happened.
//
// AUDIENCE IS EVERYONE, the pre-v1 full-data rule every other beat here keeps.
// The window is plausibly the roller's own business; when per-recipient beats
// arrive (rpg-toolkit#940) this becomes a beatClass rather than a special case.
func (e *Encounter) RecordRollWindow(in *RollWindowInput) (*RollWindowOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("record roll window: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("record roll window: %w", ErrClosed)
	}
	if in.Audience == "" {
		return nil, fmt.Errorf("record roll window: audience: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Audience]; !ok {
		return nil, fmt.Errorf("record roll window: audience %q: %w", in.Audience, ErrNoMember)
	}
	if in.Offer.Ref == "" {
		return nil, fmt.Errorf("record roll window: offer ref: %w", ErrInvalidData)
	}
	if in.Offer.Name == "" {
		// The same refusal Record keeps for a reaction's name: a beat whose
		// label is empty is a line in the story with nothing to say, and the
		// name is authored above rather than derived from the ref.
		return nil, fmt.Errorf("record roll window: offer name: %w", ErrInvalidData)
	}
	if in.Roll < 1 || in.Roll > 20 {
		// The beat exists so a player can decide with the number in front of
		// them. A d20 that does not read 1-20 is not a number to decide with.
		return nil, fmt.Errorf("record roll window: roll %d is not a d20: %w", in.Roll, ErrInvalidData)
	}

	payload, err := json.Marshal(rollWindowPayload{
		Beat:           BeatRollWindowOpened,
		Audience:       in.Audience,
		Offer:          reactionIdentityPayload{Ref: in.Offer.Ref, Name: in.Offer.Name},
		Roll:           in.Roll,
		Total:          in.Total,
		PresentationID: in.PresentationID,
	})
	if err != nil {
		return nil, fmt.Errorf("record roll window: marshal beat: %w", err)
	}

	appended, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: e.audienceFor(subjectBeat, in.Audience),
		Tags:     map[string]string{"tag": "window"},
		Payload:  payload,
	})
	if err != nil {
		return nil, fmt.Errorf("record roll window: append beat: %w", err)
	}

	return &RollWindowOutput{Seq: appended.Seq}, nil
}
