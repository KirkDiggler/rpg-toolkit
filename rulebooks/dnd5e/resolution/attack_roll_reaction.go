package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ReactionUse accepts the offered pre-roll reaction.
const ReactionUse = "use"

func (m *strikeMachine) poseBeforeRoll(folded dndEvents.AttackChainEvent) (Step, error) {
	offer := folded.BeforeRollOffers[0]
	frozen, err := json.Marshal(frozenStrike{Kind: frozenStrikeKind, Version: frozenStrikeVersion, AttackerID: m.in.AttackerID, TargetID: m.in.TargetID, Definition: m.in.Definition, Folded: folded, Outcome: &m.outcome, BeforeRoll: &offer})
	if err != nil {
		return nil, fmt.Errorf("%w: freeze pre-roll reaction: %v", ErrBadFrozen, err)
	}
	return Pose{BeforeRoll: true, Ask: Ask{Audience: offer.ReactorID, Offer: dndEvents.Offer{Audience: offer.ReactorID, Ref: &offer.Ref, Name: offer.Name}, Choices: []Choice{{ID: ReactionUse, Label: "Use " + offer.Name}}, Options: []string{ReactionUse, ReactionDecline}}, Frozen: frozen}, nil
}

func (m *strikeMachine) resumeBeforeRoll() Step {
	return Gather{name: "answer pre-roll reaction", run: func(ctx context.Context, _ events.EventBus) (Step, error) {
		frozen := m.resume.frozen
		m.outcome = *frozen.Outcome
		// Prior follow-ups were persisted with the initial interaction.
		m.outcome.FollowUps = nil
		folded := frozen.Folded
		offer := frozen.BeforeRoll
		if m.resume.answer == OfferSpend {
			price := &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionReaction: 1}, Pools: map[coreResources.ResourceKey]int{coreResources.ResourceKey(offer.ResourceKey): 1}}
			if err := payAtTheDoor(ctx, &Cost{PayerID: offer.ReactorID, Profile: price}, m.cast); err != nil {
				return nil, err
			}
			folded.DisadvantageSources = append(folded.DisadvantageSources, offer.Disadvantage)
		}
		// The fold is never replayed. Only this answered offer is removed.
		folded.BeforeRollOffers = folded.BeforeRollOffers[1:]
		return m.afterAttackChain(ctx, folded)
	}}
}
