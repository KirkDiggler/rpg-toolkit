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
// # The offer comes from the NPC
//
// A creature is offered a social verb ONLY when its binding authored entries
// for that verb (rpg-project#494 R1). There is no derived difficulty and no
// default: absence is "this creature does not do that", not "use the
// rulebook's number". [socialEntriesOf] is the one read that says so, and
// both the offer (Afford) and the door (the verb) go through it, so the panel
// and the refusal cannot disagree.
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

	// cost compiles what this verb costs on the TURN clock. Nil is not a
	// free verb; there is no nil.
	cost func(*character.Character) (*combat.SpendProfile, error)

	// authored reads the placement's own approach list off the roster row —
	// the ONLY source of this verb's difficulty (rpg-project#494 R1). Empty
	// is a creature the author did not give this verb to, not a creature to
	// derive a number for.
	authored func(encounter.Member) []encounter.CheckApproach

	// nobody is what Afford says when the actor has an audience and nobody in
	// it carries this verb's entries — the [ShortfallNoSocialEntry] text, in
	// the verb's own words rather than assembled from its name.
	nobody string

	// land is the composition op that records the verdict and rolls the
	// creature's answer to it.
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

	// Calculation is the roll behind Total, keep record and all. It rides the
	// beat so the table reads "2d20 [7, 18] kept 7 · disadvantage: Untrained"
	// rather than one number (rpg-project#462).
	Calculation *encounter.RollCalculation
}

