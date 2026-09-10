// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// ActivationIdentity names the rulebook ability that was activated. Ref and
// Name are required catalog facts carried as primitives; encounter validates
// their presence without interpreting what they mean.
type ActivationIdentity struct {
	Ref  string
	Name string
}

// ActivationResultKind names one closed result shape an activation may record.
type ActivationResultKind string

const (
	// ResultHealingApplied records actual healing after the owning sheet has
	// applied any rulebook clamp.
	ResultHealingApplied ActivationResultKind = "healing-applied"

	// ResultConditionApplied records a condition added to a member.
	ResultConditionApplied ActivationResultKind = "condition-applied"

	// ResultConditionRemoved records a condition removed from a member.
	ResultConditionRemoved ActivationResultKind = "condition-removed"

	// ResultCapacityGranted records rulebook-authored capacity granted to a
	// member, such as additional movement.
	ResultCapacityGranted ActivationResultKind = "capacity-granted"

	// ResultDamageApplied records damage actually taken after the owning
	// sheet has applied any rulebook clamp.
	//
	// It is SHAPED EXACTLY LIKE healing and validated by the same law, down
	// to the required calculation whose Total equals Requested — a cantrip's
	// 1d4 is the same kind of fact as a Second Wind's 1d10, and the player is
	// owed the face either way. It lives with the other result kinds rather
	// than in the cast's own file because a result is a result: a feature
	// that deals damage records it through [Encounter.RecordActivation] and a
	// spell that deals damage records it through [Encounter.RecordCast],
	// through this one closed family.
	ResultDamageApplied ActivationResultKind = "damage-applied"
)

// ConditionAddress is the exact neutral identity of one condition on one
// member. SourceID is exact identity: empty names only an unqualified legacy
// condition and is never a wildcard.
type ConditionAddress struct {
	MemberID     MemberID `json:"member_id"`
	ConditionRef string   `json:"condition_ref"`
	SourceID     string   `json:"source_id"`
}

// ActivationResult carries one result from a successful activation using only
// primitives this composition can persist without importing the root D&D event
// types that own the rule meaning.
//
// Every kind names a recipient: condition kinds require Address while the
// others require Target. Damage also requires DamageType, which every other
// kind refuses. Healing and damage also require Ref and Name,
// carry the amount/requested/HP facts, and require a non-nil
// [RollCalculation] whose Total equals Requested — the roll trace IS the
// roll record, so one without it is not writable. Condition-applied requires
// Address and Name. Condition-removed requires Address, Name, and Reason.
// Capacity-granted
// requires Description. Fields outside a kind's shape — including a
// calculation on any non-healing kind — are refused rather than silently
// discarded.
//
// Numeric values are rulebook facts. Encounter validates the calculation's
// internal arithmetic and its pairing with Requested, and never checks or
// recomputes the rulebook's clamp: Amount, Before, and After are preserved
// exactly as supplied.
type ActivationResult struct {
	Kind ActivationResultKind

	// Target is the recipient for non-condition results. Condition results use
	// Address so member, condition ref, and exact source identity cannot drift.
	Target  MemberID
	Address *ConditionAddress
	Ref     string
	Name    string

	Amount    int
	Requested int
	Before    int
	After     int

	// Calculation carries the sourced dice, ordered rerolls, modifiers, and
	// authoritative total behind a healing's or a damage's roll. Required for
	// ResultHealingApplied and ResultDamageApplied with Calculation.Total ==
	// Requested; forbidden on every other kind.
	Calculation *RollCalculation

	// DamageType is the rulebook damage type carried as the rulebook's own
	// primitive — "psychic" for Vicious Mockery. Required for
	// ResultDamageApplied and forbidden on every other kind.
	//
	// ONE TYPE FOR THE WHOLE RESULT, not one per component. Every damage this
	// stack deals today comes out of a single pool, and a per-component type
	// would be a shape invented for a use case nobody has: future-proofing
	// goes both ways, and the wrong guess is as expensive as no guess. A
	// spell that deals two types deals them as two results.
	DamageType string

	Description string
	Reason      string
}

// RecordActivationInput is one successful activation transaction: the actor
// and ability, an optional selected target, and zero or more results in the
// exact synchronous order the rulebook produced them.
type RecordActivationInput struct {
	Actor   MemberID
	Target  MemberID
	Ability ActivationIdentity
	Results []ActivationResult
}

// RecordActivationOutput reports where every transaction beat landed and any
// intel changes produced while noticing post-transaction consequences.
type RecordActivationOutput struct {
	Seqs        []uint64
	IntelDeltas map[MemberID]*IntelDelta
}

type preparedActivationBeat struct {
	payload  []byte
	subjects []MemberID
}

