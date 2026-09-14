// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/checks"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// frozenCheckKind and frozenCheckVersion discriminate what a stored blob is,
// the same trust boundary [frozenStrikeKind]/[frozenStrikeVersion] keep.
const (
	frozenCheckKind    = "check.post_roll"
	frozenCheckVersion = 1
)

// frozenCheck is a check stopped after its d20, in enough detail to finish it
// and in no more detail than that — the check sibling of [frozenStrike].
//
// THE FOLD IS STORED RATHER THAN RECOMPUTED, for [frozenStrike]'s reason: a
// sheet edited during the pause must not change what the checker was asked
// about after they answered.
type frozenCheck struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`

	CheckerID string                  `json:"checker_id"`
	Applied   encounter.CheckApproach `json:"applied"`
	DC        int                     `json:"dc"`
	Roll      int                     `json:"roll"`
	Total     int                     `json:"total"`
	IsNat1    bool                    `json:"is_nat1"`
	IsNat20   bool                    `json:"is_nat20"`

	AdvantageSources    []dnd5eEvents.CheckModifierSource `json:"advantage_sources"`
	DisadvantageSources []dnd5eEvents.CheckModifierSource `json:"disadvantage_sources"`
	BonusSources        []dnd5eEvents.CheckBonusSource    `json:"bonus_sources"`

	// Calculation is the settled pre-offer arithmetic and is reused verbatim
	// on resume.
	Calculation *dnd5eEvents.RollCalculation `json:"calculation"`

	// Offer is what was put on the table.
	Offer dnd5eEvents.Offer `json:"offer"`
}

// poseCheckInput is what [poseCheck] needs to freeze one posed check.
type poseCheckInput struct {
	checkerID   string
	applied     encounter.CheckApproach
	result      *checks.AbilityCheckResult
	calculation *dnd5eEvents.RollCalculation
	offers      []dnd5eEvents.Offer
}

// poseCheck stops the check and hands its state out as bytes — the check
// sibling of [strikeMachine.pose]. Same two refusals, for the same reasons:
// an offer whose audience is not the checker, or more than one offer on one
// roll, is a window this slice has not designed a freeze for.
func poseCheck(in poseCheckInput) (*Pose, error) {
	if len(in.offers) > 1 {
		return nil, fmt.Errorf("%w: %d offers on one check, and this build poses one question",
			ErrNotOffered, len(in.offers))
	}
	offer := in.offers[0]
	if offer.Audience != in.checkerID {
		return nil, fmt.Errorf("%w: %q offered on %q's check, and only the checker is asked here",
			ErrNotOffered, offer.Audience, in.checkerID)
	}
	if offer.Ref == nil || offer.Die == "" {
		return nil, fmt.Errorf("%w: an offer with no ref or no die is nothing to ask about", ErrNotOffered)
	}

	frozen, err := json.Marshal(frozenCheck{
		Kind:                frozenCheckKind,
		Version:             frozenCheckVersion,
		CheckerID:           in.checkerID,
		Applied:             in.applied,
		DC:                  in.result.DC,
		Roll:                in.result.Roll,
		Total:               in.calculation.Total,
		IsNat1:              in.result.IsNat1,
		IsNat20:             in.result.IsNat20,
		AdvantageSources:    in.result.AdvantageSources,
		DisadvantageSources: in.result.DisadvantageSources,
		BonusSources:        in.result.BonusSources,
		Calculation:         dnd5eEvents.CloneRollCalculation(in.calculation),
		Offer:               offer,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: freeze check: %v", ErrBadFrozen, err)
	}

	return &Pose{
		Ask: Ask{
			Audience: offer.Audience,
			Offer:    offer,
			Options:  []string{string(OfferSpend), string(OfferKeep)},
			Roll:     in.result.Roll,
			Total:    in.calculation.Total,
		},
		Frozen: frozen,
	}, nil
}

// CheckResumeInput continues a check that posed.
type CheckResumeInput struct {
	// Frozen is the bytes [Pose.Frozen] handed out. REQUIRED.
	Frozen []byte

	// Answer is [OfferSpend] or [OfferKeep]. REQUIRED.
	Answer OfferAnswer

	// Character is the checker's own record, freshly loaded by the caller —
	// not carried in Frozen, because persistence is the caller's business,
	// the same division [Input.Participants] keeps from a resumed Machine.
	// REQUIRED whichever the answer is: this entry attaches and installs
	// truth unconditionally (TestOnlyTheDoorInstallsGameContext), so even
	// declining needs a sheet to attach.
	Character *character.Data

	// Roller rolls the offered die. REQUIRED on [OfferSpend].
	Roller dice.Roller
}

// ResumeCheck finishes a check somebody answered.
//
// It starts where the pose was: apply the answer, then hand back the same
// [CheckOutput] a finished [MakeCheck] would. Nothing is re-rolled and
// nothing is re-folded — the check sibling of [NewStrikeResumed].
//
// The checker is attached and truth installed unconditionally, whichever
// answer this is. [OfferKeep] then touches the bus no further: declining
// costs nothing, so nothing is rolled and nothing published. Only
// [OfferSpend] rolls the die and publishes [dnd5eEvents.OfferTakenTopic], so
// the reloaded condition can hear its own die was spent and remove itself.
func ResumeCheck(ctx context.Context, in *CheckResumeInput) (*CheckOutput, error) {
	return resumeCheckOn(ctx, in, newSurface(events.NewEventBus()))
}

// resumeCheckOn is [ResumeCheck] with the surface handed in, for
// [makeCheckOn]'s testing reason.
func resumeCheckOn(ctx context.Context, in *CheckResumeInput, surf *surface) (*CheckOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if len(in.Frozen) == 0 {
		return nil, fmt.Errorf("%w: no frozen check to resume", ErrBadFrozen)
	}
	switch in.Answer {
	case OfferSpend, OfferKeep:
	default:
		return nil, fmt.Errorf("%w: %q is not an answer this machine posed", ErrNotOffered, in.Answer)
	}

	var frozen frozenCheck
	if err := json.Unmarshal(in.Frozen, &frozen); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if frozen.Kind != frozenCheckKind || frozen.Version != frozenCheckVersion {
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
	if frozen.Offer.Audience != frozen.CheckerID {
		return nil, fmt.Errorf("%w: the offer names %q on %q's roll",
			ErrNotOffered, frozen.Offer.Audience, frozen.CheckerID)
	}

	if in.Character == nil {
		return nil, fmt.Errorf("%w: resuming a check requires the checker's own record", ErrBadParticipant)
	}

	// Attached and installed unconditionally, whichever answer this is: a
	// resumed entry that only sometimes attaches is exactly what
	// TestOnlyTheDoorInstallsGameContext refuses, because [OfferKeep] would
	// otherwise be the one path in this package running without the door.
	one := Participant{Character: in.Character}
	if err := one.validate(); err != nil {
		return nil, err
	}
	if one.ID() != frozen.CheckerID {
		return nil, fmt.Errorf("%w: resuming %q's check with %q's sheet",
			ErrBadParticipant, frozen.CheckerID, one.ID())
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants: []Participant{one},
		Roller:       refusingRoller{},
	})
	if err != nil {
		_ = surf.teardown(ctx)
		return nil, err
	}
	ctx = installTruth(ctx, nil, cast, nil)

	ch, ok := cast.Character(one.ID())
	if !ok {
		return nil, errors.Join(
			fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, one.ID()),
			surf.teardown(ctx),
		)
	}

	calculation := dnd5eEvents.CloneRollCalculation(frozen.Calculation)
	total := frozen.Total

	// [OfferKeep] touches the bus no further than the attach above: declining
	// costs nothing, so nothing is rolled and nothing is published — the die
	// stays in hand for the next check, exactly as an offer nobody takes does
	// everywhere else in this package.
	if in.Answer == OfferSpend {
		if in.Roller == nil {
			return nil, errors.Join(
				fmt.Errorf("%w: a resumed check rolls with no roller", ErrNoRoller), surf.teardown(ctx))
		}

		size, sizeErr := parseDieSize(frozen.Offer.Die)
		if sizeErr != nil {
			return nil, errors.Join(fmt.Errorf("%w: offered die: %v", ErrBadFrozen, sizeErr), surf.teardown(ctx))
		}
		face, rollErr := in.Roller.Roll(ctx, size)
		if rollErr != nil {
			return nil, errors.Join(fmt.Errorf("roll offered die: %w", rollErr), surf.teardown(ctx))
		}

		component := dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{Ref: cloneCoreRef(frozen.Offer.Ref), Name: frozen.Offer.Name},
			Dice: &dnd5eEvents.DiceTrace{
				Notation: dice.SimplePool(1, size, 0).Notation(), DieSize: size,
				OriginalRolls: []int{face}, FinalRolls: []int{face}, Subtotal: face,
			},
		}
		calculation.Components = append(calculation.Components, component)
		calculation.Total += face
		if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
			return nil, errors.Join(fmt.Errorf("append offered die: %w", err), surf.teardown(ctx))
		}
		total = calculation.Total

		if err := dnd5eEvents.OfferTakenTopic.On(surf).Publish(ctx, dnd5eEvents.OfferTakenEvent{
			Audience: frozen.Offer.Audience, Ref: frozen.Offer.Ref, Face: face,
		}); err != nil {
			return nil, errors.Join(fmt.Errorf("publish offer taken: %w", err), surf.teardown(ctx))
		}
	}

	tearErr := surf.teardown(ctx)
	if tearErr != nil {
		return nil, fmt.Errorf("resolution: teardown: %w", tearErr)
	}

	var dirty *character.Data
	if ch.IsDirty() {
		dirty = ch.ToData()
	}

	result := &checks.AbilityCheckResult{
		Roll:                frozen.Roll,
		Total:               total,
		DC:                  frozen.DC,
		Success:             total >= frozen.DC,
		IsNat1:              frozen.IsNat1,
		IsNat20:             frozen.IsNat20,
		AdvantageSources:    frozen.AdvantageSources,
		DisadvantageSources: frozen.DisadvantageSources,
		BonusSources:        frozen.BonusSources,
	}

	return &CheckOutput{
		Result: result, Applied: frozen.Applied, Calculation: calculation, DirtyCharacter: dirty,
	}, nil
}
