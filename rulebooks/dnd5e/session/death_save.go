// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// DeathSaveOutcome is the session-owned projection of the provider result.
type DeathSaveOutcome string

const (
	DeathSaveOutcomeSuccess      DeathSaveOutcome = "success"
	DeathSaveOutcomeFailure      DeathSaveOutcome = "failure"
	DeathSaveOutcomeCriticalFail DeathSaveOutcome = "critical_failure"
	DeathSaveOutcomeStabilized   DeathSaveOutcome = "stabilized"
	DeathSaveOutcomeDead         DeathSaveOutcome = "dead"
	DeathSaveOutcomeRecovered    DeathSaveOutcome = "recovered"
)

// DeathSaveContinuation preserves the provider's instruction for the current
// turn. Session validates the resulting clock shape; it never derives this
// value from the d20.
type DeathSaveContinuation string

const (
	DeathSaveContinuationEndTurn         DeathSaveContinuation = "end_turn"
	DeathSaveContinuationKeepTurn        DeathSaveContinuation = "keep_turn"
	DeathSaveContinuationAlreadyAdvanced DeathSaveContinuation = "already_advanced"
)

// DeathSaveInput executes one current explicit Death Save declaration.
type DeathSaveInput struct {
	Session       string
	Member        string
	DeclarationID string
}

// DeathSaveOutput is the accepted provider result plus session persistence,
// delivery, opaque presentation correlation, and recipient-local sequence.
type DeathSaveOutput struct {
	Roll              int                   `json:"roll"`
	Outcome           DeathSaveOutcome      `json:"outcome"`
	SuccessesAdded    int                   `json:"successes_added"`
	FailuresAdded     int                   `json:"failures_added"`
	Successes         int                   `json:"successes"`
	Failures          int                   `json:"failures"`
	SuccessesNeeded   int                   `json:"successes_needed"`
	FailuresRemaining int                   `json:"failures_remaining"`
	Stabilized        bool                  `json:"stabilized"`
	Dead              bool                  `json:"dead"`
	Recovered         bool                  `json:"recovered"`
	HPRestored        int                   `json:"hp_restored"`
	Continuation      DeathSaveContinuation `json:"continuation"`
	PresentationID    string                `json:"presentation_id"`
	Seq               uint64                `json:"seq"`
	Saved             SaveReport            `json:"saved"`
	Delivery          DeliveryReport        `json:"delivery"`
}

// deathSaveResult is projected exactly once from the provider and then reused
// for response and Story. Numeric sequence and persistence reports are kept out
// because they are recipient/session delivery facts, not game facts.
type deathSaveResult struct {
	Roll              int
	Outcome           DeathSaveOutcome
	SuccessesAdded    int
	FailuresAdded     int
	Successes         int
	Failures          int
	SuccessesNeeded   int
	FailuresRemaining int
	Stabilized        bool
	Dead              bool
	Recovered         bool
	HPRestored        int
	Continuation      DeathSaveContinuation
	PresentationID    string
}