type activatedPayload struct {
	Beat    string                    `json:"beat"`
	Actor   MemberID                  `json:"actor"`
	Ability activationIdentityPayload `json:"ability"`
	Target  MemberID                  `json:"target,omitempty"`
}

type activationIdentityPayload struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

type activationResultPayload struct {
	Beat   string      `json:"beat"`
	Actor  MemberID    `json:"actor"`
	Result interface{} `json:"result"`
}

type healingAppliedPayload struct {
	Kind        ActivationResultKind `json:"kind"`
	Target      MemberID             `json:"target"`
	Amount      int                  `json:"amount"`
	Requested   int                  `json:"requested"`
	Before      int                  `json:"before"`
	After       int                  `json:"after"`
	Calculation *RollCalculation     `json:"calculation"`
	Ref         string               `json:"ref"`
	Name        string               `json:"name"`
}

type damageAppliedPayload struct {
	Kind        ActivationResultKind `json:"kind"`
	Target      MemberID             `json:"target"`
	Amount      int                  `json:"amount"`
	Requested   int                  `json:"requested"`
	Before      int                  `json:"before"`
	After       int                  `json:"after"`
	Calculation *RollCalculation     `json:"calculation"`
	Ref         string               `json:"ref"`
	Name        string               `json:"name"`
	DamageType  string               `json:"damage_type"`
}

type conditionAppliedPayload struct {
	Kind     ActivationResultKind `json:"kind"`
	Target   MemberID             `json:"target"`
	Ref      string               `json:"ref"`
	Name     string               `json:"name"`
	SourceID string               `json:"source_id,omitempty"`
}

type conditionRemovedPayload struct {
	Kind     ActivationResultKind `json:"kind"`
	Target   MemberID             `json:"target"`
	Ref      string               `json:"ref"`
	Name     string               `json:"name"`
	SourceID string               `json:"source_id,omitempty"`
	Reason   string               `json:"reason"`
}

type capacityGrantedPayload struct {
	Kind        ActivationResultKind `json:"kind"`
	Target      MemberID             `json:"target"`
	Description string               `json:"description"`
}

// RecordActivation appends one activated beat followed by one activation-result
// beat per result, preserving result order. The entire input and every payload
// are validated before the first append, so an input rejection cannot leave a
// partial transaction in the story.
//
// Every beat is a subjectBeat tagged "outcome". The activation honestly names
// the actor and optional selected target as subjects; each result names the
// actor and affected target. audienceFor currently sends both classes to the
// full roster under the pinned pre-v1 policy. This verb neither reads intel nor
// adds activation-specific visibility.
//
// noticeDown runs exactly once after all transaction beats, never between the
// activation and its results. A noticeDown error therefore leaves the complete
// transaction appended in memory and returns no output; doc.go's caller rule
// applies: discard the encounter unsaved.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember (empty or unknown actor, unknown
// selected target, or empty/unknown result target), ErrInvalidData (missing
// ability identity, unknown result kind, a missing/forbidden kind field, or a
// healing whose calculation is absent, structurally inconsistent, or whose
// Total does not equal the requested healing), an append error, or anything
// the Standing capability returns from noticeDown.
func (e *Encounter) RecordActivation(in *RecordActivationInput) (*RecordActivationOutput, error) {
	prepared, err := e.prepareActivation(in)
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
			return nil, fmt.Errorf("record activation: append beat %d: %w", i, appendErr)
		}
		seqs = append(seqs, appended.Seq)
	}

	_, intelDeltas, noticeErr := e.noticeDown()
	if noticeErr != nil {
		return nil, fmt.Errorf("record activation: %w", noticeErr)
	}

	return &RecordActivationOutput{Seqs: seqs, IntelDeltas: intelDeltas}, nil
}

