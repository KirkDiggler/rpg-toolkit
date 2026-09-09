// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// BeatCast is the "beat" value of the story beat this composition appends when
// somebody casts a spell.
//
// It is a SECOND beat rather than a widening of the activated beat, for the
// reason a spell is not a feature: the activated beat names an ability, and
// the two travel to the client as different bodies with different refs. A
// player reading their own log is owed "you cast Vicious Mockery", not "you
// activated it".
const BeatCast = "cast"

// BeatSaved is the "beat" value of the story beat this composition appends
// when a target rolls a saving throw against a cast.
//
// It is its own beat and not a reuse of the death save's, whose every other
// field — stabilized, dead, hp restored, the continuation — would read zero on
// an ordinary save and lie about what happened. What a save is, is a roll, a
// DC and an answer.
const BeatSaved = "saved"

// BeatConcentrationEnded is the "beat" value of the story beat this
// composition appends when a member's concentration on a spell ends.
//
// It is its own beat rather than an inference from the condition-removed
// results that follow it, for two reasons the log makes obvious. Three of the
// break's reasons — a duration running out, the fight ending, the spell's own
// children being spent — produce no roll and no check, so there is nothing
// else in the train that could be read as the break. And the removals land on
// OTHER members' sheets, where a reader with no break beat in front of them
// sees conditions dropping off strangers for no stated cause.
//
// There is no "concentration started" beat, deliberately: the cast beat
// already is one.
const BeatConcentrationEnded = "concentration_ended"

// ConcentrationBreak is one concentration that ended as a consequence of the
// interaction being recorded — the caster, the spell they lose, why it broke,
// the check they failed if there was one, and the conditions stripped from the
// board when it went.
//
// IT RIDES THE INTERACTION THAT CAUSED IT rather than arriving through a
// record verb of its own. A concentration break is not something that happens
// on its own account: it happens because somebody was hit, or cast again, or
// the fight ended. Recording it separately would put the break in the story at
// whatever clock reading the second call happened to reach, next to none of
// the beats that explain it, and a reader scrolling their own log would find
// "your spell ended" with the blow that ended it somewhere else entirely.
// Carried here, one hit produces one train — struck, saved,
// concentration_ended, and one condition-removed per address — in the order
// the rulebook produced them, appended by the call the caller already makes.
type ConcentrationBreak struct {
	// Caster is the member who was concentrating and now is not. Must be a
	// current member.
	Caster MemberID

	// Spell is the spell that ends. Required: a break beat that cannot say
	// what was lost is not a beat anybody can read.
	Spell SpellIdentity

	// Reason is why it ended, in the rulebook's own words — "damage",
	// "recast", "duration", "combat_end", "spell_ended", "caster_down".
	//
	// CHECKED FOR PRESENCE, NOT FOR MEANING, and a plain string rather than a
	// closed enum here, for the same C1 reason [ActivationResult.DamageType]
	// is one: the vocabulary belongs to the rulebook this module cannot
	// import. Required, because a break with no stated cause is the exact
	// thing this beat exists to prevent.
	Reason string

	// Save is the check the caster failed to keep the spell, or nil when the
	// break was ungated.
	//
	// NIL IS THE HONEST ZERO, the same as [RecordCastInput.Save]. A caster
	// dropped to zero hit points loses concentration with no roll at all, and
	// so does one whose spell simply ran out of duration — a save beat reading
	// 0 against DC 0 beside either would say a roll happened that never did.
	Save *CastSave

	// Removed are the conditions the ending spell took off the board, in the
	// order the rulebook stripped them, one per address it was holding. Each
	// must be a [ResultConditionRemoved] — they go out as the same
	// activation-result beat every other removal in this module uses, because
	// a condition removed by a broken concentration and a condition removed by
	// anything else are the same fact in the story.
	//
	// Empty is legal and means the spell was holding nothing when it broke.
	Removed []ActivationResult
}

