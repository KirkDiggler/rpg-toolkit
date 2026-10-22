// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// social.go is THE MACHINE THE ONE SOCIAL VERB RUNS ([Manager.Speak],
// rpg-project#569). It began as the machine Intimidate and Persuade shared
// (rpg-project#454, rpg-project#458): the clock gate, the audience refusal,
// the price, the staged check, the pose window and the landing. What differed
// between those two verbs — the skill, the authored list, the composition op
// — is now data on the OPTION, so the machine has one caller and no table of
// verbs.
//
// # The rule this file exists to hold in one place
//
// Speaking is offered on BOTH clocks (R3, rpg-project#457). On the turn clock
// it is the actor's turn and it costs the standard action, exactly as a swing
// does. On the WORLD clock it costs nothing at all — not because talk is free,
// but because the world clock has no economy to charge against: that is
// Move's own rule in move.go, and inventing a second answer here would be
// this seam holding two opinions about what free roam means.
//
// # The offer comes from the target
//
// A creature or prop is spoken to ONLY through the options its binding
// authored, filtered by what THIS speaker holds (R1, R9). There is no derived
// difficulty and no default line. [linesOffered] is the one read that says
// which lines are offered, and both the offer (Afford) and the door (the
// verb) go through it, so the panel and the refusal cannot disagree.