// prepareActivation validates and marshals the complete transaction before the
// caller appends any of it. It is the validation/mutation boundary for
// RecordActivation, not merely a convenience split.
func (e *Encounter) prepareActivation(in *RecordActivationInput) ([]preparedActivationBeat, error) {
	if in == nil {
		return nil, fmt.Errorf("record activation: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("record activation: %w", ErrClosed)
	}
	if in.Actor == "" {
		return nil, fmt.Errorf("record activation: actor: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Actor]; !ok {
		return nil, fmt.Errorf("record activation: actor %q: %w", in.Actor, ErrNoMember)
	}
	if in.Target != "" {
		if _, ok := e.members[in.Target]; !ok {
			return nil, fmt.Errorf("record activation: target %q: %w", in.Target, ErrNoMember)
		}
	}
	if in.Ability.Ref == "" {
		return nil, fmt.Errorf("record activation: ability ref: %w", ErrInvalidData)
	}
	if in.Ability.Name == "" {
		return nil, fmt.Errorf("record activation: ability name: %w", ErrInvalidData)
	}

	activationBytes, err := json.Marshal(activatedPayload{
		Beat:  "activated",
		Actor: in.Actor,
		Ability: activationIdentityPayload{
			Ref:  in.Ability.Ref,
			Name: in.Ability.Name,
		},
		Target: in.Target,
	})
	if err != nil {
		return nil, fmt.Errorf("record activation: activated payload: %w", err)
	}
	activationSubjects := []MemberID{in.Actor}
	if in.Target != "" {
		activationSubjects = append(activationSubjects, in.Target)
	}
	prepared := make([]preparedActivationBeat, 0, len(in.Results)+1)
	prepared = append(prepared, preparedActivationBeat{
		payload:  activationBytes,
		subjects: activationSubjects,
	})

	for i, result := range in.Results {
		resultPayload, validationErr := e.prepareActivationResult("record activation", i, result)
		if validationErr != nil {
			return nil, validationErr
		}
		resultBytes, marshalErr := json.Marshal(activationResultPayload{
			Beat:   "activation-result",
			Actor:  in.Actor,
			Result: resultPayload,
		})
		if marshalErr != nil {
			return nil, fmt.Errorf("record activation: result %d payload: %w", i, marshalErr)
		}
		prepared = append(prepared, preparedActivationBeat{
			payload:  resultBytes,
			subjects: []MemberID{in.Actor, activationResultTarget(result)},
		})
	}

	return prepared, nil
}

// prepareActivationResult validates one result and returns the payload shape
// its kind marshals as. verb names the caller in every refusal — "record
// activation" or "record cast" — because a spell's damage refused under the
// activation's name would send the reader to the wrong door.
func (e *Encounter) prepareActivationResult(
	verb string, index int, result ActivationResult,
) (interface{}, error) {
	switch result.Kind {
	case ResultHealingApplied, ResultDamageApplied,
		ResultConditionApplied, ResultConditionRemoved, ResultCapacityGranted:
	default:
		return nil, fmt.Errorf("%s: result %d kind %q: %w", verb, index, result.Kind, ErrInvalidData)
	}

	target := activationResultTarget(result)
	if target == "" {
		return nil, fmt.Errorf("%s: result %d target: %w", verb, index, ErrNoMember)
	}
	if _, ok := e.members[target]; !ok {
		return nil, fmt.Errorf("%s: result %d target %q: %w", verb, index, target, ErrNoMember)
	}
	// ONE GUARD RATHER THAN AN ARM APIECE. A damage type belongs to exactly
	// one kind, so the refusal is stated once, before the switch, and a kind
	// added later cannot quietly start accepting one by forgetting to say no.
	if result.Kind != ResultDamageApplied && result.DamageType != "" {
		return nil, forbiddenActivationResultField(verb, index, result.Kind, "damage type")
	}
	if result.Kind != ResultConditionApplied && result.Kind != ResultConditionRemoved && result.Address != nil {
		return nil, forbiddenActivationResultField(verb, index, result.Kind, "condition address")
	}

	switch result.Kind {
	case ResultHealingApplied, ResultDamageApplied:
		if err := requireActivationIdentity(verb, index, result); err != nil {
			return nil, err
		}
		if result.Calculation == nil {
			return nil, fmt.Errorf(
				"%s: result %d %s requires a calculation: %w",
				verb, index, result.Kind, ErrInvalidData,
			)
		}
		if calcErr := ValidateRollCalculation(result.Calculation); calcErr != nil {
			return nil, fmt.Errorf(
				"%s: result %d %s calculation: %v: %w",
				verb, index, result.Kind, calcErr, ErrInvalidData,
			)
		}
		if result.Calculation.Total != result.Requested {
			return nil, fmt.Errorf(
				"%s: result %d %s calculation total %d does not equal the requested %d: %w",
				verb, index, result.Kind, result.Calculation.Total, result.Requested, ErrInvalidData,
			)
		}
		if result.Description != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, "description")
		}
		if result.Reason != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, "reason")
		}
		if result.Kind == ResultDamageApplied {
			if result.DamageType == "" {
				return nil, fmt.Errorf(
					"%s: result %d %s damage type: %w", verb, index, result.Kind, ErrInvalidData,
				)
			}
			return damageAppliedPayload{
				Kind: result.Kind, Target: result.Target,
				Amount: result.Amount, Requested: result.Requested,
				Before: result.Before, After: result.After,
				Calculation: result.Calculation,
				Ref:         result.Ref, Name: result.Name,
				DamageType: result.DamageType,
			}, nil
		}
		return healingAppliedPayload{
			Kind: result.Kind, Target: result.Target,
			Amount: result.Amount, Requested: result.Requested,
			Before: result.Before, After: result.After,
			Calculation: result.Calculation,
			Ref:         result.Ref, Name: result.Name,
		}, nil

	case ResultConditionApplied:
		if err := validateConditionResult(verb, index, result); err != nil {
			return nil, err
		}
		if field := rollFactsActivationResultField(result); field != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, field)
		}
		if result.Description != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, "description")
		}
		if result.Reason != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, "reason")
		}
		return conditionAppliedPayload{
			Kind: result.Kind, Target: result.Address.MemberID,
			Ref: result.Address.ConditionRef, Name: result.Name, SourceID: result.Address.SourceID,
		}, nil

	case ResultConditionRemoved:
		if err := validateConditionResult(verb, index, result); err != nil {
			return nil, err
		}
		if result.Reason == "" {
			return nil, fmt.Errorf("%s: result %d %s reason: %w", verb, index, result.Kind, ErrInvalidData)
		}
		if field := rollFactsActivationResultField(result); field != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, field)
		}
		if result.Description != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, "description")
		}
		return conditionRemovedPayload{
			Kind: result.Kind, Target: result.Address.MemberID,
			Ref: result.Address.ConditionRef, Name: result.Name,
			SourceID: result.Address.SourceID, Reason: result.Reason,
		}, nil

	case ResultCapacityGranted:
		if result.Description == "" {
			return nil, fmt.Errorf("%s: result %d %s description: %w", verb, index, result.Kind, ErrInvalidData)
		}
		if result.Ref != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, "ref")
		}
		if result.Name != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, "name")
		}
		if field := rollFactsActivationResultField(result); field != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, field)
		}
		if result.Reason != "" {
			return nil, forbiddenActivationResultField(verb, index, result.Kind, "reason")
		}
		return capacityGrantedPayload{
			Kind: result.Kind, Target: result.Target, Description: result.Description,
		}, nil
	}

	panic("unreachable activation result kind")
}

