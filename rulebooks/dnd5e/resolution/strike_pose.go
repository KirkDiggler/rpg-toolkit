// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// OfferAnswer is one answer to a posed offer.
//
// TWO WORDS, OWNED HERE. The seam above has its own vocabulary for the same
// two answers (a session says strike and hold, because its window is a
// reaction); this package's question is "spend the thing you hold, or keep
// it", and translating at the seam is cheaper than either module speaking the
// other's language.
type OfferAnswer string

const (
	// OfferSpend takes the offer: the die is rolled, its face joins the total,
	// and the effect that offered it is told so it can consume itself.
	OfferSpend OfferAnswer = "spend"

	// OfferKeep declines, and COSTS NOTHING. Nothing is rolled, nothing is
	// published, and the effect is never told — so the die is still in hand
	// for the next roll.
	OfferKeep OfferAnswer = "keep"
)

// frozenStrikeKind and frozenStrikeVersion discriminate what a stored blob is.
//
// Written and CHECKED, so a payload from a build that froze something else, or
// froze this differently, is refused rather than read as a strike. The same
// trust boundary a stored window payload keeps one layer up.
const (
	frozenStrikeKind    = "strike.post_roll"
	frozenStrikeVersion = 2
)

// frozenStrike is a strike stopped after its d20, in enough detail to finish
// it and in no more detail than that.
//
// THE FOLD IS STORED RATHER THAN RECOMPUTED, and that is the whole reason this
// type exists instead of a note saying "re-run it". Re-folding the attack
// chain would be a second fold with side effects: a subscriber that records an
// attempt would record two, and a sheet edited during the pause would change
// what the player was asked about after they answered.
type frozenStrike struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`

	AttackerID string `json:"attacker_id"`
	TargetID   string `json:"target_id"`

	// Definition is the attack that was offered, stored whole for the reason
	// above.
	Definition combatActions.Definition `json:"definition"`

	// Folded is the attack chain after every subscriber had its say. It
	// carries the target's AC, the critical threshold, the attack bonus and
	// the advantage/disadvantage sources — everything the second half reads.
	Folded dnd5eEvents.AttackChainEvent `json:"folded"`

	// Roll is the d20 as rolled and Total is Roll plus the folded bonus. Both
	// are stored so the pair can be CHECKED against each other on the way back
	// in: a total that is not roll plus bonus is a blob nobody should act on.
	Roll  int `json:"roll"`
	Total int `json:"total"`

	// Calculation is the settled pre-offer arithmetic and is reused verbatim on resume.
	Calculation *dnd5eEvents.RollCalculation `json:"calculation"`

	// Offer is what was put on the table.
	Offer dnd5eEvents.Offer `json:"offer"`
}

// strikeResume is a frozen strike plus the answer it came back with.
type strikeResume struct {
	frozen frozenStrike
	answer OfferAnswer
}

// StrikeResumeInput continues a strike that posed.
type StrikeResumeInput struct {
	// Frozen is the bytes [Pose.Frozen] handed out. REQUIRED.
	Frozen []byte

	// Answer is [OfferSpend] or [OfferKeep]. REQUIRED — an empty answer is a
	// caller that has not asked anybody yet.
	Answer OfferAnswer

	// Roller rolls the offered die. REQUIRED whichever the answer is: the
	// machine that rolls carries its own roller, and refusing a nil one at the
	// door rather than on the spend branch means a mis-wired caller finds out
	// on the first decline instead of the first spend.
	Roller dice.Roller
}

// NewStrikeResumed returns the machine that finishes a strike somebody
// answered.
//
// It starts where the pose was: apply the answer, decide the hit against the
// total that results, publish the post-roll chain ONCE across the pair, and
// deal damage. Nothing is re-rolled and nothing is re-folded.
//
// # It fails closed on a frozen blob it cannot trust
//
// A d20 outside 1-20, or a total that is not the roll plus the folded bonus,
// is refused by name — before the world is loaded and before anything is
// charged. Repairing either would resolve an attack nobody rolled.
func NewStrikeResumed(in *StrikeResumeInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if len(in.Frozen) == 0 {
		return nil, fmt.Errorf("%w: no frozen strike to resume", ErrBadFrozen)
	}
	switch in.Answer {
	case OfferSpend, OfferKeep:
	default:
		return nil, fmt.Errorf("%w: %q is not an answer this machine posed", ErrNotOffered, in.Answer)
	}
	if in.Roller == nil {
		return nil, fmt.Errorf("%w: a resumed strike rolls with no roller", ErrNoRoller)
	}

	var frozen frozenStrike
	if err := json.Unmarshal(in.Frozen, &frozen); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if frozen.Kind != frozenStrikeKind || frozen.Version != frozenStrikeVersion {
		return nil, fmt.Errorf("%w: kind %q version %d is not one this build froze",
			ErrBadFrozen, frozen.Kind, frozen.Version)
	}
	if frozen.Roll < 1 || frozen.Roll > 20 {
		return nil, fmt.Errorf("%w: a d20 does not read %d", ErrBadFrozen, frozen.Roll)
	}
	if err := dnd5eEvents.ValidateRollCalculation(frozen.Calculation); err != nil {
		return nil, fmt.Errorf("%w: calculation: %v", ErrBadFrozen, err)
	}
	if len(frozen.Calculation.Components) < 2 || frozen.Calculation.Components[0].Dice == nil ||
		frozen.Calculation.Components[0].Source.Ref == nil ||
		!frozen.Calculation.Components[0].Source.Ref.Equals(&frozen.Definition.Ref) ||
		frozen.Calculation.Components[0].Source.Name != frozen.Definition.Name ||
		frozen.Calculation.Components[0].Dice.DieSize != 20 ||
		frozen.Calculation.Components[0].Dice.Subtotal != frozen.Roll ||
		frozen.Calculation.Components[1].Modifier == nil ||
		frozen.Calculation.Components[1].Source.Ref == nil ||
		!frozen.Calculation.Components[1].Source.Ref.Equals(&frozen.Definition.Ref) ||
		frozen.Calculation.Components[1].Source.Name != frozen.Definition.Name ||
		*frozen.Calculation.Components[1].Modifier != frozen.Folded.AttackBonus ||
		frozen.Calculation.Total != frozen.Total {
		return nil, fmt.Errorf("%w: roll, bonus, and total do not match the frozen calculation", ErrBadFrozen)
	}
	if frozen.Offer.Audience != frozen.AttackerID {
		// The same rule the pose enforced, checked again on the way back in:
		// this slice poses to the roller and to nobody else, so a blob saying
		// otherwise is one this build could not have written.
		return nil, fmt.Errorf("%w: the offer names %q on %q's roll",
			ErrNotOffered, frozen.Offer.Audience, frozen.AttackerID)
	}

	machine := newStrikeMachine(&StrikeInput{
		AttackerID: frozen.AttackerID,
		TargetID:   frozen.TargetID,
		Definition: frozen.Definition,
		Roller:     in.Roller,
	})
	machine.resume = &strikeResume{frozen: frozen, answer: in.Answer}
	return machine, nil
}

// gatherPostRollOffers folds the offer chain on the interaction's own bus and
// hands back what was put on the table.
func gatherPostRollOffers(
	event *dnd5eEvents.PostRollOfferEvent,
	next func(context.Context, []dnd5eEvents.Offer) (Step, error),
) Gather {
	return Gather{
		name: "post roll offers",
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			chain := events.NewStagedChain[*dnd5eEvents.PostRollOfferEvent](combat.ModifierStages)
			modified, err := dnd5eEvents.PostRollOfferChain.On(bus).PublishWithChain(ctx, event, chain)
			if err != nil {
				return nil, fmt.Errorf("publish post roll offers: %w", err)
			}
			folded, err := modified.Execute(ctx, event)
			if err != nil {
				return nil, fmt.Errorf("execute post roll offers: %w", err)
			}
			return next(ctx, folded.Offers)
		},
	}
}

// pose stops the machine and hands its state out as bytes.
//
// # It refuses what it cannot freeze, loudly
//
// Two refusals, and both are the shelf being named rather than guessed at.
// An offer whose audience is not the roller is a window this slice has not
// designed a freeze for — the first one posed to somebody else is Cutting
// Words, and it arrives with its own design. More than one offer on one roll
// is two questions about one number, which is the same shelf item from the
// other side.
func (m *strikeMachine) pose(
	folded dnd5eEvents.AttackChainEvent, roll int, offers []dnd5eEvents.Offer,
) (Step, error) {
	if len(offers) > 1 {
		return nil, fmt.Errorf("%w: %d offers on one roll, and this build poses one question",
			ErrNotOffered, len(offers))
	}
	offer := offers[0]
	if offer.Audience != m.outcome.AttackerID {
		return nil, fmt.Errorf("%w: %q offered on %q's roll, and only the roller is asked here",
			ErrNotOffered, offer.Audience, m.outcome.AttackerID)
	}
	if offer.Ref == nil || offer.Die == "" {
		return nil, fmt.Errorf("%w: an offer with no ref or no die is nothing to ask about", ErrNotOffered)
	}

	frozen, err := json.Marshal(frozenStrike{
		Kind:        frozenStrikeKind,
		Version:     frozenStrikeVersion,
		AttackerID:  m.outcome.AttackerID,
		TargetID:    m.outcome.TargetID,
		Definition:  m.in.Definition.Clone(),
		Folded:      folded,
		Roll:        roll,
		Total:       m.outcome.Total,
		Calculation: dnd5eEvents.CloneRollCalculation(m.outcome.Calculation),
		Offer:       offer,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: freeze strike: %v", ErrBadFrozen, err)
	}

	return Pose{
		Ask: Ask{
			Audience: offer.Audience,
			Offer:    offer,
			Options:  []string{string(OfferSpend), string(OfferKeep)},
			Roll:     roll,
			Total:    m.outcome.Total,
		},
		Frozen: frozen,
	}, nil
}

// resumeStep is the first step of a resumed strike: apply the answer, then
// carry on from exactly where the pose was.
func (m *strikeMachine) resumeStep() Step {
	frozen := m.resume.frozen

	// The outcome is rebuilt from the frozen half rather than recomputed. This
	// is the machine's whole state at the moment it stopped.
	m.outcome = StrikeOutcome{
		AttackerID:  frozen.AttackerID,
		TargetID:    frozen.TargetID,
		Roll:        frozen.Roll,
		Total:       frozen.Total,
		Calculation: dnd5eEvents.CloneRollCalculation(frozen.Calculation),
		TargetAC:    frozen.Folded.TargetAC,
		Folded:      frozen.Folded,
	}

	return Gather{
		name: fmt.Sprintf("answer %s for %s", frozen.Offer.Ref.String(), frozen.Offer.Audience),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			if m.resume.answer == OfferSpend {
				if err := m.spendOffer(ctx, bus); err != nil {
					return nil, err
				}
			}

			// Decided against whatever the total now is — and through the
			// ARITHMETIC BRANCH ONLY. A d6 added to a natural 1 still misses
			// and a natural 20 was already a hit, because settleHit reads the
			// d20 the fold rolled rather than the number it adds up to.
			m.settleHit(frozen.Roll, frozen.Folded)

			hasAdvantage := len(frozen.Folded.AdvantageSources) > 0
			hasDisadvantage := len(frozen.Folded.DisadvantageSources) > 0
			return m.afterOffers(frozen.Folded, frozen.Roll, hasAdvantage, hasDisadvantage), nil
		},
	}
}

// spendOffer rolls the offered die, adds its face, and tells whoever offered
// it.
//
// THE MACHINE ROLLS. The effect that offered the die never sees it: it is told
// the face afterwards, on OfferTakenTopic, which is the same shape a reaction's
// condition is told its reaction fired. That is what makes "spent when taken"
// a mechanism rather than a promise — an offer nobody takes publishes nothing
// and the die stays in hand.
func (m *strikeMachine) spendOffer(ctx context.Context, bus events.EventBus) error {
	offer := m.resume.frozen.Offer

	size, err := parseDieSize(offer.Die)
	if err != nil {
		return fmt.Errorf("%w: offered die: %v", ErrBadFrozen, err)
	}
	face, err := m.in.Roller.Roll(ctx, size)
	if err != nil {
		return fmt.Errorf("roll offered die: %w", err)
	}

	component := dnd5eEvents.RollComponent{
		Source: dnd5eEvents.RollSource{Ref: cloneCoreRef(offer.Ref), Name: offer.Name},
		Dice: &dnd5eEvents.DiceTrace{
			Notation: dice.SimplePool(1, size, 0).Notation(), DieSize: size,
			OriginalRolls: []int{face}, FinalRolls: []int{face}, Subtotal: face,
		},
	}
	m.outcome.Calculation.Components = append(m.outcome.Calculation.Components, component)
	m.outcome.Calculation.Total += face
	if err := dnd5eEvents.ValidateRollCalculation(m.outcome.Calculation); err != nil {
		return fmt.Errorf("append offered die: %w", err)
	}
	m.outcome.Total = m.outcome.Calculation.Total

	if err := dnd5eEvents.OfferTakenTopic.On(bus).Publish(ctx, dnd5eEvents.OfferTakenEvent{
		Audience: offer.Audience,
		Ref:      offer.Ref,
		Face:     face,
	}); err != nil {
		return fmt.Errorf("publish offer taken: %w", err)
	}
	return nil
}
