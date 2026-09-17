// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// persuade.go carries the second shenanigan across the seam (rpg-project#458,
// ideas/shenanigans/front-room-goblin.md) — and carrying it is nearly all it
// does, which is the point. Intimidate came first; Persuade is what proves it
// built a machine rather than a feature.
//
// THE MACHINE IS IN social.go, shared. What lives here is this verb's own
// vocabulary: its input, its output and its doc. The four facts that differ —
// Persuasion where Intimidation is, character.CostOfPersuade, the placement's
// `persuade:` list, and the composition's Persuade op — are four fields of one
// value there.

import (
	"context"
	"fmt"
)

// PersuadeInput names who appeals to whom.
type PersuadeInput struct {
	// Session is the session to act in.
	Session string

	// Member is whose words — and whose sheet — make the appeal.
	Member string

	// Target is the member being appealed to. Must be able to SEE Member:
	// the audience is the witnesses, and somebody who cannot see who is
	// speaking is not among them.
	Target string
}

// PersuadeOutput is the attempt, in the open — [IntimidateOutput]'s fields
// with [IntimidateOutput]'s contracts, field for field, because a client that
// learned one shape must not have to learn a second.
type PersuadeOutput struct {
	// Beaten is whether the check beat the applied route's DC. The session
	// rolled it; the composition was told this verdict and nothing else.
	Beaten bool `json:"beaten"`

	// Total is what the check totalled. No omitempty: zero is an answer.
	// CARRIED WHILE PAUSED TOO, and it is the PRE-OFFER total then.
	Total int `json:"total"`

	// DC is what it had to reach — the APPLIED route's own difficulty. The
	// placement's authored number when it named one, and otherwise the
	// creature's own passive Insight.
	DC int `json:"dc"`

	// Applied is the route the attempt actually took — chosen by this seam
	// as the member's best listed approach, exactly as a lock's is.
	Applied DoorApproach `json:"applied"`

	// Target echoes who was appealed to, so a caller reads the result off
	// the answer rather than off what it passed in.
	Target string `json:"target"`

	// Seq is the `persuaded` beat's sequence in the ACTOR's own delivered
	// numbering (stream.go). A failed attempt gets one too: somebody tried.
	Seq uint64 `json:"seq"`

	// Paused is true when the attempt stopped to ask the member whether to
	// spend a held offer before the verdict is settled. Answer it with
	// [Manager.React]. While it is true, Roll and Total carry and Beaten, DC
	// and Applied are the zero value — [IntimidateOutput.Paused]'s rule,
	// shared rather than restated.
	Paused bool `json:"paused,omitempty"`

	// Roll is the d20 as rolled, present only when Paused. THE CREATURE'S DC
	// IS DELIBERATELY NOT SURFACED alongside it: a player deciding whether a
	// die is worth spending should not be able to read off whether it would
	// close the gap.
	Roll *int `json:"roll,omitempty"`

	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// Persuade appeals to a member as Member: a real ability check against the
// target's authored or derived approaches, resolved through the same path
// Intimidate's take, and answered by whatever the author wrote the creature
// does about it.
//
// # The DC is the creature's, authored or derived
//
// The placement's `persuade:` list when the author priced one, and otherwise
// ONE approach — Persuasion against the stat block's own passive Insight.
// Derived from the SAME number a threat is: a creature reads a liar and a
// flatterer with the same sense, and giving the two verbs different derived
// difficulties would be inventing a rule nobody wrote.
//
// # It works on both clocks, and costs an action on only one
//
// [Manager.Intimidate]'s rule, shared: the turn clock charges
// [character.CostOfPersuade] and requires the actor's turn; the world clock
// charges nothing, because it has no economy (R3, rpg-project#457).
//
// # A failed appeal is an outcome, not an error
//
// No deed is landed — and the author's `persuade_failed` table still fires,
// which is where the goblin's bad directions come from.
//
// # A character with no Persuasion rolls at disadvantage
//
// The untrained rule (rpg-project#457 R2), scoped to skill verbs, of which
// this is one. The attempt stays open to everyone; the roll is no longer the
// same for everyone.
//
// Errors: ErrNilInput, ErrNoMemberID, ErrNoSession, ErrNoEncounter,
// ErrNotYourTurn, ErrDowned, ErrCannotAfford, ErrNoSheet, ErrUnwitnessed.
func (m *Manager) Persuade(ctx context.Context, in *PersuadeInput) (*PersuadeOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("persuade: %w", ErrNilInput)
	}
	out, err := m.speak(ctx, m.persuadeVerb(), in.Session, in.Member, in.Target)
	if err != nil {
		return nil, err
	}

	return &PersuadeOutput{
		Beaten: out.Beaten, Total: out.Total, DC: out.DC, Applied: out.Applied,
		Target: in.Target, Seq: out.Seq, Paused: out.Paused, Roll: out.Roll,
		Saved: out.Saved, Delivery: out.Delivery,
	}, nil
}
