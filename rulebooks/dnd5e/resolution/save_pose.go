// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// frozenSaveKind and frozenSaveVersion discriminate what a stored blob is,
// the same trust boundary [frozenStrikeKind]/[frozenCheckKind] keep.
const (
	frozenSaveKind    = "save.post_roll"
	frozenSaveVersion = 1
)

// frozenSave is a saving throw stopped after its d20, in enough detail to
// finish it and in no more detail than that — the save sibling of
// [frozenCheck]. THE FOLD IS STORED RATHER THAN RECOMPUTED, for
// [frozenStrike]'s reason.
type frozenSave struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`

	SaverID string            `json:"saver_id"`
	Ability abilities.Ability `json:"ability"`
	DC      int               `json:"dc"`
	Roll    int               `json:"roll"`
	Total   int               `json:"total"`
	IsNat1  bool              `json:"is_nat1"`
	IsNat20 bool              `json:"is_nat20"`

	BonusSources []dnd5eEvents.SaveBonusSource `json:"bonus_sources"`

	// Calculation is the settled pre-offer arithmetic — [saves.MakeSavingThrow]
	// already builds it sourced, unlike a check, so it is stored verbatim
	// rather than rebuilt. It is also where advantage and disadvantage are
	// frozen: the d20 component's keep record names the rules that met over
	// it, so no parallel source list sits beside it here (rpg-project#462 R1).
	Calculation *dnd5eEvents.RollCalculation `json:"calculation"`

	// Offer is what was put on the table.
	Offer dnd5eEvents.Offer `json:"offer"`
}

// saveResume is a frozen save plus the answer it came back with.
type saveResume struct {
	frozen frozenSave
	answer OfferAnswer
}

// newSaveResumedFromJSON builds the machine that finishes a saving throw
// somebody answered, from the raw bytes [poseContest] embedded. Unexported:
// a bare save is never resolved on its own ([requestSave] is the only caller
// of [NewSave]), so resuming one is a concern internal to [contestMachine]'s
// own resume, not a public entry.
//
// It starts where the pose was: apply the answer, then hand back the same
// [SaveOutcome] a finished [NewSave] would. Nothing is re-rolled and nothing
// is re-folded — the save sibling of [NewStrikeResumed] and [ResumeCheck].
//
// # It fails closed on a frozen blob it cannot trust
//
// The same checks [NewStrikeResumed] runs before the world is loaded and
// before anything is charged: kind and version, a d20 in range, and a
// calculation that validates and agrees with the stored total.
func newSaveResumedFromJSON(raw json.RawMessage, answer OfferAnswer, roller dice.Roller) (Machine, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: no frozen save to resume", ErrBadFrozen)
	}
	switch answer {
	case OfferSpend, OfferKeep:
	default:
		return nil, fmt.Errorf("%w: %q is not an answer this machine posed", ErrNotOffered, answer)
	}

	var frozen frozenSave
	if err := json.Unmarshal(raw, &frozen); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if frozen.Kind != frozenSaveKind || frozen.Version != frozenSaveVersion {
		return nil, fmt.Errorf("%w: kind %q version %d is not one this build froze",
			ErrBadFrozen, frozen.Kind, frozen.Version)
	}
	if frozen.Roll < 1 || frozen.Roll > 20 {
		return nil, fmt.Errorf("%w: a d20 does not read %d", ErrBadFrozen, frozen.Roll)
	}
	if err := dnd5eEvents.ValidateRollCalculation(frozen.Calculation); err != nil {
		return nil, fmt.Errorf("%w: calculation: %v", ErrBadFrozen, err)
	}
	if frozen.Calculation.Total != frozen.Total {
		return nil, fmt.Errorf("%w: total does not match the frozen calculation", ErrBadFrozen)
	}
	if frozen.Offer.Audience != frozen.SaverID {
		return nil, fmt.Errorf("%w: the offer names %q on %q's roll",
			ErrNotOffered, frozen.Offer.Audience, frozen.SaverID)
	}
	if answer == OfferSpend && roller == nil {
		return nil, fmt.Errorf("%w: a resumed save rolls with no roller", ErrNoRoller)
	}

	return &saveMachine{
		in:     &SaveInput{SaverID: frozen.SaverID, Ability: frozen.Ability, DC: frozen.DC, Roller: roller},
		resume: &saveResume{frozen: frozen, answer: answer},
	}, nil
}

// pose stops the machine and hands its state out as bytes — the save sibling
// of [strikeMachine.pose] and [poseCheck]. Same two refusals, for the same
// reasons.
func (m *saveMachine) pose(result *saves.SavingThrowResult, offers []dnd5eEvents.Offer) (Step, error) {
	if len(offers) > 1 {
		return nil, fmt.Errorf("%w: %d offers on one save, and this build poses one question",
			ErrNotOffered, len(offers))
	}
	offer := offers[0]
	if offer.Audience != m.in.SaverID {
		return nil, fmt.Errorf("%w: %q offered on %q's save, and only the saver is asked here",
			ErrNotOffered, offer.Audience, m.in.SaverID)
	}
	if offer.Ref == nil || offer.Die == "" {
		return nil, fmt.Errorf("%w: an offer with no ref or no die is nothing to ask about", ErrNotOffered)
	}

	frozen, err := json.Marshal(frozenSave{
		Kind: frozenSaveKind, Version: frozenSaveVersion,
		SaverID: m.in.SaverID, Ability: m.in.Ability, DC: m.in.DC,
		Roll: result.Roll, Total: result.Total,
		IsNat1: result.IsNat1, IsNat20: result.IsNat20,
		BonusSources: result.BonusSources,
		Calculation:  dnd5eEvents.CloneRollCalculation(result.Calculation),
		Offer:        offer,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: freeze save: %v", ErrBadFrozen, err)
	}

	return Pose{
		Ask: Ask{
			Audience:    offer.Audience,
			Offer:       offer,
			Options:     []string{string(OfferSpend), string(OfferKeep)},
			Roll:        result.Roll,
			Total:       result.Total,
			Calculation: dnd5eEvents.CloneRollCalculation(result.Calculation),
		},
		Frozen: frozen,
	}, nil
}

// resumeStep is the first step of a resumed save: apply the answer, then
// finish with exactly the outcome a fresh save would have produced.
func (m *saveMachine) resumeStep() Step {
	frozen := m.resume.frozen

	return Gather{
		name: fmt.Sprintf("answer %s for %s", frozen.Offer.Ref.String(), frozen.Offer.Audience),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			total := frozen.Total
			calculation := dnd5eEvents.CloneRollCalculation(frozen.Calculation)

			if m.resume.answer == OfferSpend {
				if err := m.spendOffer(ctx, bus, frozen.Offer, calculation); err != nil {
					return nil, err
				}
				total = calculation.Total
			}

			return Done{Outcome: SaveOutcome{Result: &saves.SavingThrowResult{
				Roll:         frozen.Roll,
				Total:        total,
				DC:           frozen.DC,
				Success:      total >= frozen.DC,
				IsNat1:       frozen.IsNat1,
				IsNat20:      frozen.IsNat20,
				BonusSources: frozen.BonusSources,
				Calculation:  calculation,
			}}}, nil
		},
	}
}

// spendOffer rolls the offered die, adds its face, and tells whoever offered
// it — the save sibling of [strikeMachine.spendOffer].
func (m *saveMachine) spendOffer(
	ctx context.Context, bus events.EventBus, offer dnd5eEvents.Offer, calculation *dnd5eEvents.RollCalculation,
) error {
	if m.in.Roller == nil {
		return fmt.Errorf("%w: a resumed save rolls with no roller", ErrNoRoller)
	}
	size, err := parseDieSize(offer.Die)
	if err != nil {
		return fmt.Errorf("%w: offered die: %v", ErrBadFrozen, err)
	}
	face, err := m.in.Roller.Roll(ctx, size)
	if err != nil {
		return fmt.Errorf("roll offered die: %w", err)
	}

	component := dnd5eEvents.RollComponent{
		Source: dnd5eEvents.RollSource{
			Ref: cloneCoreRef(offer.Ref), Name: offer.Name, SourceID: offer.SourceID,
		},
		Dice: &dnd5eEvents.DiceTrace{
			Notation: dice.SimplePool(1, size, 0).Notation(), DieSize: size,
			OriginalRolls: []int{face}, FinalRolls: []int{face}, Subtotal: face,
		},
	}
	calculation.Components = append(calculation.Components, component)
	calculation.Total += face
	if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
		return fmt.Errorf("append offered die: %w", err)
	}

	if err := dnd5eEvents.OfferTakenTopic.On(bus).Publish(ctx, dnd5eEvents.OfferTakenEvent{
		Audience: offer.Audience, Ref: offer.Ref, Face: face,
	}); err != nil {
		return fmt.Errorf("publish offer taken: %w", err)
	}
	return nil
}
