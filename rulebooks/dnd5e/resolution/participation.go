// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// ParticipantParticipation is the root rulebook's complete participation
// answer for one persisted participant.
type ParticipantParticipation struct {
	// Member is the participant's ID.
	Member string

	// Participation is combat's provider-owned policy for the derived life
	// state. Callers consume it; resolution does not reinterpret it.
	Participation combat.Participation

	// DeathSaves is the provider projection of authoritative progress for a
	// Dying, Stabilized, or Dead character. It is nil for Conscious characters
	// and for monsters.
	DeathSaves *character.DeathSaveProgress
}

// ParticipationInput is the ordered persisted cast to assess.
type ParticipationInput struct {
	Participants []Participant
}

// ParticipationOutput carries exactly one answer per input participant, in
// input order.
type ParticipationOutput struct {
	Members []ParticipantParticipation
}

// Participation attaches the supplied records on one transient bus, asks the
// root rulebook for their narrow participation/progress data, and tears every
// registration down before returning. It never asks for a full StatusView:
// display catalogs are unrelated to combat participation, so a valid, loadable
// condition such as Shield may answer here even though the root display
// projection deliberately refuses to describe it.
//
// It remains a lenient read at the record boundary: character effects that this
// build cannot parse are audibly dropped under the existing Standing policy,
// while unreadable monster traits still refuse because their loader has no
// lenient mode.
func Participation(ctx context.Context, in *ParticipationInput) (*ParticipationOutput, error) {
	return participationOn(ctx, in, newSurface(events.NewEventBus()))
}

// participationOn is Participation with its transient surface supplied for
// lifecycle and Standing compatibility tests.
func participationOn(
	ctx context.Context, in *ParticipationInput, surf *surface,
) (out *ParticipationOutput, err error) {
	defer func() {
		tearErr := surf.teardown(ctx)
		if tearErr == nil {
			return
		}

		out = nil
		if err != nil {
			err = errors.Join(err, tearErr)
			return
		}
		err = fmt.Errorf("resolution: teardown: %w", tearErr)
	}()

	if in == nil {
		return nil, ErrNilInput
	}
	for _, participant := range in.Participants {
		if err := participant.validate(); err != nil {
			return nil, err
		}
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants:   in.Participants,
		Roller:         refusingRoller{},
		DropUnreadable: true,
	})
	if err != nil {
		return nil, err
	}

	ctx = installTruth(ctx, nil, cast)

	members := make([]ParticipantParticipation, 0, len(in.Participants))
	for _, participant := range in.Participants {
		id := participant.ID()
		if ch, ok := cast.Character(id); ok {
			view := ch.ParticipationView()
			member := ParticipantParticipation{
				Member:        id,
				Participation: combat.ParticipationFor(view.LifeState),
			}
			if view.DeathSaves != nil {
				progress := *view.DeathSaves
				member.DeathSaves = &progress
			}
			members = append(members, member)
			continue
		}

		if monster, ok := cast.Monster(id); ok {
			state := combat.ClassifyLifeState(combat.LifeStateInput{
				Kind: combat.CombatantKindMonster,
				Down: combat.IsDown(monster),
			})
			members = append(members, ParticipantParticipation{
				Member:        id,
				Participation: combat.ParticipationFor(state),
			})
			continue
		}

		return nil, fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, id)
	}

	return &ParticipationOutput{Members: members}, nil
}
