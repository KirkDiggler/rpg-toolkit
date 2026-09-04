// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// hold.go is THE HOLD VERB'S RULE HALF (rpg-project#368, design §4.3, R5 and
// R10), beside loot.go and sharing its reach rule, its turn rule and its
// transfer noun.
//
// # A verb is named by what the record will say (design §4.1)
//
// "Aldric holds the heirloom" is a statement, and the beat kind, the journal
// fact and this file all carry that word. Interact is a fine button and a
// poor fact, so it names no rule half here — it stays what it is, the NPC
// verb. If one button is ever wanted, Loot and Hold become declarations under
// it the way Unarmed Strike sits under Attack; the rule halves do not change.
//
// # Why HOLD and not TAKE (R10, Kirk 2026-09-04)
//
// Two reasons, and the second is the one that decided it.
//
// "Hold means something — it is a fact; maybe it is a one-handed or
// two-handed hold; maybe someone has to hold it." A holding is run-scoped
// state about a pair of hands, and the questions that come next — how many
// hands, whether holding this means not holding that — are questions about
// holding. HANDS ARE A NAMED SHELF, not built: nothing here counts them.
//
// And TAKE IS RESERVED. Taking a thing off a merchant lands it in the
// character's INVENTORY and survives the run; this verb writes a `holds:`
// fact that does not. Two verbs that both read "take" and differ in whether
// the thing is still yours next week is the collision the naming rule exists
// to prevent.
//
// # The prop leaves the atlas for EVERYONE
//
// Where a thing physically is folds on the truth grain (ruled 2026-09-01):
// one answer, the same for every member, unlike knowledge — which is
// audience-scoped and always will be. So a held prop is dropped from
// [Encounter.Atlas] itself rather than from the per-member projection, and
// every member's atlas inherits that by construction. A `held` beat goes to
// everyone present so a client can patch the atlas it is holding.
//
// # The probe law (slice 1), applied to props
//
// A guessed id must not be able to map a room nobody has found. So EVERY
// refusal about a prop standing in space the member is not shown is the same
// refusal — a bare ErrNoProp, byte-identical to the answer for an id that
// names nothing at all. A prop the member CAN see refuses by name: there is
// no secret in a pillar.
//
// The design names this rule for the "no such prop" / "not holdable" pair
// (§4.3). It has to be the whole gate rather than that pair, because the
// same leak runs through every later refusal: told apart from ErrNoProp, an
// ErrOutOfRange or an ErrAlreadyHeld about an unseen id answers "yes, there
// is something by that name in a room you have not found" just as loudly.
// So the visibility gate is hoisted ABOVE all of them, and the design's
// stated order holds unchanged for every prop the member can actually see.

// HoldInput declares picking up one holdable prop.
type HoldInput struct {
	// Member is who takes.
	Member MemberID

	// Target is the placement, by its author-given id ([PropInput.ID]).
	Target PropID

	// Range is the maximum distance, in cells, the prop may stand from
	// Member. Zero (the default) means adjacent, as [LootInput.Range] does.
	Range int
}

// HoldOutput acknowledges that the take happened, and deliberately nothing
// more — [LootOutput]'s shape and for its reason.
type HoldOutput struct{}