// ConcentrationCheck is a concentration the caster KEPT — the check they were
// asked for and made, with nothing following it.
//
// IT EXISTS BECAUSE A ROLL THAT CHANGED NOTHING STILL HAPPENED. A bard hit for
// 9 who makes the DC 10 rolled a d20 at the table, and a record that carried
// only [ConcentrationBreak] would show the blow and no check at all — the
// player rolled and their own log did not say so. That is the record failing
// to tell the truth on the day it was written, and the failure is silent,
// which is the worst kind: nothing is missing that anybody can point at.
//
// # It is a second shape rather than a Save field that sometimes breaks things
//
// The two are disjoint by construction and the refusal below enforces it. A
// check that HELD is one of these; a check that FAILED rides the break it
// caused, in [ConcentrationBreak.Save]. The alternative on offer — one list of
// every check, with breaks following it — reads tidier and is worse, because
// it separates a failed check from the break it explains. With two members
// checking in one interaction the story would read save, save, ended,
// removals, and a reader would have to work out which of the two rolls was the
// one that lost the spell. Keeping the failed check ON its break is what makes
// every break's beats contiguous, which is the whole reason these beats ride
// the interaction at all.
//
// So the question "which list does this save go in" has exactly one answer for
// any save, and it is not a matter of taste: did the spell survive.
type ConcentrationCheck struct {
	// Spell is the spell the caster kept. Required, the same as a break's:
	// a check beat that cannot say what was at stake is not readable.
	Spell SpellIdentity

	// Save is the check, and its Saver is the concentrating caster.
	//
	// THERE IS NO SEPARATE CASTER FIELD, deliberately. The saver of a
	// concentration check IS the caster — nobody else can roll to keep
	// somebody's spell up — so a second field naming them would be a field
	// that can disagree with this one, and a zero value that lies the day
	// they do.
	//
	// Succeeded must be true. A failed concentration check ends the spell, so
	// one recorded here as having changed nothing is either a rule that did
	// not run or a break that lost its removals on the way, and both are
	// worth failing loudly for rather than writing down.
	Save CastSave
}

// SpellIdentity names the rulebook spell that was cast. Ref and Name are
// required catalog facts carried as primitives; encounter validates their
// presence without interpreting what they mean.
//
// It mirrors [ActivationIdentity] field for field and is deliberately NOT the
// same type: a spell ref and an ability ref are different catalog namespaces
// (dnd5e:spells: against dnd5e:features:), they travel to the client as
// different messages, and a call site that reads Spell: SpellIdentity{...}
// says what it is doing.
type SpellIdentity struct {
	Ref  string
	Name string
}

// CastSave is the one saving throw a cast's gate produced: who rolled, what
// they rolled with, the d20 and the number it reached, the DC it was against,
// and whether it beat it.
//
// THE ANSWER IS A BOOL and not an outcome word, because a save has exactly two
// answers. Half-on-success, and every other partial, is a property of what the
// spell then delivers — which is the result beats' business, not this one's.
type CastSave struct {
	// Saver is the member who rolled. Must be a member of this encounter.
	Saver MemberID

	// Ability is the rulebook ability the save was made with, carried as the
	// rulebook's own primitive — "wisdom" for Vicious Mockery. Required:
	// a save against nothing is not a save the table can read.
	Ability string

	// Roll is the d20 as rolled and Total the number it reached after the
	// saver's modifiers.
	Roll  int
	Total int

	// DC is the number the total was against.
	DC int

	// Calculation is the authoritative sourced arithmetic. Roll and Total are
	// retained summaries and must agree with its d20 component and total.
	Calculation *RollCalculation

	// Succeeded is the rulebook's ruling. Encounter records it without
	// recomputing success policy from Total and DC.
	Succeeded bool
}

// CastTargetResult is one member named by a cast, the save they rolled when
// gated, and the effects delivered to that target transaction. Results may
// name another recipient (for example the caster); caller target order remains
// the ordering authority.
type CastTargetResult struct {
	Target  MemberID
	Save    *CastSave
	Results []ActivationResult
}

// RecordCastInput is one cast transaction: one caster, one spell, and an
// ordered target-result list. Empty Targets is the honest shape for a
// self/no-target cast.
type RecordCastInput struct {
	Actor   MemberID
	Spell   SpellIdentity
	Targets []CastTargetResult

	// ConcentrationBreaks are the concentrations this cast ended, in the order
	// the rulebook ended them. Their beats are appended after the cast's own,
	// so the whole break reads inside the cast that caused it.
	//
	// A cast breaks concentration two ways and both arrive here: the caster
	// casting a second concentration spell drops the first, and a cast that
	// deals damage can break somebody else's. Empty is the ordinary case.
	ConcentrationBreaks []ConcentrationBreak

	// ConcentrationChecks are the concentrations this cast tested and did NOT
	// break, in the order they were rolled. Their saved beats are appended
	// before any break's, so a reader sees every check this cast asked for.
	ConcentrationChecks []ConcentrationCheck
}

