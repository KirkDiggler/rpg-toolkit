// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// social.go is THE MACHINE BOTH SOCIAL VERBS RUN (rpg-project#454,
// rpg-project#458). Intimidate came first and Persuade proved it was a
// machine: the clock gate, the audience refusal, the price, the staged check,
// the pose window and the landing are identical, and what differs is data —
// which skill the derived approach rolls, which placement list the author
// priced, which cost compiler prices it, and which composition op lands it.
//
// # The rule this file exists to hold in one place
//
// A social verb is offered on BOTH clocks (R3, rpg-project#457). On the turn
// clock it is the actor's turn and it costs the standard action, exactly as a
// swing does. On the WORLD clock it costs nothing at all — not because a
// threat is free, but because the world clock has no economy to charge
// against: that is Move's own rule in move.go, and inventing a second answer
// here would be this seam holding two opinions about what free roam means.
//
// Everything else — down, witnesses, approaches, the pose window — was already
// clock-independent and did not have to change.

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

// socialVerb is everything that differs between a threat and an appeal, in
// one value.
//
// A TABLE RATHER THAN A SWITCH, so adding Deceive is adding a value here and
// a two-line exported wrapper — and so that a reader can see at a glance that
// the two verbs differ in four facts and nothing else.
type socialVerb struct {
	// verb is the seam's own name for it, priced by Afford and carried on a
	// paused window so the answer knows which verb to finish.
	verb Verb

	// skill is the skill the DERIVED approach rolls when the author priced
	// none. The authored list, when there is one, wins whole.
	skill skills.Skill

	// cost compiles what this verb costs on the TURN clock. Nil is not a
	// free verb; there is no nil.
	cost func(*character.Character) (*combat.SpendProfile, error)

	// authored reads the placement's own approach list off the roster row.
	authored func(encounter.Member) []encounter.CheckApproach

	// land is the composition op that records the verdict and rolls the
	// creature's reaction to it.
	land func(
		context.Context, *encounter.Encounter, *socialLanding,
	) (beaten bool, seq uint64, err error)
}

// socialLanding is one settled verdict on its way into the composition.
type socialLanding struct {
	Actor  encounter.MemberID
	Target encounter.MemberID
	Beaten bool
	DC     int
	Total  int
}

// intimidateVerb is the threat: Intimidation against the target's own
// difficulty, priced at the standard action, landing a fear the coward's mind
// reads (rulebooks/dnd5e/behavior).
func (m *Manager) intimidateVerb() socialVerb {
	return socialVerb{
		verb:     VerbIntimidate,
		skill:    skills.Intimidation,
		cost:     character.CostOfIntimidate,
		authored: func(placed encounter.Member) []encounter.CheckApproach { return placed.Intimidate },
		land: func(
			ctx context.Context, enc *encounter.Encounter, in *socialLanding,
		) (bool, uint64, error) {
			out, err := enc.Intimidate(ctx, &encounter.IntimidateInput{
				Actor: in.Actor, Target: in.Target, Beaten: in.Beaten,
				DC: in.DC, Total: in.Total, Roller: &diceSeam{roller: m.dice},
			})
			if err != nil {
				return false, 0, err
			}

			return out.Beaten, out.Seq, nil
		},
	}
}

// persuadeVerb is the appeal: Intimidate's twin, with Persuasion where
// Intimidation is and the same price. Every shipped mind holds its deed and
// does nothing with it, which is the zero value telling the truth rather than
// a gap (rpg-project#458).
func (m *Manager) persuadeVerb() socialVerb {
	return socialVerb{
		verb:     VerbPersuade,
		skill:    skills.Persuasion,
		cost:     character.CostOfPersuade,
		authored: func(placed encounter.Member) []encounter.CheckApproach { return placed.Persuade },
		land: func(
			ctx context.Context, enc *encounter.Encounter, in *socialLanding,
		) (bool, uint64, error) {
			out, err := enc.Persuade(ctx, &encounter.PersuadeInput{
				Actor: in.Actor, Target: in.Target, Beaten: in.Beaten,
				DC: in.DC, Total: in.Total, Roller: &diceSeam{roller: m.dice},
			})
			if err != nil {
				return false, 0, err
			}

			return out.Beaten, out.Seq, nil
		},
	}
}

