// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// This file holds the four story record functions — attack, step, cast and
// check — and the landings built around them. Each record function is the
// same for an output that paused and one that finished: what settled is told
// in the output it settled in (one pause envelope), so a paused output records
// what settled before its pause and a resumed one records what settled after.

// settled reports whether an output carries a unit to tell. A paused output
// whose pause came before anything settled carries none — nil, or a sequence
// that has swung nothing yet.
func settled(out *resolution.Output) bool {
	switch o := out.Outcome.(type) {
	case nil:
		return false
	case resolution.SequenceOutcome:
		return len(o.Steps) > 0 || out.Posed == nil
	default:
		return true
	}
}

// tellConcentration is a landing's record step where concentration can exist
// with no unit to ride behind: a cast paused on a save, which tells its cast
// only when it ends, and a turn boundary. It tells the checks and breaks
// through the encounter's own verb, with actor as the member whose rule
// caused them, and tells nothing when there is nothing to tell.
func tellConcentration(actor string) func(*encounter.Encounter, concentration) error {
	return func(enc *encounter.Encounter, told concentration) error {
		if told.empty() {
			return nil
		}
		_, err := enc.TellConcentration(&encounter.TellConcentrationInput{
			Actor:  encounter.MemberID(actor),
			Checks: told.Checks,
			Breaks: told.Breaks,
		})
		return err
	}
}

// attackLanded is what an attack landing recorded, for a verb that reports
// it: the hit's record, and the roll-window beat a post-roll window told.
type attackLanded struct {
	hit    *encounter.RecordOutput
	window *encounter.RollWindowOutput
}

// attackLanding is the one landing of an attack story's output, paused or
// finished: Record is [Manager.recordAttack] over what settled, nil when
// nothing did (so concentration with no unit is refused, as it can exist only
// behind a hit); Window poses the pause when there is one. live is the
// encounter a seam was called from, nil for a verb. got, when non-nil, is
// filled with what was recorded.
func (m *Manager) attackLanding(
	scope *writeScope, live *encounter.Encounter, story windowStory, out *resolution.Output, got *attackLanded,
) *landing {
	if got == nil {
		got = &attackLanded{}
	}
	l := &landing{Live: live}
	if settled(out) {
		outcome := out.Outcome
		l.Record = func(enc *encounter.Encounter, told concentration) error {
			hit, err := m.recordAttack(enc, story, outcome, told)
			got.hit = hit
			return err
		}
	}
	if out.Posed != nil {
		pause := *out.Posed
		l.Window = func(enc *encounter.Encounter) error {
			beat, err := openWindow(enc, scope, pause, story)
			got.window = beat
			return err
		}
	}
	return l
}

// recordAttack is the attack story's record function.
//
// A lone strike records its hit unless it is the [resolution.StrikeOutcome]
// Continued half of a post-hit pause, whose hit was told when it paused; then
// its retaliation. A sequence records each settled step the same way, each
// with its own concentration. It returns the hit's record for a verb that
// reports one.
func (m *Manager) recordAttack(
	enc *encounter.Encounter, story windowStory, outcome resolution.Outcome, told concentration,
) (*encounter.RecordOutput, error) {
	switch o := outcome.(type) {
	case resolution.StrikeOutcome:
		if o.Continued {
			return nil, recordRetaliation(enc, o.Retaliation, told)
		}
		hit, err := enc.Record(recordStrike(story.Attacker, story.Target, o,
			attackRefFor(story.Definition), story.PresentationID, told.Checks, told.Breaks))
		if err != nil {
			return nil, err
		}
		return hit, recordRetaliation(enc, o.Retaliation, concentration{})
	case resolution.SequenceOutcome:
		return nil, recordSequence(enc, story, o)
	default:
		return nil, fmt.Errorf("%w: attack produced %T", ErrInvalidWorld, outcome)
	}
}