import (
	"context"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// speakCost compiles what a line costs on the TURN clock: the standard
// action, read off the sheet by the rulebook's own compiler.
//
// THE RULEBOOK'S APPEAL PRICE, BORROWED BY NAME. The character package prices
// a social action in [character.CostOfPersuade] (and its twin
// [character.CostOfIntimidate], the same standard action). A line is that
// same action, and this module may not edit the rulebook's to rename it; the
// price stays the rulebook's either way, which is the point of not writing a
// SpendProfile here.
var speakCost = character.CostOfPersuade

// speakLanding is one settled verdict on its way into the composition.
type speakLanding struct {
	Actor    encounter.MemberID
	Target   encounter.MemberID
	Prop     encounter.PropID
	OptionID string

	// Skill is the applied route's ability — what the composition decides
	// the deed by (R3) and what the `spoke` beat reports.
	Skill string

	Beaten bool
	DC     int
	Total  int

	// Calculation is the roll behind Total, keep record and all. It rides the
	// beat so the table reads "2d20 [7, 18] kept 7 · disadvantage: Poisoned"
	// rather than one number (rpg-project#462).
	Calculation *encounter.RollCalculation
}

// landSpoken hands one settled verdict to the composition's one social op,
// which appends the `spoke` beat, lands the skill's deed, and rolls the
// option's answer rows with this session's dice.
func (m *Manager) landSpoken(ctx context.Context, enc *encounter.Encounter, in *speakLanding) (*encounter.SpeakOutput, error) {
	return enc.Speak(ctx, &encounter.SpeakInput{
		Actor: in.Actor, Target: in.Target, Prop: in.Prop,
		OptionID: in.OptionID, Skill: in.Skill,
		Beaten: in.Beaten, DC: in.DC, Total: in.Total, Calculation: in.Calculation,
		Roller: &diceSeam{roller: m.dice},
	})
}

// speak runs the one social verb end to end.
//
// Validation order, and nothing is charged on the way to a refusal: the input
// names a speaker, exactly one target and a line; the session opens; the
// clock is read; an off-turn actor ON THE TURN CLOCK is refused; a downed
// actor is refused; the line is looked up among those offered to this
// speaker; a creature target is checked to be able to SEE the actor; and only
// then is the action spent — because a line the target could never have
// heard must not cost the actor their turn on the way to being told so.
//
// Errors: ErrNoMemberID, ErrNoOption, ErrNoSession, ErrNoEncounter,
// ErrNotYourTurn, ErrDowned, ErrNoMember, ErrNoProp, ErrCannotAfford,
// ErrUnwitnessed.
func (m *Manager) speak(ctx context.Context, in *SpeakInput) (*SpeakOutput, error) {
	member := in.Member
	if member == "" {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, ErrNoMemberID)
	}
	if (in.Target == "") == (in.Prop == "") {
		return nil, fmt.Errorf("%s: a line is said to exactly one creature or prop: %w", VerbSpeak, ErrNoMemberID)
	}
	if in.OptionID == "" {
		return nil, fmt.Errorf("%s: no line named: %w", VerbSpeak, ErrNoOption)
	}

	scope, err := m.openForChange(ctx, string(VerbSpeak), in.Session)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, err)
	}

	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(member)})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, translate(err))
	}
	// REFUSED OUTSIDE THE ACTOR'S TURN *ON THE TURN CLOCK ONLY* (R3, Kirk,
	// 2026-09-17: "this will be the first getting verbs outside combat"). A
	// front room has no turn to be out of, so the world clock is not gated.
	if ClockKind(clock.Kind) == ClockTurn && string(clock.Active) != member {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, ErrNotYourTurn)
	}
	if err := refuseIfDown(scope, "member", member); err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, err)
	}

	// ASKED BEFORE ANYTHING IS CHARGED, AND BEFORE ANYTHING IS ROLLED, through
	// the read the offer lists by. A line the target does not carry, or one
	// whose `requires` this speaker does not meet, is refused here — so a
	// stale client echoing a line that has since stopped being offered gets
	// the panel's own answer rather than a check against a number nobody
	// offered them.
	lines, err := linesOffered(scope.enc, member, in.Target, in.Prop)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, err)
	}
	idx := slices.IndexFunc(lines, func(o encounter.Option) bool { return o.ID == in.OptionID })
	if idx < 0 {
		return nil, fmt.Errorf("%s: %q is not a line this speaker is offered: %w", VerbSpeak, in.OptionID, ErrNoOption)
	}
	line := lines[idx]

	if in.Target != "" {
		witnesses, err := scope.enc.Witnesses(encounter.MemberID(member))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", VerbSpeak, translate(err))
		}
		if !slices.Contains(witnesses, encounter.MemberID(in.Target)) {
			// THIS SEAM'S OWN SENTINEL, not the composition's. A raw
			// encounter error crossing here is the S2 leak.
			return nil, fmt.Errorf("%s: target %q cannot see the actor: %w", VerbSpeak, in.Target, ErrUnwitnessed)
		}
	}

	// FREE ON THE WORLD CLOCK, priced on the turn clock — move.go's rule,
	// shared rather than restated.
	if ClockKind(clock.Kind) == ClockTurn {
		if err := m.spendOnSocial(ctx, scope, member, clock.Round); err != nil {
			return nil, fmt.Errorf("%s: %w", VerbSpeak, err)
		}
	}

	if err := m.stageCheck(ctx, scope, "member", member); err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, err)
	}
	// A character with no proficiency in the skill the applied route rolls
	// simply adds no proficiency bonus: one d20, no rule imposed
	// (rpg-project#569).
	outcome, verr := m.resolveStagedCheckPoseable(scope, member, line.Check)
	if verr != nil {
		return nil, fmt.Errorf("%s %q: %w", VerbSpeak, in.OptionID, verr)
	}

	// THE ATTEMPT STOPPED TO ASK. The checker holds something that could join
	// the roll and resolution posed rather than settling. Nothing has reached
	// the target yet, so there is no deed and no beat beyond the pause itself
	// — but the action, if one was spent, is spent and stays spent.
	if outcome.Posed != nil {
		return m.poseSocialWindow(ctx, scope, in, outcome.Posed)
	}

	verdict := outcome.Verdict
	if err := requireCalculation(string(VerbSpeak), member, verdict.Calculation); err != nil {
		return nil, err
	}

	spoken, err := m.landSpoken(ctx, scope.enc, &speakLanding{
		Actor:       encounter.MemberID(member),
		Target:      encounter.MemberID(in.Target),
		Prop:        encounter.PropID(in.Prop),
		OptionID:    in.OptionID,
		Skill:       verdict.Applied.Ability,
		Beaten:      verdict.Beaten,
		DC:          verdict.Applied.DC,
		Total:       verdict.Total,
		Calculation: verdict.Calculation,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, err)
	}

	return &SpeakOutput{
		Beaten:   spoken.Beaten,
		Total:    verdict.Total,
		DC:       verdict.Applied.DC,
		Applied:  projectApproach(verdict.Applied),
		Target:   in.Target,
		Prop:     in.Prop,
		OptionID: in.OptionID,
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// linesOffered is THE ONE READ of which lines a target offers one speaker —
// the law both the offer and the door go through (rpg-project#569 R1, R9).
//
// THE COMPOSITION ANSWERS IT. Which options a binding carries, and whether
// each one's `requires` holds for this speaker's own holdings, are the
// composition's to judge ([encounter.Encounter.OptionsFor],
// [encounter.Encounter.PropOptionsFor]); this seam forwards the question and
// translates the refusal. An empty list is a target with nothing to say to
// THIS speaker — no options, or none they qualify for — and the two are
// deliberately indistinguishable, on the probe law's reason: telling them
// apart would confirm a line the author hid behind a fact.
//
// A prop is asked under the probe law: one this speaker cannot be shown is
// ErrNoProp, exactly as an id that names nothing.
func linesOffered(enc *encounter.Encounter, member, target, prop string) ([]encounter.Option, error) {
	if prop != "" {
		lines, err := enc.PropOptionsFor(encounter.MemberID(member), encounter.PropID(prop))
		if err != nil {
			return nil, translate(err)
		}
		return lines, nil
	}
	lines, err := enc.OptionsFor(encounter.MemberID(member), encounter.MemberID(target))
	if err != nil {
		return nil, translate(err)
	}
	return lines, nil
}

// spendOnSocial charges a line's price on the actor's own sheet and writes it
// back, through the same readying and the same [combat.Pay] gate a walk goes
// through (economy.go).
//
// TURN CLOCK ONLY — its one caller is gated. SAVED BEFORE THE CHECK IS
// STAGED, not after the verb finishes: stageCheck fetches the checker's record
// from the repository, so a spend still sitting in memory here would be staged
// away, and a paused attempt would resume against a sheet that never paid.
func (m *Manager) spendOnSocial(
	ctx context.Context, scope *writeScope, member string, round int,
) error {
	sheet, err := m.loadAttackSheet(ctx, member)
	if err != nil {
		return err
	}
	if err := resolution.ReadyForTurn(ctx, sheet, resolution.Turn{Number: round, Speed: sheet.GetSpeed()}); err != nil {
		return fmt.Errorf("member %q: %w: %v", member, ErrBadCost, err)
	}

	profile, err := speakCost(sheet)
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
// shape, for a line instead of a lock.
//
// THE STORY CARRIES THE LINE. A resume must land the option the member
// picked, on the target they picked, so the window names both; nothing about
// the check is re-read on the way back except the composition's own door,
// which asks again whether the line is still offered.
func (m *Manager) poseSocialWindow(
	ctx context.Context, scope *writeScope, in *SpeakInput, posed *resolution.Pause,
) (*SpeakOutput, error) {
	member := in.Member
	ask := posed.Ask
	if ask.Audience != member {
		return nil, fmt.Errorf("%s: %w: the machine asked %q on %q's roll",
			VerbSpeak, ErrInvalidWorld, ask.Audience, member)
	}
	if ask.Offer.Ref.ID == "" || ask.Offer.Name == "" {
		return nil, fmt.Errorf("%s: %w: the machine asked about an unnamed offer", VerbSpeak, ErrInvalidWorld)
	}

	if err := requirePosedCalculation(string(VerbSpeak), member, ask.Calculation); err != nil {
		return nil, err
	}

	offer := ReactionRef{Ref: ask.Offer.Ref.String(), Name: ask.Offer.Name}
	story := windowStory{Kind: storyCheck, Target: in.Target, Prop: in.Prop, Verb: VerbSpeak, Option: in.OptionID}
	if err := poseWindow(scope, *posed, story); err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, err)
	}

	if _, err := scope.enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(ask.Audience),
		Offer:    encounter.ReactionIdentity{Ref: offer.Ref, Name: offer.Name},
		Roll:     ask.Roll,
		Total:    ask.Total,
		// The beat that asks shows the whole roll, not the one face the two
		// scalars could carry (rpg-project#462 R5).
		Calculation: rollCalculationFor(ask.Calculation),
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, reportUnrecorded(scope, translate(err)))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", VerbSpeak, err)
	}

	// The PRE-OFFER total rides out beside the roll, and the DC does not —
	// [poseUnlockWindow]'s shape and [SpeakOutput.Paused]'s reasoning.
	roll := ask.Roll

	return &SpeakOutput{
		Paused:   true,
		Total:    ask.Total,
		Roll:     &roll,
		Target:   in.Target,
		Prop:     in.Prop,
		OptionID: in.OptionID,
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// socialPlacement is the half of a placement that says what the creature
// OFFERS and what it DOES — the lines an author wrote on it, and the table it
// answers from.
//
// A STRUCT RATHER THAN MORE POSITIONAL ARGUMENTS on [place], which already
// takes thirteen.
type socialPlacement struct {
	// Options is the lines a character can say to this creature
	// (rpg-project#569), carried off the compiled placement. Nil is a
	// creature nobody wrote a line for, which cannot be spoken to.
	Options []encounter.Option

	// Table is the creature's whole policy, ALREADY FOLDED: the rulebook's
	// default for its kind under the author's two layers. Whichever verb built
	// this placement did the folding ([foldedTable]); [place] carries it.
	Table encounter.Table

	// Temper is its temperament with the profile already filled in from the
	// rulebook, or a faction's mix for the composition to deal one from
	// ([resolvedTemper]).
	Temper encounter.Temper
}
