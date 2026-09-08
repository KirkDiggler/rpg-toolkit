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

	// Succeeded is whether the total beat the DC. It is the CASTER'S ruling,
	// recorded rather than recomputed: Total >= DC is the ordinary rule and
	// this composition does not own the exceptions to it.
	Succeeded bool
}

// RecordCastInput is one cast transaction: the caster and the spell, an
// optional named target, the save its gate produced if it had one, and zero or
// more delivered effects in the exact synchronous order the rulebook produced
// them.
type RecordCastInput struct {
	Actor  MemberID
	Target MemberID
	Spell  SpellIdentity

	// Save is the gate's saving throw, or nil for a cast that had no gate.
	// NIL IS THE HONEST ZERO — True Strike delivers a condition and rolls
	// nothing, and a save beat reading 0 against DC 0 would say a roll
	// happened that never did.
	Save *CastSave

	// Results are the effects actually delivered, in order. A successful save
	// against Vicious Mockery delivers none, and that is a complete cast.
	Results []ActivationResult
}

// RecordCastOutput reports where every transaction beat landed and any intel
// changes produced while noticing post-transaction consequences.
type RecordCastOutput struct {
	Seqs        []uint64
	IntelDeltas map[MemberID]*IntelDelta
}

type castPayload struct {
	Beat   string               `json:"beat"`
	Actor  MemberID             `json:"actor"`
	Spell  spellIdentityPayload `json:"spell"`
	Target MemberID             `json:"target,omitempty"`
}

type spellIdentityPayload struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

type savedPayload struct {
	Beat      string               `json:"beat"`
	Saver     MemberID             `json:"saver"`
	Ability   string               `json:"ability"`
	Roll      int                  `json:"roll"`
	Total     int                  `json:"total"`
	DC        int                  `json:"dc"`
	Succeeded bool                 `json:"succeeded"`
	Source    spellIdentityPayload `json:"source"`
}

// RecordCast appends one cast beat, then the saved beat if the spell's gate
// rolled one, then one activation-result beat per delivered effect, preserving
// result order. The entire input and every payload are validated before the
// first append, so an input rejection cannot leave a partial transaction in
// the story.
//
// # It is RecordActivation's sibling and reuses its results
//
// The delivered effects go out as the SAME activation-result beat carrying the
// SAME closed result kinds, because a condition applied by a spell and a
// condition applied by a feature are the same fact in the story and every host
// that already reads one should read the other with no new code. What a spell
// adds is the two beats above them: the cast itself, and the save that decided
// whether anything followed.
//
// # It does not refuse a paused encounter
//
// RecordActivation does not either. A composition's pause is a driven turn's
// remainder — a monster mid-walk — and a cast recorded while one is held is a
// post-roll consequence being narrated, exactly what RecordRollWindow exists
// for. A record verb narrates; only verbs that move the clock refuse a pause
// (see EndTurn's ErrTurnPaused).
//
// AUDIENCE IS EVERYONE for all three shapes, the pre-v1 full-data rule every
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
// named target, empty or unknown saver, or empty/unknown result target),
// ErrInvalidData (missing spell identity, a save with no ability or a roll
// that is not a d20, unknown result kind, a missing/forbidden kind field, or a
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
	if in.Target != "" {
		if _, ok := e.members[in.Target]; !ok {
			return nil, fmt.Errorf("record cast: target %q: %w", in.Target, ErrNoMember)
		}
	}
	if in.Spell.Ref == "" {
		return nil, fmt.Errorf("record cast: spell ref: %w", ErrInvalidData)
	}
	if in.Spell.Name == "" {
		return nil, fmt.Errorf("record cast: spell name: %w", ErrInvalidData)
	}
	spell := spellIdentityPayload{Ref: in.Spell.Ref, Name: in.Spell.Name}

	castBytes, err := json.Marshal(castPayload{
		Beat:   BeatCast,
		Actor:  in.Actor,
		Spell:  spell,
		Target: in.Target,
	})
	if err != nil {
		return nil, fmt.Errorf("record cast: cast payload: %w", err)
	}
	castSubjects := []MemberID{in.Actor}
	if in.Target != "" {
		castSubjects = append(castSubjects, in.Target)
	}

	prepared := make([]preparedActivationBeat, 0, len(in.Results)+2)
	prepared = append(prepared, preparedActivationBeat{
		payload:  castBytes,
		subjects: castSubjects,
	})

	if in.Save != nil {
		savedBytes, savedSubjects, saveErr := e.prepareCastSave(in.Actor, in.Save, spell)
		if saveErr != nil {
			return nil, saveErr
		}
		prepared = append(prepared, preparedActivationBeat{
			payload:  savedBytes,
			subjects: savedSubjects,
		})
	}

	for i, result := range in.Results {
		resultPayload, validationErr := e.prepareActivationResult("record cast", i, result)
		if validationErr != nil {
			return nil, validationErr
		}
		resultBytes, marshalErr := json.Marshal(activationResultPayload{
			Beat:   "activation-result",
			Actor:  in.Actor,
			Result: resultPayload,
		})
		if marshalErr != nil {
			return nil, fmt.Errorf("record cast: result %d payload: %w", i, marshalErr)
		}
		prepared = append(prepared, preparedActivationBeat{
			payload:  resultBytes,
			subjects: []MemberID{in.Actor, result.Target},
		})
	}

	return prepared, nil
}

func (e *Encounter) prepareCastSave(
	actor MemberID, save *CastSave, spell spellIdentityPayload,
) ([]byte, []MemberID, error) {
	if save.Saver == "" {
		return nil, nil, fmt.Errorf("record cast: save saver: %w", ErrNoMember)
	}
	if _, ok := e.members[save.Saver]; !ok {
		return nil, nil, fmt.Errorf("record cast: save saver %q: %w", save.Saver, ErrNoMember)
	}
	if save.Ability == "" {
		return nil, nil, fmt.Errorf("record cast: save ability: %w", ErrInvalidData)
	}
	if save.Roll < 1 || save.Roll > 20 {
		// The beat exists so a player can read the save that was made. A d20
		// that does not read 1-20 is not a save anybody rolled.
		return nil, nil, fmt.Errorf("record cast: save roll %d is not a d20: %w", save.Roll, ErrInvalidData)
	}
	if save.DC < 1 {
		// A DC of zero is not a difficulty; it is a field nobody filled in.
		return nil, nil, fmt.Errorf("record cast: save dc %d: %w", save.DC, ErrInvalidData)
	}

	savedBytes, err := json.Marshal(savedPayload{
		Beat:      BeatSaved,
		Saver:     save.Saver,
		Ability:   save.Ability,
		Roll:      save.Roll,
		Total:     save.Total,
		DC:        save.DC,
		Succeeded: save.Succeeded,
		Source:    spell,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("record cast: saved payload: %w", err)
	}

	subjects := []MemberID{actor}
	if save.Saver != actor {
		subjects = append(subjects, save.Saver)
	}
	return savedBytes, subjects, nil
}