// recordSequence writes ONE BEAT PER SWING, for the steps this output settled.
//
// Because a beat is a roll: every field the story keeps about an attack is
// singular, so the goblin boss's Multiattack is two Struck/Missed beats, each
// naming the COMPONENT that swung — looked up by the step's own action ref in
// the attacker's list, never by position, because a resumed sequence reports
// from the swing it paused on.
//
// Each step carries its OWN concentration (Kirk's ruling, 2026-09-20), and the
// output-level lists are empty for a sequence, so reading both places cannot
// record one save twice. A Continued step's hit was told at its pause; it
// records only its retaliation.
func recordSequence(enc *encounter.Encounter, story windowStory, sequence resolution.SequenceOutcome) error {
	if len(sequence.Steps) == 0 {
		// A finished sequence that swung nothing is a machine defect, and a
		// silent return would look exactly like a turn where nothing was in
		// reach.
		return fmt.Errorf("strike: %w: %s swung nothing", ErrInvalidWorld, sequence.Action.String())
	}
	for index, step := range sequence.Steps {
		told := concentration{Checks: step.ConcentrationChecks, Breaks: step.ConcentrationBreaks}
		if step.Strike.Continued {
			if err := recordRetaliation(enc, step.Strike.Retaliation, told); err != nil {
				return err
			}
			continue
		}
		component, found := definitionFor(story.Components, step.Action)
		if !found {
			// Unreachable through resolution, which resolved these refs
			// against this very list. Named anyway: a beat labelled with the
			// wrong weapon is worse than a turn that failed.
			return fmt.Errorf("strike: %w: %s swung %s, which the attacker does not carry",
				ErrBadAttack, sequence.Action.String(), step.Action.String())
		}
		recorded := recordStrike(story.Attacker, story.Target, step.Strike,
			attackRefFor(component), "", told.Checks, told.Breaks)
		if _, err := enc.Record(recorded); err != nil {
			return fmt.Errorf("strike: step %d: %w", sequence.From+index, translate(err))
		}
		if err := recordRetaliation(enc, step.Strike.Retaliation, concentration{}); err != nil {
			return err
		}
	}
	return nil
}