// RecordCastOutput reports where every transaction beat landed and any intel
// changes produced while noticing post-transaction consequences.
type RecordCastOutput struct {
	Seqs        []uint64
	IntelDeltas map[MemberID]*IntelDelta
}

type castPayload struct {
	Beat    string               `json:"beat"`
	Actor   MemberID             `json:"actor"`
	Spell   spellIdentityPayload `json:"spell"`
	Targets []MemberID           `json:"targets,omitempty"`
}

type spellIdentityPayload struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

type savedPayload struct {
	Beat        string               `json:"beat"`
	Saver       MemberID             `json:"saver"`
	Ability     string               `json:"ability"`
	Roll        int                  `json:"roll"`
	Total       int                  `json:"total"`
	DC          int                  `json:"dc"`
	Succeeded   bool                 `json:"succeeded"`
	Calculation *RollCalculation     `json:"calculation"`
	Source      spellIdentityPayload `json:"source"`
}

type concentrationEndedPayload struct {
	Beat   string               `json:"beat"`
	Caster MemberID             `json:"caster"`
	Spell  spellIdentityPayload `json:"spell"`
	Reason string               `json:"reason"`
}

// RecordCast appends one cast beat naming the ordered target list, then each
// target's saved beat (when gated) and activation-result beats before moving to
// the next target. The entire input and every payload are validated before the
// first append, so an input rejection cannot leave a partial transaction in
// the story.
//
// # It is RecordActivation's sibling and reuses its results
//
// The delivered effects go out as the SAME activation-result beat carrying the
// SAME closed result kinds, because a condition applied by a spell and a
// condition applied by a feature are the same fact in the story and every host
// that already reads one should read the other with no new code. What a spell
// adds is the cast beat ahead of the ordered per-target save/result trains.
//
// # It does not refuse a paused encounter
//
// RecordActivation does not either. A composition's pause is a driven turn's
// remainder — a monster mid-walk — and a cast recorded while one is held is a
// post-roll consequence being narrated, exactly what RecordRollWindow exists
// for. A record verb narrates; only verbs that move the clock refuse a pause
// (see EndTurn's ErrTurnPaused).
//
// AUDIENCE IS EVERYONE for every shape, the pre-v1 full-data rule every
// other beat here keeps, including the save's numbers. When per-recipient
// beats arrive (rpg-toolkit#940) that becomes a beatClass rather than a
// special case.
//
// noticeDown runs exactly once after all transaction beats, never between
// them, so a damaged member who drops is noticed once for the whole cast. A
// noticeDown error leaves the complete transaction appended in memory and
// returns no output; doc.go's caller rule applies: discard the encounter
// unsaved.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember (empty or unknown actor, unknown
// listed target, empty or unknown saver, or empty/unknown result target),
// ErrInvalidData (duplicate targets, a target/save mismatch, missing spell
// identity, a save with no ability or authoritative calculation, a roll that
// is not a d20, unknown result kind, a missing/forbidden kind field, or a
// healing or damage whose calculation is absent, structurally inconsistent, or
// whose Total does not equal the requested amount), an append error, or
// anything the Standing capability returns from noticeDown.
func (e *Encounter) RecordCast(in *RecordCastInput) (*RecordCastOutput, error) {
	prepared, err := e.prepareCast(in)
	if err != nil {
		return nil, err
	}

	at := uint64(e.clock.ToData().HighWater)
	seqs := make([]uint64, 0, len(prepared))
	for i, beat := range prepared {
		appended, appendErr := e.appendBeat(&record.AppendInput{
			At:       at,
			Audience: e.audienceFor(subjectBeat, beat.subjects...),
			Tags:     map[string]string{"tag": "outcome"},
			Payload:  beat.payload,
		})
		if appendErr != nil {
			return nil, fmt.Errorf("record cast: append beat %d: %w", i, appendErr)
		}
		seqs = append(seqs, appended.Seq)
	}

	_, intelDeltas, noticeErr := e.noticeDown()
	if noticeErr != nil {
		return nil, fmt.Errorf("record cast: %w", noticeErr)
	}

	return &RecordCastOutput{Seqs: seqs, IntelDeltas: intelDeltas}, nil
}