// Hold picks up a holdable prop: it leaves the floor for everyone, and the
// holder has it.
//
// A `held` beat goes to everyone present, naming holder and prop. The prop's
// disappearance is not a mutation of the field — the atlas stays construction
// truth — it is a fact folded over it, so the same append-only journal that
// answers "who has what" answers "where is it".
//
// Validation order (R5): nil input → empty member or target → negative Range
// → closed → not a member → no such prop → NOT SHOWN THIS MEMBER (refused as
// no such prop) → not holdable → already held → not their turn in a fight →
// not in range.
//
// Errors: ErrNilInput, ErrNoMember, ErrClosed, ErrNotMember, ErrNoProp,
// ErrNotHoldable, ErrAlreadyHeld, ErrNotActive, ErrOutOfRange,
// ErrBadPlacement.
func (e *Encounter) Hold(in *HoldInput) (*HoldOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("hold: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("hold: %w", ErrNoMember)
	}
	if in.Target == "" {
		return nil, fmt.Errorf("hold: %w", ErrNoProp)
	}
	if in.Range < 0 {
		return nil, fmt.Errorf("hold: range %d is negative: %w", in.Range, ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("hold: %w", ErrClosed)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("hold: %w", ErrNotMember)
	}

	// THE PROBE LAW, as the whole gate. Nothing in this field has that id,
	// for anybody: there is no secret to keep and nothing to point at.
	index := e.field.propIndexOf(in.Target)
	if index < 0 {
		return nil, fmt.Errorf("hold: %q: %w", in.Target, ErrNoProp)
	}

	prop := e.field.props[index]
	cell := e.field.cellAt(prop.At)
	placement, moved := e.holdings.propPlacements()[in.Target]
	if moved && !placement.gone {
		// A dropped prop is picked up from where it now lies, not from where
		// the author drew it — and it is that cell the member must be
		// shown, for the same reason.
		cell = placement.at
	}

	// Not shown this member: the SAME refusal as an id that names nothing,
	// so no guess can tell the two apart. Everything below is about a prop
	// the member can see, and says what it means.
	shown, err := e.showsCellTo(in.Member, cell)
	if err != nil {
		return nil, fmt.Errorf("hold: %w", err)
	}
	if !shown {
		return nil, fmt.Errorf("hold: %q: %w", in.Target, ErrNoProp)
	}

	if _, holdable := e.field.holdable[in.Target]; !holdable {
		return nil, fmt.Errorf("hold: %q: %w", in.Target, ErrNotHoldable)
	}
	if moved && placement.gone {
		return nil, fmt.Errorf("hold: %q: %w", in.Target, ErrAlreadyHeld)
	}

	if err := e.refuseOffTurn("take", in.Member); err != nil {
		return nil, err
	}
	from, placed := e.canvas.GetEntityPosition(string(in.Member))
	if !placed {
		return nil, fmt.Errorf("hold: member %q: %w", in.Member, ErrBadPlacement)
	}
	if err := e.refuseOutOfReachCell("take", from, cell, in.Range, string(in.Target)); err != nil {
		return nil, err
	}

	at := uint64(e.clock.ToData().HighWater)
	if err := e.holdings.markHeld(in.Member, in.Target); err != nil {
		return nil, fmt.Errorf("hold: %w", err)
	}
	if err := e.holdings.holdProp(in.Member, in.Target, "hold"); err != nil {
		return nil, fmt.Errorf("hold: %w", err)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"beat":   "held",
		"holder": string(in.Member),
		"prop":   string(in.Target),
	})
	if err != nil {
		return nil, fmt.Errorf("hold: marshal beat: %w", err)
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(subjectBeat, in.Member),
		Tags:     map[string]string{"tag": "hold"},
		Payload:  payload,
	}); err != nil {
		return nil, fmt.Errorf("hold: append beat: %w", err)
	}

	return &HoldOutput{}, nil
}

// showsCellTo reports whether a member's own atlas contains a cell — the
// probe law's question, asked of the projection that already answers it
// rather than of a second copy of the concealment rules.
//
// A field with no concealment shows every member the whole floor, so this is
// true for any floor cell there, which is exactly right: nothing is hidden,
// so nothing needs hiding behind an evasive refusal.
//
// THE ERROR IS RETURNED, NEVER FOLDED INTO "NOT SHOWN" (Copilot, PR #1497
// review). Swallowing it would answer a wiring fault — an atlas that could
// not be built at all — with ErrNoProp, which is the one refusal in this
// verb designed to be indistinguishable from three others. A caller
// debugging why a prop they can see refuses would have nothing to go on. The
// probe law is about what a PLAYER may infer from a refusal; it is not a
// reason to lie to the operator.
//
// UNREACHABLE TODAY, and said out loud rather than left for the next reader
// to work out: [Encounter.Atlas] has one return and it is nil, and
// [Encounter.AtlasFor]'s only other failure is ErrNotMember, which
// [Encounter.Hold] has already refused before it gets here. So no test kills
// a mutant that swallows this error, and a mutation pass says so. It stays
// because the alternative is discarding an error return — which is the
// habit that makes the next failure silent, whenever Atlas grows one.
func (e *Encounter) showsCellTo(member MemberID, cell spatial.Position) (bool, error) {
	atlas, err := e.AtlasFor(member)
	if err != nil {
		return false, fmt.Errorf("atlas for %q: %w", member, err)
	}
	for _, c := range atlas.Cells {
		if c == cell {
			return true, nil
		}
	}
	return false, nil
}