func activationResultTarget(result ActivationResult) MemberID {
	if result.Address != nil {
		return result.Address.MemberID
	}
	return result.Target
}

func validateConditionResult(verb string, index int, result ActivationResult) error {
	if result.Address == nil {
		return fmt.Errorf("%s: result %d %s address: %w", verb, index, result.Kind, ErrInvalidData)
	}
	if result.Target != "" {
		return forbiddenActivationResultField(verb, index, result.Kind, "target outside address")
	}
	if result.Ref != "" {
		return forbiddenActivationResultField(verb, index, result.Kind, "ref outside address")
	}
	if result.Address.ConditionRef == "" {
		return fmt.Errorf("%s: result %d %s condition ref: %w", verb, index, result.Kind, ErrInvalidData)
	}
	if _, err := core.ParseString(result.Address.ConditionRef); err != nil {
		return fmt.Errorf("%s: result %d %s condition ref %q: %v: %w",
			verb, index, result.Kind, result.Address.ConditionRef, err, ErrInvalidData)
	}
	if result.Name == "" {
		return fmt.Errorf("%s: result %d %s name: %w", verb, index, result.Kind, ErrInvalidData)
	}
	return nil
}

func requireActivationIdentity(verb string, index int, result ActivationResult) error {
	if result.Ref == "" {
		return fmt.Errorf("%s: result %d %s ref: %w", verb, index, result.Kind, ErrInvalidData)
	}
	if result.Name == "" {
		return fmt.Errorf("%s: result %d %s name: %w", verb, index, result.Kind, ErrInvalidData)
	}
	return nil
}

// rollFactsActivationResultField names the first roll fact a result with no
// roll behind it — every kind but healing and damage — may not carry: the calculation trace, then the healing's numeric
// facts. Calculation is a pointer, so its PRESENCE is what is detected; the
// numeric facts are plain ints, so what is detected (and refused) is a
// NON-ZERO value — a zero is indistinguishable from an absent field here and
// is not refused by these scalar checks.
func rollFactsActivationResultField(result ActivationResult) string {
	if result.Calculation != nil {
		return "calculation"
	}
	switch {
	case result.Amount != 0:
		return "amount"
	case result.Requested != 0:
		return "requested"
	case result.Before != 0:
		return "before"
	case result.After != 0:
		return "after"
	default:
		return ""
	}
}

func forbiddenActivationResultField(verb string, index int, kind ActivationResultKind, field string) error {
	return fmt.Errorf("%s: result %d %s forbids %s: %w", verb, index, kind, field, ErrInvalidData)
}