// intimidateVerb is the threat: Intimidation against the target's own
// authored difficulty, priced at the standard action, landing a fear the
// coward's mind reads (rulebooks/dnd5e/behavior).
func (m *Manager) intimidateVerb() socialVerb {
	return socialVerb{
		verb:     VerbIntimidate,
		cost:     character.CostOfIntimidate,
		authored: func(placed encounter.Member) []encounter.CheckApproach { return placed.Intimidate },
		nobody:   "nobody here can be intimidated",
		land: func(
			ctx context.Context, enc *encounter.Encounter, in *socialLanding,
		) (bool, uint64, error) {
			out, err := enc.Intimidate(ctx, &encounter.IntimidateInput{
				Actor: in.Actor, Target: in.Target, Beaten: in.Beaten,
				DC: in.DC, Total: in.Total, Calculation: in.Calculation,
				Roller: &diceSeam{roller: m.dice},
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
		cost:     character.CostOfPersuade,
		authored: func(placed encounter.Member) []encounter.CheckApproach { return placed.Persuade },
		nobody:   "nobody here can be persuaded",
		land: func(
			ctx context.Context, enc *encounter.Encounter, in *socialLanding,
		) (bool, uint64, error) {
			out, err := enc.Persuade(ctx, &encounter.PersuadeInput{
				Actor: in.Actor, Target: in.Target, Beaten: in.Beaten,
				DC: in.DC, Total: in.Total, Calculation: in.Calculation,
				Roller: &diceSeam{roller: m.dice},
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
// refused, a downed actor is refused, the target's authored entries are read,
// the target is checked to be able to SEE the actor, and only then is the
// action spent — because a threat the target could never have heard must not
// cost the actor their turn on the way to being told so.
//
// Errors: ErrNilInput, ErrNoMemberID, ErrNoSession, ErrNoEncounter,
// ErrNotYourTurn, ErrDowned, ErrNoMember, ErrNoSocialEntry, ErrCannotAfford,
// ErrUnwitnessed.
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

	// ASKED BEFORE ANYTHING IS CHARGED, AND BEFORE ANYTHING IS ROLLED. A
	// creature the author never gave this verb to is refused by name here
	// (R3, rpg-project#494), so a stale client echoing a row that has since
	// stopped being offered gets the same answer the panel gives rather than
	// a check against a number nobody wrote.
	roster, err := scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.verb, translate(err))
	}
	approaches, err := socialEntriesOf(roster, target, spec)
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
	if err := requireCalculation(string(spec.verb), member, verdict.Calculation); err != nil {
		return nil, err
	}

	beaten, seq, err := spec.land(ctx, scope.enc, &socialLanding{
		Actor:       encounter.MemberID(member),
		Target:      encounter.MemberID(target),
		Beaten:      verdict.Beaten,
		DC:          verdict.Applied.DC,
		Total:       verdict.Total,
		Calculation: verdict.Calculation,
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

// socialEntriesOf is THE ONE READ of what an author priced on a creature for
// one social verb — the law both the offer and the door go through
// (rpg-project#494 R1/R2/R3).
//
// THE AUTHORED LIST IS THE WHOLE ANSWER. A creature is offered a social verb
// when, and only when, its binding wrote entries for it; no entries is
// [ErrNoSocialEntry], not a difficulty derived from the stat block. That
// derived approach — the verb's own skill against passive Insight — is
// RETIRED: it let a creature nobody configured be threatened anyway, which
// made absence mean "use the default" instead of "this creature does not do
// that". The World Builder is now the only place a creature gains a social
// verb, which is the point.
//
// IT TAKES THE ROSTER, NOT THE ENCOUNTER, and that is not an accident. Afford
// asks this question once per witness to build one row's candidates; reading
// the roster inside would read it once per witness on a panel that refreshes
// every frame. The caller reads the roster once and every witness is judged
// by this one body.
func socialEntriesOf(
	roster []encounter.Member, target string, spec socialVerb,
) ([]encounter.CheckApproach, error) {
	for i := range roster {
		if string(roster[i].ID) != target {
			continue
		}
		entries := spec.authored(roster[i])
		if len(entries) == 0 {
			return nil, fmt.Errorf(
				"target %q has no authored %s entries: %w", target, spec.verb, ErrNoSocialEntry)
		}

		return entries, nil
	}

	return nil, fmt.Errorf("target %q: %w", target, ErrNoMember)
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

	if err := requirePosedCalculation(string(spec.verb), member, ask.Calculation); err != nil {
		return nil, err
	}

	offer := ReactionRef{Ref: ask.Offer.Ref.String(), Name: ask.Offer.Name}
	payload, err := marshalCheckOfferPayload(checkOfferWindowPayload{
		Audience: ask.Audience,
		Target:   target,
		// WHICH verb is paused, so the answer finishes the one that was asked
		// (window.go): a resumed Persuade that landed an Intimidate would be
		// a silently wrong deed on a mind.
		Verb:        spec.verb,
		Offer:       offer,
		Roll:        ask.Roll,
		Total:       ask.Total,
		Calculation: sessionRollCalculationOf(ask.Calculation),
		Frozen:      posed.Frozen,
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
		// The beat that asks shows the whole roll, not the one face the two
		// scalars could carry (rpg-project#462 R5).
		Calculation: rollCalculationFor(ask.Calculation),
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

// socialPlacement is the half of a placement that says what the creature COSTS
// and what it DOES — what an author priced, and the table it answers from.
//
// A STRUCT RATHER THAN FOUR MORE POSITIONAL ARGUMENTS on [place], which already
// takes thirteen: two adjacent []DoorApproach parameters are one careless call
// away from being silently swapped, and the swap would compile.
//
// `Answers` USED TO SIT HERE, with a [session.Answer] twin beside it and a
// converter under it. It is gone rather than kept beside Table: the four social
// outcomes are KEYS of the same table now (rpg-project#465), and carrying both
// would be the dual representation this repo bans — with the added defect that
// a host filling one and not the other would get a creature whose social
// answers and whose turns came from different files.
type socialPlacement struct {
	Intimidate []DoorApproach
	Persuade   []DoorApproach

	// Table is the creature's whole policy, ALREADY FOLDED: the rulebook's
	// default for its kind under the author's two layers. Whichever verb built
	// this placement did the folding ([foldedTable]); [place] carries it.
	Table encounter.Table

	// Temper is its temperament with the profile already filled in from the
	// rulebook, or a faction's mix for the composition to deal one from
	// ([resolvedTemper]).
	Temper encounter.Temper
}