// socialOutcome is what a finished or paused social verb reached, in the one
// shape both exported outputs are built from.
type socialOutcome struct {
	Beaten   bool
	Total    int
	DC       int
	Applied  DoorApproach
	Seq      uint64
	Paused   bool
	Roll     *int
	Saved    SaveReport
	Delivery DeliveryReport
}

// speak runs one social verb end to end.
//
// Validation order, and nothing is charged on the way to a refusal: the
// session opens, the clock is read, an off-turn actor ON THE TURN CLOCK is
// refused, a downed actor is refused, the target's approaches are compiled,
// the target is checked to be able to SEE the actor, and only then is the
// action spent — because a threat the target could never have heard must not
// cost the actor their turn on the way to being told so.
//
// Errors: ErrNilInput, ErrNoMemberID, ErrNoSession, ErrNoEncounter,
// ErrNotYourTurn, ErrDowned, ErrCannotAfford, ErrNoSheet, ErrUnwitnessed.
func (m *Manager) speak(
	ctx context.Context, spec socialVerb, session, member, target string,
) (*socialOutcome, error) {
	if member == "" || target == "" {
		return nil, fmt.Errorf("%s: %w", spec.verb, ErrNoMemberID)
	}

	scope, err := m.openForChange(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, err)
	}

	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(member)})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, translate(err))
	}
	// REFUSED OUTSIDE THE ACTOR'S TURN *ON THE TURN CLOCK ONLY*. This used to
	// refuse a world-clock actor outright, on the reasoning that the verb
	// costs an action and an action belongs to a turn. R3 (Kirk,
	// 2026-09-17) ruled the other way: "this will be the first getting verbs
	// outside combat", and a front room has no turn to be out of. So the gate
	// is now what it always meant — not somebody ELSE's turn — and the world
	// clock, which has no turns at all, is simply not gated.
	if ClockKind(clock.Kind) == ClockTurn && string(clock.Active) != member {
		return nil, fmt.Errorf("%s: %w", spec.verb, ErrNotYourTurn)
	}
	if err := refuseIfDown(scope, "member", member); err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, err)
	}

	// ASKED BEFORE ANYTHING IS CHARGED. The composition asks again for itself
	// when the deed actually lands — this read is a read of a moment and it
	// does not get to be the authority — but a verb the target could never
	// have heard must not cost the actor their action on the way to being
	// refused.
	approaches, err := m.socialApproaches(scope, spec, target)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, err)
	}

	witnesses, err := scope.enc.Witnesses(encounter.MemberID(member))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, translate(err))
	}
	if !slices.Contains(witnesses, encounter.MemberID(target)) {
		// THIS SEAM'S OWN SENTINEL, not the composition's. A raw encounter
		// error crossing here is the S2 leak, and a host that cannot match it
		// answers Internal for what is an ordinary refusal.
		return nil, fmt.Errorf("%s: target %q cannot see the actor: %w", spec.verb, target, ErrUnwitnessed)
	}

	// FREE ON THE WORLD CLOCK, priced on the turn clock — move.go's rule,
	// shared rather than restated. There is no economy to charge against in
	// free roam, so there is nothing to spend and nothing to refuse.
	if ClockKind(clock.Kind) == ClockTurn {
		if err := m.spendOnSocial(ctx, scope, spec, member, clock.Round); err != nil {
			return nil, fmt.Errorf("%s: %w", spec.verb, err)
		}
	}

	if err := m.stageCheck(ctx, scope, "member", member); err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, err)
	}
	// TRUE: a social verb is a SKILL verb and takes the untrained rule
	// (rpg-project#457 R2). A character with no proficiency in the skill the
	// applied route rolls takes disadvantage, named in the result's sources.
	outcome, verr := m.resolveStagedCheckPoseable(scope, member, approaches, true)
	if verr != nil {
		return nil, fmt.Errorf("%s %q: %w", spec.verb, target, verr)
	}

	// THE ATTEMPT STOPPED TO ASK. The checker holds something that could join
	// the roll and resolution posed rather than settling. Nothing has reached
	// the creature yet, so there is no deed and no beat beyond the pause
	// itself — but the action, if one was spent, is spent and stays spent.
	if outcome.Posed != nil {
		return m.poseSocialWindow(ctx, scope, spec, member, target, outcome.Posed)
	}

	verdict := outcome.Verdict
	beaten, seq, err := spec.land(ctx, scope.enc, &socialLanding{
		Actor:  encounter.MemberID(member),
		Target: encounter.MemberID(target),
		Beaten: verdict.Beaten,
		DC:     verdict.Applied.DC,
		Total:  verdict.Total,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, err)
	}

	return &socialOutcome{
		Beaten:   beaten,
		Total:    verdict.Total,
		DC:       verdict.Applied.DC,
		Applied:  projectApproach(verdict.Applied),
		Seq:      scope.deliveredSeq(member, seq),
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// socialApproaches is the check the target prices, or the one its stat block
// derives.
//
// THE AUTHORED LIST WINS WHOLE. An author who wrote `intimidate:` or
// `persuade:` priced every route through it, and a derived approach quietly
// appended beside theirs would be a DC nobody chose sitting next to the DCs
// they did.
//
// The derived one is a single route: the verb's own skill against passive
// Insight. It needs a monster sheet, so a target with none — a player
// character — is refused rather than given a made-up number. Talking another
// player round is a use case nobody has brought, and the day somebody does it
// arrives with its own DC.
func (m *Manager) socialApproaches(
	scope *writeScope, spec socialVerb, target string,
) ([]encounter.CheckApproach, error) {
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
	if authored := spec.authored(*placed); len(authored) > 0 {
		return authored, nil
	}

	sheet, ok := npcSheet(scope.data, target)
	if !ok {
		return nil, fmt.Errorf(
			"target %q has no stat block to derive a difficulty from, and its placement priced none: %w",
			target, ErrNoSheet)
	}

	return []encounter.CheckApproach{{
		Ability: string(spec.skill),
		DC:      sheet.PassiveInsight(),
	}}, nil
}

// spendOnSocial charges the verb's price on the actor's own sheet and writes
// it back, through the same readying and the same [combat.Pay] gate a walk
// goes through (economy.go).
//
// TURN CLOCK ONLY — its one caller is gated. SAVED BEFORE THE CHECK IS
// STAGED, not after the verb finishes: stageCheck fetches the checker's record
// from the repository, so a spend still sitting in memory here would be staged
// away, and a paused attempt would resume against a sheet that never paid.
func (m *Manager) spendOnSocial(
	ctx context.Context, scope *writeScope, spec socialVerb, member string, round int,
) error {
	sheet, err := m.loadAttackSheet(ctx, member)
	if err != nil {
		return err
	}
	if err := readyForTurn(ctx, sheet, round); err != nil {
		return fmt.Errorf("member %q: %w: %v", member, ErrBadCost, err)
	}

	profile, err := spec.cost(sheet)
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

// poseSocialWindow commits the half of the attempt that happened and asks the
// member the question resolution stopped on — [Manager.poseUnlockWindow]'s
// shape, for a social verb instead of a lock.
func (m *Manager) poseSocialWindow(
	ctx context.Context, scope *writeScope, spec socialVerb, member, target string, posed *resolution.Pose,
) (*socialOutcome, error) {
	ask := posed.Ask
	if ask.Audience != member {
		return nil, fmt.Errorf("%s: %w: the machine asked %q on %q's roll",
			spec.verb, ErrInvalidWorld, ask.Audience, member)
	}
	if ask.Offer.Ref == nil || ask.Offer.Name == "" {
		return nil, fmt.Errorf("%s: %w: the machine asked about an unnamed offer", spec.verb, ErrInvalidWorld)
	}
	if len(ask.Options) != 2 {
		return nil, fmt.Errorf("%s: %w: the machine posed %d answers and this seam poses two",
			spec.verb, ErrInvalidWorld, len(ask.Options))
	}

	offer := ReactionRef{Ref: ask.Offer.Ref.String(), Name: ask.Offer.Name}
	payload, err := marshalCheckOfferPayload(checkOfferWindowPayload{
		Audience: ask.Audience,
		Target:   target,
		// WHICH verb is paused, so the answer finishes the one that was asked
		// (window.go): a resumed Persuade that landed an Intimidate would be
		// a silently wrong deed on a mind.
		Verb:   spec.verb,
		Offer:  offer,
		Roll:   ask.Roll,
		Total:  ask.Total,
		Frozen: posed.Frozen,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %v", spec.verb, ErrInvalidSession, err)
	}

	if _, err := scope.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(ask.Audience),
		Options:  []interrupt.Option{interrupt.Option(ReactStrike), interrupt.Option(ReactHold)},
		Payload:  payload,
		At:       scope.baseline,
	}); err != nil {
		return nil, fmt.Errorf("%s: %w: %v", spec.verb, ErrInvalidSession, err)
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
		return nil, fmt.Errorf("%s: %w", spec.verb, reportUnrecorded(scope, translate(err)))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, err)
	}

	// The PRE-OFFER total rides out beside the roll, and the DC does not —
	// [poseUnlockWindow]'s shape and [IntimidateOutput.Paused]'s reasoning.
	roll := ask.Roll

	return &socialOutcome{
		Paused:   true,
		Total:    ask.Total,
		Roll:     &roll,
		Seq:      scope.deliveredSeq(member, recorded.Seq),
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// Reaction is ONE entry in an authored reaction table, in this seam's own
// vocabulary (rpg-project#458).
//
// SPELLED HERE RATHER THAN IMPORTED (S2): no composition type crosses this
// seam's exported surface, so a host hands over these and [reactionsOf]
// converts at the boundary — the same move [DoorApproach] makes for a check.
type Reaction struct {
	// Weight is this entry's share of the table, AT LEAST 1. The authoring
	// dialect resolves an omitted weight to 1 before a host ever sees one;
	// the composition refuses anything lower.
	Weight int `json:"weight"`

	// Say is the creature's line, carried verbatim onto the beat. Empty means
	// it says nothing.
	Say string `json:"say,omitempty"`

	// Fact is the world fact every witness learns when this entry fires.
	// Empty means it teaches nothing.
	Fact string `json:"fact,omitempty"`

	// Flee sends the creature away from whoever spoke to it, for its own full
	// speed, as a directed move off anybody's turn.
	Flee bool `json:"flee,omitempty"`
}

// socialPlacement is the shenanigan half of a placement — what an author
// priced and what the creature does about it.
//
// A STRUCT RATHER THAN THREE MORE POSITIONAL ARGUMENTS on [place], which
// already takes fourteen: two adjacent []DoorApproach parameters are one
// careless call away from being silently swapped, and the swap would compile.
type socialPlacement struct {
	Intimidate []DoorApproach
	Persuade   []DoorApproach
	Reactions  map[string][]Reaction
}

// reactionsOf converts an authored reaction table to the composition's shape
// at the boundary and nowhere else, nil staying nil so a placement that
// authored none crosses as none.
func reactionsOf(reactions map[string][]Reaction) map[string][]encounter.Reaction {
	if reactions == nil {
		return nil
	}
	out := make(map[string][]encounter.Reaction, len(reactions))
	for key, entries := range reactions {
		rows := make([]encounter.Reaction, 0, len(entries))
		for _, entry := range entries {
			rows = append(rows, encounter.Reaction{
				Weight: entry.Weight,
				Say:    entry.Say,
				Fact:   encounter.FactID(entry.Fact),
				Flee:   entry.Flee,
			})
		}
		out[key] = rows
	}

	return out
}
