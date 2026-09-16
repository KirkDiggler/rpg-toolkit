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
// the standard action.

import (
	"context"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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
// Unlock's lock checks take.
//
// # The DC is the monster's, authored or derived
//
// The placement's `intimidate:` list when the author priced one, and
// otherwise ONE approach — Intimidation against the stat block's own passive
// Insight (10 + its Wisdom modifier, plus proficiency if the definition lists
// Insight). A goblin is DC 9 and a thug is DC 10, and neither number is
// stored anywhere: "passive is derived, never stored" (living-world §3).
//
// NOTHING IS GATED; EVERYTHING IS A CHECK (living-world §13). Every character
// may attempt this on every monster. The DC is the monster's, never a lock on
// the attempt.
//
// # It costs the standard action, and pays before it rolls
//
// [character.CostOfIntimidate], charged through the same [combat.Pay] gate a
// walk pays through, BEFORE the check is staged. A threat that pauses for a
// Bardic Inspiration has already cost the action; a member who could answer
// the question and then threaten somebody else with the same action would be
// getting two for one.
//
// # A failed threat is an outcome, not an error
//
// Nothing is landed and nothing is learned, the goblin shoots on its next
// turn, and the caller may try again next turn with a new action.
//
// # Nothing is charged on the way to a refusal
//
// The target must be able to SEE the actor, and that is asked of the
// composition ([encounter.Encounter.Witnesses]) BEFORE the action is spent.
// The composition asks again for itself when the deed lands — a read of a
// moment does not get to be the authority — but a threat the target could
// never have heard must not cost the actor their turn on the way to being
// told so.
//
// Errors: ErrNilInput, ErrNoMemberID, ErrNoSession, ErrNoEncounter,
// ErrNotYourTurn, ErrDowned, ErrCannotAfford, ErrNoSheet,
// encounter.ErrUnwitnessed.
func (m *Manager) Intimidate(ctx context.Context, in *IntimidateInput) (*IntimidateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("intimidate: %w", ErrNilInput)
	}
	if in.Member == "" || in.Target == "" {
		return nil, fmt.Errorf("intimidate: %w", ErrNoMemberID)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}

	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", translate(err))
	}
	// REFUSED OUTSIDE THE ACTOR'S TURN, exactly as a swing is: this costs an
	// action, and an action belongs to a turn. A free-roaming member has no
	// turn to spend, so there is nothing to price and the verb is not
	// available at all.
	if ClockKind(clock.Kind) != ClockTurn {
		return nil, fmt.Errorf("intimidate: member %q is not on the fight clock: %w", in.Member, ErrNotYourTurn)
	}
	if string(clock.Active) != in.Member {
		return nil, fmt.Errorf("intimidate: %w", ErrNotYourTurn)
	}
	if err := refuseIfDown(scope, "member", in.Member); err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}

	// ASKED BEFORE ANYTHING IS CHARGED. The composition asks again for
	// itself when the deed actually lands — this read is a read of a moment
	// and it does not get to be the authority — but a threat the target
	// could never have heard must not cost the actor their action on the way
	// to being refused.
	approaches, err := m.intimidateApproaches(scope, in.Target)
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}

	witnesses, err := scope.enc.Witnesses(encounter.MemberID(in.Member))
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", translate(err))
	}
	if !slices.Contains(witnesses, encounter.MemberID(in.Target)) {
		return nil, fmt.Errorf("intimidate: target %q cannot see the actor: %w",
			in.Target, encounter.ErrUnwitnessed)
	}

	if err := m.spendOnIntimidate(ctx, scope, in.Member, clock.Round); err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}

	if err := m.stageCheck(ctx, scope, "member", in.Member); err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}
	outcome, verr := m.resolveStagedCheckPoseable(scope, in.Member, approaches)
	if verr != nil {
		return nil, fmt.Errorf("intimidate %q: %w", in.Target, verr)
	}

	// THE ATTEMPT STOPPED TO ASK. The checker holds something that could
	// join the roll and resolution posed rather than settling. Nothing has
	// reached the monster yet, so there is no deed and no beat beyond the
	// pause itself — but the action is spent, and stays spent.
	if outcome.Posed != nil {
		return m.poseIntimidateWindow(ctx, scope, in, outcome.Posed)
	}

	verdict := outcome.Verdict
	landed, err := scope.enc.Intimidate(&encounter.IntimidateInput{
		Actor:  encounter.MemberID(in.Member),
		Target: encounter.MemberID(in.Target),
		Beaten: verdict.Beaten,
		DC:     verdict.Applied.DC,
		Total:  verdict.Total,
	})
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}

	return &IntimidateOutput{
		Beaten:   landed.Beaten,
		Total:    verdict.Total,
		DC:       verdict.Applied.DC,
		Applied:  projectApproach(verdict.Applied),
		Target:   in.Target,
		Seq:      scope.deliveredSeq(in.Member, landed.Seq),
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// intimidateApproaches is the check the target prices, or the one its stat
// block derives.
//
// THE AUTHORED LIST WINS WHOLE. An author who wrote `intimidate:` priced
// every route through it, and a derived approach quietly appended beside
// theirs would be a DC nobody chose sitting next to the DCs they did.
//
// The derived one is a single route: Intimidation against passive Insight.
// It needs a monster sheet, so a target with none — a player character — is
// refused rather than given a made-up number. Intimidating another player is
// a use case nobody has brought, and the day somebody does it arrives with
// its own DC.
func (m *Manager) intimidateApproaches(scope *writeScope, target string) ([]encounter.CheckApproach, error) {
	roster, err := scope.enc.Members()
	if err != nil {
		return nil, translate(err)
	}

	var placed *encounter.Member
	for i := range roster {
		if string(roster[i].ID) == target {
			placed = &roster[i]
			break
		}
	}
	if placed == nil {
		return nil, fmt.Errorf("target %q: %w", target, ErrNoMember)
	}
	if len(placed.Intimidate) > 0 {
		return placed.Intimidate, nil
	}

	sheet, ok := npcSheet(scope.data, target)
	if !ok {
		return nil, fmt.Errorf(
			"target %q has no stat block to derive a difficulty from, and its placement priced none: %w",
			target, ErrNoSheet)
	}

	return []encounter.CheckApproach{{
		Ability: string(skills.Intimidation),
		DC:      sheet.PassiveInsight(),
	}}, nil
}

// spendOnIntimidate charges the standard action on the actor's own sheet and
// writes it back, through the same readying and the same [combat.Pay] gate a
// walk goes through (economy.go).
//
// SAVED BEFORE THE CHECK IS STAGED, not after the verb finishes. stageCheck
// fetches the checker's record from the repository, so a spend still sitting
// in memory here would be staged away — and a paused attempt would resume
// against a sheet that never paid.
func (m *Manager) spendOnIntimidate(ctx context.Context, scope *writeScope, member string, round int) error {
	sheet, err := m.loadAttackSheet(ctx, member)
	if err != nil {
		return err
	}
	if err := readyForTurn(ctx, sheet, round); err != nil {
		return fmt.Errorf("member %q: %w: %v", member, ErrBadCost, err)
	}

	profile, err := character.CostOfIntimidate(sheet)
	if err != nil {
		return fmt.Errorf("member %q: %w: %v", member, ErrBadCost, err)
	}
	if err := combat.Pay(sheet, profile); err != nil {
		return fmt.Errorf("member %q: %w: the action is already spent", member, ErrCannotAfford)
	}

	// saveWalker is the scope-reporting character write, named for its first
	// caller rather than for what it does.
	return m.saveWalker(ctx, scope, sheet)
}

// poseIntimidateWindow commits the half of the attempt that happened and asks
// the member the question resolution stopped on — [Manager.poseUnlockWindow]'s
// shape, for a threat instead of a lock.
func (m *Manager) poseIntimidateWindow(
	ctx context.Context, scope *writeScope, in *IntimidateInput, posed *resolution.Pose,
) (*IntimidateOutput, error) {
	ask := posed.Ask
	if ask.Audience != in.Member {
		return nil, fmt.Errorf("intimidate: %w: the machine asked %q on %q's roll",
			ErrInvalidWorld, ask.Audience, in.Member)
	}
	if ask.Offer.Ref == nil || ask.Offer.Name == "" {
		return nil, fmt.Errorf("intimidate: %w: the machine asked about an unnamed offer", ErrInvalidWorld)
	}
	if len(ask.Options) != 2 {
		return nil, fmt.Errorf("intimidate: %w: the machine posed %d answers and this seam poses two",
			ErrInvalidWorld, len(ask.Options))
	}

	offer := ReactionRef{Ref: ask.Offer.Ref.String(), Name: ask.Offer.Name}
	payload, err := marshalCheckOfferPayload(checkOfferWindowPayload{
		Audience: ask.Audience,
		Target:   in.Target,
		Offer:    offer,
		Roll:     ask.Roll,
		Total:    ask.Total,
		Frozen:   posed.Frozen,
	})
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w: %v", ErrInvalidSession, err)
	}

	if _, err := scope.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(ask.Audience),
		Options:  []interrupt.Option{interrupt.Option(ReactStrike), interrupt.Option(ReactHold)},
		Payload:  payload,
		At:       scope.baseline,
	}); err != nil {
		return nil, fmt.Errorf("intimidate: %w: %v", ErrInvalidSession, err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true

	recorded, err := scope.enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(ask.Audience),
		Offer:    encounter.ReactionIdentity{Ref: offer.Ref, Name: offer.Name},
		Roll:     ask.Roll,
		Total:    ask.Total,
	})
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", reportUnrecorded(scope, translate(err)))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}

	// The PRE-OFFER total rides out beside the roll, and the DC does not —
	// [poseUnlockWindow]'s shape and [IntimidateOutput.Paused]'s reasoning.
	roll := ask.Roll
	return &IntimidateOutput{
		Paused:   true,
		Total:    ask.Total,
		Roll:     &roll,
		Target:   in.Target,
		Seq:      scope.deliveredSeq(in.Member, recorded.Seq),
		Saved:    report,
		Delivery: delivery,
	}, nil
}
