// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// persuade.go is INTIMIDATE'S TWIN ON THE SAME MACHINE (rpg-project#458,
// ideas/shenanigans/front-room-goblin.md): the second social verb, added to
// prove the first one built a machine rather than a feature.
//
// Everything that makes a threat work makes an appeal work, and it makes it
// work through the SAME BODY ([Encounter.social]) rather than a copy: the
// audience is the witnesses, the target must be able to see the actor, the
// beat goes down the log whether it landed or not, a beaten check lands a
// deed, and the author's table decides what the creature does about it. What
// differs is the verb the deed lands under and which half of the table the
// verdict reads — both data, neither behaviour.
//
// WHAT IS NOT HERE IS THE POINT. No clock gate and no bubble: this verb works
// in a front room with no fight in it, which is the primitive this slice buys
// (R3). No price and no DC: a price is a question about a sheet and a DC is a
// question about a stat block, and this composition can see neither (C1).

// BeatPersuaded is the "beat" value of the story beat this composition appends
// when somebody talks a member round — landed or not.
//
// EXPORTED BECAUSE A DECODER READS IT, [BeatIntimidated]'s reason exactly, and
// it matters here for the same reason it matters there: this beat is the ONLY
// account of the roll, and a missed appeal writes nothing else whatsoever.
const BeatPersuaded = "persuaded"

// PersuadeInput names who appealed to whom, whether the check was beaten, and
// the numbers the table should see. [IntimidateInput]'s fields, with
// [IntimidateInput]'s contracts.
type PersuadeInput struct {
	// Actor is the member making the appeal. Must be a member of this
	// encounter (ErrNotMember) and must be placed (ErrBadPlacement).
	Actor MemberID

	// Target is the member being appealed to. Must be a member, and must be
	// able to SEE the actor — in [Encounter.witnessesOf] of the actor's cell
	// (ErrUnwitnessed otherwise).
	Target MemberID

	// Beaten is whether the check beat the DC. CARRIED, NEVER COMPARED: this
	// module holds no 5e, so who decides a total beats a difficulty lives on
	// the other side of this seam.
	//
	// FALSE LANDS NO DEED and is not an error — but it is NOT nothing:
	// `persuade_failed` is a table of its own, and a creature that gives bad
	// directions when you fail to win it over is the whole reason failure is
	// authorable (the design).
	Beaten bool

	// DC and Total are the numbers the table sees, carried onto the beat so
	// the roll is visible whether it landed or not. Filled by the session,
	// which ran the check; nothing here reads them.
	DC    int
	Total int

	// Roller is the world's die, the one the reaction table is picked with.
	// REQUIRED ([ErrNoRoller]) — see [IntimidateInput.Roller] for why it is
	// per call rather than a constructor capability.
	Roller dice.Roller
}

// PersuadeOutput reports what the appeal reached.
type PersuadeOutput struct {
	// Beaten echoes what the caller said, so a caller reads the result off
	// the answer rather than off the fact that it called.
	Beaten bool

	// Witnesses is every member who saw it — who holds the deed on a
	// success, and who was told regardless. Sorted, the audience's own order.
	Witnesses []MemberID

	// Seq is the sequence number of the `persuaded` beat. A failed attempt
	// gets one too: somebody tried, and the story is what happened rather
	// than what worked.
	Seq uint64
}

// Persuade reports an appeal to a member, and on a beaten check lands the deed
// that won it over — then rolls whatever the author wrote the creature does
// about it.
//
// Validation order, refusals and beat ordering are [Encounter.Intimidate]'s,
// because it is the same body.
//
// Errors: ErrNilInput, ErrNoMember, ErrNoRoller, ErrClosed, ErrNotMember,
// ErrBadPlacement, ErrUnwitnessed.
func (e *Encounter) Persuade(ctx context.Context, in *PersuadeInput) (*PersuadeOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("persuade: %w", ErrNilInput)
	}
	out, err := e.social(ctx, socialInput{
		verb: DeedPersuade, beat: BeatPersuaded, tag: "persuade",
		actor: in.Actor, target: in.Target, beaten: in.Beaten,
		dc: in.DC, total: in.Total, roller: in.Roller,
	})
	if err != nil {
		return nil, err
	}

	return &PersuadeOutput{Beaten: out.beaten, Witnesses: out.witnesses, Seq: out.seq}, nil
}