// DeathSave executes the active Dying character's selected current offer.
// Required input validation precedes opening the write scope. Selection then
// precedes both opaque-ID generation and the sole d20 roll. The changed
// character is saved before Story so encounter participation observes
// the authoritative post-save state; a later failure therefore returns a
// SaveError and cannot be retried as though nothing happened.
func (m *Manager) DeathSave(ctx context.Context, in *DeathSaveInput) (*DeathSaveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("death save: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("death save: %w", ErrNoMemberID)
	}
	if in.DeclarationID == "" {
		return nil, fmt.Errorf("death save: %w", ErrNoDeclarationID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("death save: %w", err)
	}
	roster, err := scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("death save: %w", translate(err))
	}
	kind := map[string]encounter.MemberKind{}
	for _, member := range roster {
		kind[string(member.ID)] = member.Kind
	}
	if _, ok := kind[in.Member]; !ok {
		return nil, fmt.Errorf("death save: member %q: %w", in.Member, ErrNoMember)
	}
	if kind[in.Member] != encounter.MemberKind(KindPlayer) {
		return nil, fmt.Errorf("death save: member %q: %w", in.Member, ErrNotACharacter)
	}

	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("death save: %w", translate(err))
	}
	if ClockKind(clock.Kind) != ClockTurn {
		return nil, fmt.Errorf("death save: %w", ErrNotInFight)
	}
	if string(clock.Active) != in.Member {
		return nil, fmt.Errorf("death save: %w", ErrNotYourTurn)
	}

	// One actor record load. This same sheet proves eligibility, regenerates the
	// selector, and supplies resolution's persisted input.
	actor := m.loadActorSheet(ctx, in.Member)
	offers, err := m.compileOffersFor(
		ctx, scope.enc, scope.data, scope.session, in.Member, clock, actor, VerbDeathSave,
	)
	if err != nil {
		return nil, fmt.Errorf("death save: %w", err)
	}
	selected, err := selectCompiledOffer(offers, VerbDeathSave, in.DeclarationID)
	if err != nil {
		return nil, fmt.Errorf("death save: %w", err)
	}
	if selected.sheet == nil {
		return nil, fmt.Errorf("death save: %w", ErrStaleDeclaration)
	}

	presentationID := m.presentationIDs.Generate()
	if err := validatePresentationID(presentationID); err != nil {
		return nil, fmt.Errorf("death save: generated presentation id: %w", err)
	}

	resolved, err := resolution.DeathSave(ctx, &resolution.DeathSaveInput{
		Character: selected.sheet.ToData(),
		Roller:    &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("death save: %w", translateResolution(err))
	}
	if resolved == nil || resolved.Character == nil {
		return nil, fmt.Errorf("death save: %w: provider returned no character", ErrBadCharacter)
	}
	result := projectDeathSaveResult(resolved.Result, presentationID)

	// Save first: Record immediately re-assesses participation through the
	// repository and must see this result, not the pre-roll character.
	if err := m.saveCharacterRecord(ctx, scope, resolved.Character); err != nil {
		return nil, fmt.Errorf("death save: %w", err)
	}

	pendingGlobalSeq, err := scope.enc.NextStorySeq()
	if err != nil {
		return nil, fmt.Errorf("death save: %w", reportUnrecorded(scope, translate(err)))
	}
	recorded, err := scope.enc.Record(deathSaveRecord(in.Member, result))
	if err != nil {
		return nil, fmt.Errorf("death save: %w", reportUnrecorded(scope, translate(err)))
	}
	if recorded.Seq != pendingGlobalSeq {
		return nil, fmt.Errorf("death save: %w", reportUnrecorded(scope,
			fmt.Errorf("recorded sequence %d, expected pending %d: %w",
				recorded.Seq, pendingGlobalSeq, ErrInvalidWorld)))
	}
	if err := assertDeathSaveContinuation(scope.enc, in.Member, result.Continuation); err != nil {
		return nil, fmt.Errorf("death save: %w", reportUnrecorded(scope, err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("death save: %w", err)
	}

	return result.output(scope.deliveredSeq(in.Member, recorded.Seq), report, delivery), nil
}

func projectDeathSaveResult(
	in character.MakeDeathSaveOutput, presentationID string,
) deathSaveResult {
	return deathSaveResult{
		Roll: in.Roll, Outcome: DeathSaveOutcome(in.Outcome),
		SuccessesAdded: in.SuccessesAdded, FailuresAdded: in.FailuresAdded,
		Successes: in.Progress.Successes, Failures: in.Progress.Failures,
		SuccessesNeeded:   in.Progress.SuccessesNeeded,
		FailuresRemaining: in.Progress.FailuresRemaining,
		Stabilized:        in.Progress.Stabilized, Dead: in.Progress.Dead,
		Recovered: in.RegainedConscious, HPRestored: in.HPRestored,
		Continuation:   DeathSaveContinuation(in.Continuation),
		PresentationID: presentationID,
	}
}

func deathSaveRecord(member string, result deathSaveResult) *encounter.RecordInput {
	return &encounter.RecordInput{
		Kind: encounter.OutcomeDeathSave, Actor: encounter.MemberID(member),
		DeathSave: &encounter.DeathSaveDetail{
			Roll: result.Roll, Outcome: string(result.Outcome),
			SuccessesAdded: result.SuccessesAdded, FailuresAdded: result.FailuresAdded,
			Successes: result.Successes, Failures: result.Failures,
			SuccessesNeeded:   result.SuccessesNeeded,
			FailuresRemaining: result.FailuresRemaining,
			Stabilized:        result.Stabilized, Dead: result.Dead,
			Recovered: result.Recovered, HPRestored: result.HPRestored,
			Continuation: string(result.Continuation), PresentationID: result.PresentationID,
		},
	}
}

func assertDeathSaveContinuation(
	enc *encounter.Encounter, member string, continuation DeathSaveContinuation,
) error {
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(member)})
	if continuation == DeathSaveContinuationAlreadyAdvanced {
		if err != nil {
			if errors.Is(translate(err), ErrClosed) {
				return nil
			}
			return translate(err)
		}
		if ClockKind(clock.Kind) == ClockTurn && string(clock.Active) == member {
			return fmt.Errorf("dead member %q remained active: %w", member, ErrInvalidWorld)
		}
		return nil
	}
	switch continuation {
	case DeathSaveContinuationEndTurn, DeathSaveContinuationKeepTurn:
		// Both provider continuations retain the active slot in this command;
		// EndTurn tells the caller what to declare next, while KeepTurn permits
		// ordinary offers immediately.
	default:
		return fmt.Errorf("unknown death save continuation %q: %w", continuation, ErrInvalidWorld)
	}
	if err != nil {
		return translate(err)
	}
	if ClockKind(clock.Kind) != ClockTurn || string(clock.Active) != member {
		return fmt.Errorf("continuation %q did not retain active member %q: %w",
			continuation, member, ErrInvalidWorld)
	}
	return nil
}

func (r deathSaveResult) output(
	seq uint64, saved SaveReport, delivery DeliveryReport,
) *DeathSaveOutput {
	return &DeathSaveOutput{
		Roll: r.Roll, Outcome: r.Outcome,
		SuccessesAdded: r.SuccessesAdded, FailuresAdded: r.FailuresAdded,
		Successes: r.Successes, Failures: r.Failures,
		SuccessesNeeded: r.SuccessesNeeded, FailuresRemaining: r.FailuresRemaining,
		Stabilized: r.Stabilized, Dead: r.Dead, Recovered: r.Recovered,
		HPRestored: r.HPRestored, Continuation: r.Continuation,
		PresentationID: r.PresentationID, Seq: seq, Saved: saved, Delivery: delivery,
	}
}