// prepareCast validates and marshals the complete transaction before the
// caller appends any of it. It is the validation/mutation boundary for
// RecordCast, not merely a convenience split.
func (e *Encounter) prepareCast(in *RecordCastInput) ([]preparedActivationBeat, error) {
	if in == nil {
		return nil, fmt.Errorf("record cast: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("record cast: %w", ErrClosed)
	}
	if in.Actor == "" {
		return nil, fmt.Errorf("record cast: actor: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Actor]; !ok {
		return nil, fmt.Errorf("record cast: actor %q: %w", in.Actor, ErrNoMember)
	}
	targets := make([]MemberID, len(in.Targets))
	seenTargets := make(map[MemberID]struct{}, len(in.Targets))
	for i, target := range in.Targets {
		if target.Target == "" {
			return nil, fmt.Errorf("record cast: target %d: %w", i, ErrNoMember)
		}
		if _, ok := e.members[target.Target]; !ok {
			return nil, fmt.Errorf("record cast: target %d %q: %w", i, target.Target, ErrNoMember)
		}
		if _, duplicate := seenTargets[target.Target]; duplicate {
			return nil, fmt.Errorf("record cast: target %d %q is duplicated: %w", i, target.Target, ErrInvalidData)
		}
		seenTargets[target.Target] = struct{}{}
		targets[i] = target.Target
	}
	if in.Spell.Ref == "" {
		return nil, fmt.Errorf("record cast: spell ref: %w", ErrInvalidData)
	}
	if in.Spell.Name == "" {
		return nil, fmt.Errorf("record cast: spell name: %w", ErrInvalidData)
	}
	spell := spellIdentityPayload{Ref: in.Spell.Ref, Name: in.Spell.Name}

	castBytes, err := json.Marshal(castPayload{
		Beat: BeatCast, Actor: in.Actor, Spell: spell, Targets: targets,
	})
	if err != nil {
		return nil, fmt.Errorf("record cast: cast payload: %w", err)
	}
	castSubjects := append([]MemberID{in.Actor}, targets...)

	beatCount := 1
	for _, target := range in.Targets {
		if target.Save != nil {
			beatCount++
		}
		beatCount += len(target.Results)
	}
	prepared := make([]preparedActivationBeat, 0, beatCount)
	prepared = append(prepared, preparedActivationBeat{
		payload:  castBytes,
		subjects: castSubjects,
	})

	for targetIndex, target := range in.Targets {
		if target.Save != nil {
			savedBytes, savedSubjects, saveErr := e.prepareSaveBeat(
				fmt.Sprintf("record cast: target %d", targetIndex), in.Actor, target.Save, spell,
			)
			if saveErr != nil {
				return nil, saveErr
			}
			if target.Save.Saver != target.Target {
				return nil, fmt.Errorf(
					"record cast: target %d %q has save for %q: %w",
					targetIndex, target.Target, target.Save.Saver, ErrInvalidData,
				)
			}
			prepared = append(prepared, preparedActivationBeat{
				payload: savedBytes, subjects: savedSubjects,
			})
		}

		for resultIndex, result := range target.Results {
			resultPayload, validationErr := e.prepareActivationResult(
				fmt.Sprintf("record cast: target %d", targetIndex), resultIndex, result,
			)
			if validationErr != nil {
				return nil, validationErr
			}
			resultBytes, marshalErr := json.Marshal(activationResultPayload{
				Beat: "activation-result", Actor: in.Actor, Result: resultPayload,
			})
			if marshalErr != nil {
				return nil, fmt.Errorf(
					"record cast: target %d result %d payload: %w", targetIndex, resultIndex, marshalErr,
				)
			}
			prepared = append(prepared, preparedActivationBeat{
				payload: resultBytes, subjects: []MemberID{in.Actor, activationResultTarget(result)},
			})
		}
	}

	checkBeats, checkErr := e.prepareConcentrationChecks("record cast", in.Actor, in.ConcentrationChecks)
	if checkErr != nil {
		return nil, checkErr
	}
	prepared = append(prepared, checkBeats...)

	breakBeats, breakErr := e.prepareConcentrationBreaks("record cast", in.Actor, in.ConcentrationBreaks)
	if breakErr != nil {
		return nil, breakErr
	}
	prepared = append(prepared, breakBeats...)

	return prepared, nil
}

// prepareSaveBeat validates one saving throw and marshals its beat. verb names
// the caller in every refusal — "record cast" for a spell's own gate, "record"
// for a concentration check ridden in on a strike — because a save refused
// under the wrong verb's name sends the reader to the wrong door.
func (e *Encounter) prepareSaveBeat(
	verb string, actor MemberID, save *CastSave, spell spellIdentityPayload,
) ([]byte, []MemberID, error) {
	if save.Saver == "" {
		return nil, nil, fmt.Errorf("%s: save saver: %w", verb, ErrNoMember)
	}
	if _, ok := e.members[save.Saver]; !ok {
		return nil, nil, fmt.Errorf("%s: save saver %q: %w", verb, save.Saver, ErrNoMember)
	}
	if save.Ability == "" {
		return nil, nil, fmt.Errorf("%s: save ability: %w", verb, ErrInvalidData)
	}
	if save.Roll < 1 || save.Roll > 20 {
		// The beat exists so a player can read the save that was made. A d20
		// that does not read 1-20 is not a save anybody rolled.
		return nil, nil, fmt.Errorf("%s: save roll %d is not a d20: %w", verb, save.Roll, ErrInvalidData)
	}
	if save.DC < 1 {
		// A DC of zero is not a difficulty; it is a field nobody filled in.
		return nil, nil, fmt.Errorf("%s: save dc %d: %w", verb, save.DC, ErrInvalidData)
	}
	if err := validateRecordedD20(save.Calculation, save.Roll, save.Total); err != nil {
		return nil, nil, fmt.Errorf("%s: save calculation: %v: %w", verb, err, ErrInvalidData)
	}

	savedBytes, err := json.Marshal(savedPayload{
		Beat:        BeatSaved,
		Saver:       save.Saver,
		Ability:     save.Ability,
		Roll:        save.Roll,
		Total:       save.Total,
		DC:          save.DC,
		Succeeded:   save.Succeeded,
		Calculation: save.Calculation,
		Source:      spell,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("%s: saved payload: %w", verb, err)
	}

	subjects := []MemberID{actor}
	if save.Saver != actor {
		subjects = append(subjects, save.Saver)
	}
	return savedBytes, subjects, nil
}

// prepareConcentrationChecks validates and marshals one saved beat per check
// that held. verb names the caller in every refusal.
//
// actor is the member whose interaction asked for the checks, and is a subject
// of each beat for the same reason it is a subject of a break's: the roll
// happened because of what they did.
func (e *Encounter) prepareConcentrationChecks(
	verb string, actor MemberID, checks []ConcentrationCheck,
) ([]preparedActivationBeat, error) {
	if len(checks) == 0 {
		return nil, nil
	}

	prepared := make([]preparedActivationBeat, 0, len(checks))
	for i, check := range checks {
		if check.Spell.Ref == "" {
			return nil, fmt.Errorf("%s: concentration check %d spell ref: %w", verb, i, ErrInvalidData)
		}
		if check.Spell.Name == "" {
			return nil, fmt.Errorf("%s: concentration check %d spell name: %w", verb, i, ErrInvalidData)
		}
		// The one invariant that keeps the two lists disjoint. A failed
		// concentration check ends the spell, so a failure recorded as having
		// changed nothing is a break that went missing — and a missing break
		// is exactly the silent wrong this shape exists to prevent.
		if !check.Save.Succeeded {
			return nil, fmt.Errorf(
				"%s: concentration check %d did not succeed and so is a break, not a check: %w",
				verb, i, ErrInvalidData,
			)
		}

		spell := spellIdentityPayload{Ref: check.Spell.Ref, Name: check.Spell.Name}
		savedBytes, savedSubjects, saveErr := e.prepareSaveBeat(
			fmt.Sprintf("%s: concentration check %d", verb, i), actor, &checks[i].Save, spell,
		)
		if saveErr != nil {
			return nil, saveErr
		}
		prepared = append(prepared, preparedActivationBeat{
			payload:  savedBytes,
			subjects: savedSubjects,
		})
	}

	return prepared, nil
}

// prepareConcentrationBreaks validates and marshals every beat the supplied
// breaks produce, in train order: the failed check if there was one, then the
// break itself, then one condition-removed result per address the spell was
// holding. verb names the caller in every refusal.
//
// It marshals and returns rather than appending, so a break refused for a
// missing reason or an unknown caster costs the caller nothing — the whole
// transaction is still validated before its first beat lands, which is the
// property that keeps a rejected input from leaving half a story behind.
//
// actor is the member whose interaction caused the breaks — the striker, or
// the caster of the spell being recorded — and NOT the concentrating member.
// It is a subject of every beat here because the break is a consequence of
// what they did, and a reader following their own log has to be able to see
// what their blow ended.
func (e *Encounter) prepareConcentrationBreaks(
	verb string, actor MemberID, breaks []ConcentrationBreak,
) ([]preparedActivationBeat, error) {
	if len(breaks) == 0 {
		return nil, nil
	}

	prepared := make([]preparedActivationBeat, 0, len(breaks)*2)
	for i, broken := range breaks {
		if broken.Caster == "" {
			return nil, fmt.Errorf("%s: concentration break %d caster: %w", verb, i, ErrNoMember)
		}
		if _, ok := e.members[broken.Caster]; !ok {
			return nil, fmt.Errorf(
				"%s: concentration break %d caster %q: %w", verb, i, broken.Caster, ErrNoMember,
			)
		}
		if broken.Spell.Ref == "" {
			return nil, fmt.Errorf("%s: concentration break %d spell ref: %w", verb, i, ErrInvalidData)
		}
		if broken.Spell.Name == "" {
			return nil, fmt.Errorf("%s: concentration break %d spell name: %w", verb, i, ErrInvalidData)
		}
		if broken.Reason == "" {
			return nil, fmt.Errorf("%s: concentration break %d reason: %w", verb, i, ErrInvalidData)
		}
		spell := spellIdentityPayload{Ref: broken.Spell.Ref, Name: broken.Spell.Name}

		if broken.Save != nil {
			savedBytes, savedSubjects, saveErr := e.prepareSaveBeat(verb, actor, broken.Save, spell)
			if saveErr != nil {
				return nil, saveErr
			}
			prepared = append(prepared, preparedActivationBeat{
				payload:  savedBytes,
				subjects: savedSubjects,
			})
		}

		endedBytes, marshalErr := json.Marshal(concentrationEndedPayload{
			Beat:   BeatConcentrationEnded,
			Caster: broken.Caster,
			Spell:  spell,
			Reason: broken.Reason,
		})
		if marshalErr != nil {
			return nil, fmt.Errorf("%s: concentration break %d payload: %w", verb, i, marshalErr)
		}
		endedSubjects := []MemberID{actor}
		if broken.Caster != actor {
			endedSubjects = append(endedSubjects, broken.Caster)
		}
		prepared = append(prepared, preparedActivationBeat{
			payload:  endedBytes,
			subjects: endedSubjects,
		})

		for j, removed := range broken.Removed {
			// ONE KIND ONLY. Every other result kind carries something a
			// removal cannot mean — an amount, a description, a fresh
			// condition — and a break that could smuggle one in would let a
			// caller write damage into a beat whose whole claim is that
			// nothing was rolled.
			if removed.Kind != ResultConditionRemoved {
				return nil, fmt.Errorf(
					"%s: concentration break %d removed %d kind %q: %w",
					verb, i, j, removed.Kind, ErrInvalidData,
				)
			}
			resultPayload, validationErr := e.prepareActivationResult(
				fmt.Sprintf("%s: concentration break %d", verb, i), j, removed,
			)
			if validationErr != nil {
				return nil, validationErr
			}
			resultBytes, resultErr := json.Marshal(activationResultPayload{
				Beat:   "activation-result",
				Actor:  actor,
				Result: resultPayload,
			})
			if resultErr != nil {
				return nil, fmt.Errorf(
					"%s: concentration break %d removed %d payload: %w", verb, i, j, resultErr,
				)
			}
			prepared = append(prepared, preparedActivationBeat{
				payload:  resultBytes,
				subjects: []MemberID{actor, activationResultTarget(removed)},
			})
		}
	}

	return prepared, nil
}