// stepLanding is the one landing of a step story's output, paused or
// finished, and the windows it opens.
//
// Record is [stepRecord] over the reactions that settled. Window poses every
// pause the output leaves: the one a swing stopped on, and each player the
// step asks whether to swing (ruling E8 — the ask stops the step, all asks
// stand at once). THE STEP ITSELF IS NEVER LANDED HERE: the encounter's pause
// holds it and takes it once, after the last asked player answered.
func (m *Manager) stepLanding(
	scope *writeScope, live *encounter.Encounter, story windowStory, out *resolution.Output,
) (*landing, []encounter.PausedWindow, error) {
	l := &landing{Live: live}
	var asked []resolution.Pause
	switch moved := out.Outcome.(type) {
	case nil:
	case resolution.MovementOutcome:
		record, err := stepRecord(moved)
		if err != nil {
			return nil, nil, err
		}
		l.Record = record
		asked = moved.Asked
	default:
		// Refused at the record step: after the sheets are written, so the
		// report names what landed.
		l.Record = func(*encounter.Encounter, concentration) error {
			return fmt.Errorf("%w: movement produced %T", ErrInvalidWorld, out.Outcome)
		}
	}

	var pauses []resolution.Pause
	if out.Posed != nil {
		pauses = append(pauses, *out.Posed)
	}
	pauses = append(pauses, asked...)
	windows := make([]encounter.PausedWindow, 0, len(pauses))
	for _, pause := range pauses {
		windows = append(windows, encounter.PausedWindow{
			Audience: encounter.MemberID(pause.Ask.Audience),
			Reaction: encounter.ReactionIdentity{Ref: pause.Ask.Offer.Ref.String(), Name: pause.Ask.Offer.Name},
		})
	}
	if len(pauses) > 0 {
		l.Window = func(*encounter.Encounter) error {
			for _, pause := range pauses {
				if err := poseWindow(scope, pause, story); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return l, windows, nil
}

// stepRecord is the step story's record function: one strike beat per
// reaction the step provoked, named AS the reaction it was, then its
// retaliation; a Continued reaction's hit was told at its pause, so it
// records only its retaliation.
//
// EVERY BEAT IS BUILT BEFORE ANYTHING IS WRITTEN. The only way building one
// can fail is a reaction with no display name, and that refusal costs nothing
// durable when it comes first.
func stepRecord(moved resolution.MovementOutcome) (func(*encounter.Encounter, concentration) error, error) {
	type told struct {
		beat        *encounter.RecordInput
		retaliation *resolution.RetaliationOutcome
		continued   concentration
	}
	tells := make([]told, 0, len(moved.Reactions))
	for _, reaction := range moved.Reactions {
		own := concentration{Checks: reaction.ConcentrationChecks, Breaks: reaction.ConcentrationBreaks}
		if reaction.Struck.Continued {
			tells = append(tells, told{retaliation: reaction.Struck.Retaliation, continued: own})
			continue
		}
		if reaction.ConditionName == "" {
			// The composition refuses a reaction identity with no name, and
			// inventing one from the ref would ship whatever the id spells.
			return nil, fmt.Errorf("move: reactor %q reacted with %q: %w: no display name",
				reaction.ReactorID, reaction.ConditionRef, ErrInvalidWorld)
		}
		// NO PRESENTATION TOKEN: a reaction is a roll the server took inside
		// somebody else's step, so no client simulated its die.
		beat := recordStrike(
			reaction.ReactorID, reaction.Against, reaction.Struck,
			AttackRef{Ref: reaction.AttackRef.String(), Name: reaction.AttackName, DamageType: DamageType(reaction.DamageType)}, "",
			own.Checks, own.Breaks,
		)
		// What the beat was taken AS — the only thing that explains why a
		// fighter dealt damage on a skeleton's turn.
		beat.Reaction = &encounter.ReactionIdentity{Ref: reaction.ConditionRef, Name: reaction.ConditionName}
		tells = append(tells, told{beat: beat, retaliation: reaction.Struck.Retaliation})
	}
	return func(enc *encounter.Encounter, _ concentration) error {
		for _, t := range tells {
			if t.beat != nil {
				if _, err := enc.Record(t.beat); err != nil {
					return err
				}
			}
			if err := recordRetaliation(enc, t.retaliation, t.continued); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

// recordRetaliation records the save and effects a taken post-hit reaction
// imposed, on enc, with the concentration its damage tested. Nil records
// nothing.
func recordRetaliation(enc *encounter.Encounter, r *resolution.RetaliationOutcome, told concentration) error {
	if r == nil {
		return nil
	}
	ref := SpellRef{Ref: r.Offer.Ref.String(), Name: r.Offer.Name}
	save, err := castSave(resolution.CastTargetOutcome{TargetID: r.TargetID, Save: &r.Result}, r.Offer.Ref)
	if err != nil {
		return err
	}
	results := make([]encounter.ActivationResult, 0, len(r.Result.Imposed))
	for _, applied := range r.Result.Imposed {
		result, e := imposedResult(applied, ref)
		if e != nil {
			return e
		}
		results = append(results, result)
	}
	_, err = enc.RecordActivation(&encounter.RecordActivationInput{
		Actor: encounter.MemberID(r.Offer.ReactorID), Target: encounter.MemberID(r.TargetID),
		Ability: encounter.ActivationIdentity{Ref: ref.Ref, Name: ref.Name}, Save: save, Results: results,
		ConcentrationChecks: told.Checks, ConcentrationBreaks: told.Breaks,
	})
	return translate(err)
}

// openWindow poses a pause through [poseWindow] and tells the roll-window
// beat that asks, where the story has one: a player's own attack paused on
// its d20, and a cast's save offered a die with no choices. Every other
// window is told by the composition's own beats.
func openWindow(
	enc *encounter.Encounter, scope *writeScope, pause resolution.Pause, story windowStory,
) (*encounter.RollWindowOutput, error) {
	if err := poseWindow(scope, pause, story); err != nil {
		return nil, err
	}
	ask := pause.Ask
	offer := encounter.ReactionIdentity{Ref: ask.Offer.Ref.String(), Name: ask.Offer.Name}
	switch {
	case story.Kind == storyAttack && pause.Kind == resolution.PausePostRoll:
		// The presentation id the declaring client has been correlating its
		// throw against since before the dice fell.
		return enc.RecordRollWindow(&encounter.RollWindowInput{
			PresentationID: story.PresentationID,
			Audience:       encounter.MemberID(ask.Audience),
			Offer:          offer,
			Roll:           ask.Roll,
			Total:          ask.Total,
		})
	case story.Kind == storyCast && len(ask.Offer.Choices) == 0:
		return enc.RecordRollWindow(&encounter.RollWindowInput{
			Audience: encounter.MemberID(ask.Audience),
			Offer:    offer,
			Roll:     ask.Roll,
			Total:    ask.Total,
		})
	default:
		return nil, nil
	}
}

// answerCheck is the check story's resume and landing: the check is finished
// through [resolution.ResumeCheck] over the checker's CURRENT sheet, and the
// verb it was for lands its own beat — Unlock for a door, the social verb for
// a target — exactly as the unpaused verb does. Nothing is re-rolled and the
// action, spent before the question, is not charged again.
func (m *Manager) answerCheck(
	ctx context.Context, scope *writeScope, w pendingWindow, answer resolution.Answer,
) (*ReactOutput, error) {
	data, err := m.sheetsFor(nil).load(ctx, "member", w.Audience)
	if err != nil {
		return nil, err
	}
	out, err := resolution.ResumeCheck(ctx, &resolution.CheckResumeInput{
		Pause:     w.Pause,
		Answer:    answer,
		Character: data,
		Roller:    &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, translateResolution(err)
	}
	if out.Posed != nil {
		return nil, fmt.Errorf("%w: a resumed check asked again", ErrInvalidWorld)
	}
	if out.DirtyCharacter != nil {
		if err := m.sheetsFor(scope).save(ctx, out.DirtyCharacter); err != nil {
			return nil, err
		}
	}
	// THE RESUMED HALF APPENDS THE SAME BEATS the unposed path does, so it
	// fails closed the same way on a check that lost its arithmetic.
	if err := requireCalculation("react", w.Audience, rollCalculationFor(out.Calculation)); err != nil {
		return nil, err
	}
	if w.Story.Door != "" {
		if _, err := scope.enc.Unlock(&encounter.UnlockInput{
			Door:        w.Story.Door,
			Beaten:      out.Result.Success,
			Actor:       encounter.MemberID(w.Audience),
			Total:       out.Result.Total,
			Applied:     out.Applied,
			Calculation: rollCalculationFor(out.Calculation),
		}); err != nil {
			return nil, translate(err)
		}
	} else if err := m.landResumedSocial(ctx, scope, w.Story, w.Audience, out); err != nil {
		return nil, err
	}
	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, err
	}
	return &ReactOutput{Saved: report, Delivery: delivery}, nil
}

// landResumedSocial finishes the social verb a check-offer window paused,
// through the same composition op the unpaused path uses.
//
// THE STORY SAYS WHICH VERB, and this switch is why it has to
// ([windowStory.Verb]). While Intimidate was the only social verb,
// "a target and no door" named it unambiguously; with two, a resumed Persuade
// that landed an Intimidate would put the wrong deed on a mind — the coward
// would take fear from a conversation it was talked round by. The window is
// refused at the trust boundary when it names neither.
//
// NOTHING IS RE-ROLLED. The verdict handed in is the resumed machine's, offer
// included; this only carries it across the seam. The action, if one was
// spent, was spent before the question was asked.
func (m *Manager) landResumedSocial(
	ctx context.Context, scope *writeScope, story windowStory, audience string, out *resolution.CheckOutput,
) error {
	landing := &socialLanding{
		Actor:  encounter.MemberID(audience),
		Target: encounter.MemberID(story.Target),
		Beaten: out.Result.Success,
		DC:     out.Applied.DC,
		Total:  out.Result.Total,
		// The RESUMED calculation, which is the pre-offer one plus whatever
		// the answer added — never the frozen one the window asked with. The
		// caller refused a nil one before reaching either branch.
		Calculation: rollCalculationFor(out.Calculation),
	}

	var spec socialVerb
	switch story.Verb {
	case VerbIntimidate:
		spec = m.intimidateVerb()
	case VerbPersuade:
		spec = m.persuadeVerb()
	default:
		// Unreachable: thawWindow refuses a target under any other
		// verb. Refusing rather than defaulting keeps the day that stops being
		// true from silently landing a threat.
		return fmt.Errorf("%w: a paused check names target %q under verb %q",
			ErrInvalidSession, story.Target, story.Verb)
	}

	if _, _, err := spec.land(ctx, scope.enc, landing); err != nil {
		return translate(err)
	}

	return nil
}

// recordStrike builds the record one swing produces, from the identities a
// caller holds rather than from a compiled definition, including everything
// the swing's concentration consequences ride in on.
//
// It takes identities because a reaction's swing and a resumed swing have no
// compiled offer to read a definition off, and reconstructing one to recover
// two strings would be a second answer to what was swung. [recordFor] is this
// with an AttackInput and a definition in front of it.
//
// # The breaks and checks are PASSED THROUGH, not built here
//
// Resolution hands over [encounter.ConcentrationCheck] and
// [encounter.ConcentrationBreak] already assembled — the rolls, the reasons,
// the stripped addresses and the names they go out under. This seam copies two
// slice headers onto the record and knows nothing about what is in them, which
// is the whole of Kirk's ruling made structural: *"resolution is the place
// that should resolve things. it shouldn't have to leak out."* An earlier draft
// of this file projected those shapes here, and every line of it was this seam
// having an opinion about a rule.
//
// They are supplied on EVERY strike path — a player's swing, a monster's, and
// a resumed one — because they are written here rather than at the three call
// sites. A path that assembled its own record would be a path where a
// defender's concentration silently survives.
func recordStrike(
	attacker, target string, struck resolution.StrikeOutcome, ref AttackRef, presentationID string,
	checks []encounter.ConcentrationCheck, breaks []encounter.ConcentrationBreak,
) *encounter.RecordInput {
	if struck.Warded != nil {
		// NO PresentationID and NO top-level Calculation: both are refused
		// by encounter.Record on every kind but Struck/Missed, because both
		// describe an attack roll that never happened here — the ward
		// stopped this swing before the d20. The warding save's OWN
		// calculation still rides inside WardedDetail.Save, a different
		// field with no such restriction.
		return &encounter.RecordInput{
			Kind:    encounter.OutcomeWarded,
			Actor:   encounter.MemberID(attacker),
			Targets: []encounter.MemberID{encounter.MemberID(target)},
			Attack: &encounter.AttackIdentity{
				Ref: ref.Ref, Name: ref.Name, DamageType: string(ref.DamageType),
			},
			Warded: &encounter.WardedDetail{
				Source: encounter.MemberID(struck.Warded.SourceID),
				Save: encounter.CastSave{
					Saver:       encounter.MemberID(attacker),
					Ability:     string(struck.Warded.Ability),
					Roll:        struck.Warded.Save.Roll,
					Total:       struck.Warded.Save.Total,
					DC:          struck.Warded.Save.DC,
					Calculation: rollCalculationFor(struck.Warded.Save.Calculation),
					Succeeded:   struck.Warded.Save.Success,
				},
			},
			ConcentrationChecks: checks,
			ConcentrationBreaks: breaks,
		}
	}

	values := map[encounter.OutcomeValue]int{
		encounter.ValueRoll:    struck.Roll,
		encounter.ValueTotal:   struck.Total,
		encounter.ValueAgainst: struck.TargetAC,
	}
	kind := encounter.OutcomeMissed
	if struck.Hit {
		kind = encounter.OutcomeStruck
		values[encounter.ValueAmount] = struck.Damage
	}

	recorded := &encounter.RecordInput{
		Kind:     kind,
		Actor:    encounter.MemberID(attacker),
		Targets:  []encounter.MemberID{encounter.MemberID(target)},
		Values:   values,
		Critical: struck.Critical,
		Attack: &encounter.AttackIdentity{
			Ref: ref.Ref, Name: ref.Name, DamageType: string(ref.DamageType),
		},

		PresentationID: presentationID,
		Calculation:    rollCalculationFor(struck.Calculation),
	}
	if struck.Hit {
		recorded.DamageComponents = recordDamageComponents(struck.DamageComponents)
		// The fold's attribution is NOT recorded beside the dice any more. It
		// arrives inside Calculation, on the d20 component's keep record, which
		// is the pool it actually describes — a list beside it could disagree
		// with it, and this seam wrote both (rpg-project#462 R1).
	}

	// Set outside the Hit arm on purpose: whether a miss can end or test a
	// concentration is a rulebook fact, and resolution answers it by handing
	// over empty lists. A guard here would be this seam deciding it.
	recorded.ConcentrationChecks = checks
	recorded.ConcentrationBreaks = breaks
	return recorded
}
