// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// intimidate.go carries the first shenanigan across the seam
// (rpg-project#454, ideas/shenanigans/intimidate.md): a check whose success
// changes what a monster believes, and therefore what it does next.
//
// It is [Manager.Unlock]'s shape with a mind where the door is. The session
// stages the member's stored record, resolution resolves their best listed
// approach with the check chain live, this seam tells the composition Beaten
// and the composition compares nothing. What is different is the DC and the
// price: a lock's DC is always authored, and a monster's is authored OR
// DERIVED from its own stat block; a lock costs nothing, and a threat costs
// the standard action — on the turn clock, and nothing at all on the world
// clock.
//
// THE MACHINE ITSELF IS IN social.go, shared with Persuade. What lives here is
// this verb's own vocabulary: its input, its output and its doc.

import (
	"context"
	"fmt"
)

// IntimidateInput names who threatens whom.
type IntimidateInput struct {
	// Session is the session to act in.
	Session string

	// Member is whose nerve — and whose sheet — makes the threat.
	Member string

	// Target is the member being threatened. Must be able to SEE Member:
	// the audience is the witnesses to the threat, and somebody who cannot
	// see who is making it is not among them.
	Target string
}

// IntimidateOutput is the attempt, in the open: the roll is public down to
// the number (full data until v1.0), and the beat every witness hears
// carries the same facts.
type IntimidateOutput struct {
	// Beaten is whether the check beat the applied route's DC. The session
	// rolled it; the composition was told this verdict and nothing else.
	Beaten bool `json:"beaten"`

	// Total is what the check totalled. No omitempty: zero is an answer.
	//
	// CARRIED WHILE PAUSED TOO, and it is the PRE-OFFER total then — see
	// [IntimidateOutput.Paused].
	Total int `json:"total"`

	// DC is what it had to reach — the APPLIED route's own difficulty. The
	// placement's authored number when it named one, and otherwise the
	// monster's own passive Insight.
	DC int `json:"dc"`

	// Applied is the route the attempt actually took — chosen by this seam
	// as the member's best listed approach, exactly as a lock's is.
	Applied DoorApproach `json:"applied"`

	// Target echoes who was threatened, so a caller reads the result off
	// the answer rather than off what it passed in.
	Target string `json:"target"`

	// Seq is the `intimidated` beat's sequence in the ACTOR's own delivered
	// numbering (stream.go). A failed attempt gets one too: somebody tried.
	Seq uint64 `json:"seq"`

	// Paused is true when the attempt stopped to ask the member whether to
	// spend a held offer before the verdict is settled —
	// [UnlockOutput.Paused]'s shape. Answer it with [Manager.React].
	//
	// # While it is true: Roll and Total, and nothing else
	//
	// Roll carries the d20 and Total the PRE-OFFER total — what the check
	// stands at before the offered die would join it. Beaten, DC and
	// Applied are the zero value: there is no verdict yet, only a question,
	// and reporting a difficulty against no verdict would read as one.
	//
	// THIS IS UNLOCK'S RULE, SHARED RATHER THAN RESTATED. Both verbs pause
	// the same machine on the same kind of check, and a client that learned
	// one shape must not have to learn a second. The reasoning is
	// [checkOfferWindowPayload.Roll]'s: the total alone is safe to show
	// BECAUSE the DC is withheld, so a player weighing whether the die is
	// worth spending cannot read off whether it would close the gap.
	// Withholding the total as well would cost them the one number the
	// decision is actually about.
	//
	// THE ACTION IS ALREADY SPENT when this is true. It is charged before
	// anything is rolled, so a member who pauses cannot answer the question
	// and then threaten somebody else with the same action.
	Paused bool `json:"paused,omitempty"`

	// Roll is the d20 as rolled, present only when Paused. THE MONSTER'S DC
	// IS DELIBERATELY NOT SURFACED alongside it, for
	// [checkOfferWindowPayload.Roll]'s reason: a player deciding whether a
	// die is worth spending should not be able to read off whether it would
	// close the gap.
	Roll *int `json:"roll,omitempty"`

	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// Intimidate threatens a member as Member: a real ability check against the
// target's authored or derived approaches, resolved through the same path
// Unlock's lock checks take, and answered by whatever the author wrote the
// creature does about it.
//
// # The DC is the monster's, authored or derived
//
// The placement's `intimidate:` list when the author priced one, and
// otherwise ONE approach — Intimidation against the stat block's own passive
// Insight. A goblin is DC 9 and a thug is DC 10, and neither number is stored
// anywhere: "passive is derived, never stored" (living-world §3).
//
// NOTHING IS GATED; EVERYTHING IS A CHECK (living-world §13). Every character
// may attempt this on every monster. What CHANGED is the roll, not the
// attempt: a character with no Intimidation rolls at disadvantage
// (rpg-project#457 R2), named in the result's own sources.
//
// # It works on both clocks, and costs an action on only one
//
// On the turn clock it must be the actor's turn and it costs
// [character.CostOfIntimidate], charged BEFORE the check is staged so a threat
// that pauses for a Bardic Inspiration cannot be answered and then spent on
// somebody else. On the WORLD clock it costs nothing, exactly as Move does
// there: the world clock has no economy (R3, rpg-project#457 — "this will be
// the first getting verbs outside combat").
//
// # A failed threat is an outcome, not an error
//
// No deed is landed — and the author's `intimidate_failed` table still fires,
// which is what makes attempting worse than not attempting when they wrote
// one.
//
// # Nothing is charged on the way to a refusal
//
// The target must be able to SEE the actor, and that is asked of the
// composition ([encounter.Encounter.Witnesses]) BEFORE anything is spent. The
// composition asks again for itself when the deed lands — a read of a moment
// does not get to be the authority.
//
// Errors: ErrNilInput, ErrNoMemberID, ErrNoSession, ErrNoEncounter,
// ErrNotYourTurn, ErrDowned, ErrCannotAfford, ErrNoSheet, ErrUnwitnessed.
func (m *Manager) Intimidate(ctx context.Context, in *IntimidateInput) (*IntimidateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("intimidate: %w", ErrNilInput)
	}
	out, err := m.speak(ctx, m.intimidateVerb(), in.Session, in.Member, in.Target)
	if err != nil {
		return nil, err
	}

	return &IntimidateOutput{
		Beaten: out.Beaten, Total: out.Total, DC: out.DC, Applied: out.Applied,
		Target: in.Target, Seq: out.Seq, Paused: out.Paused, Roll: out.Roll,
		Saved: out.Saved, Delivery: out.Delivery,
	}, nil
}
