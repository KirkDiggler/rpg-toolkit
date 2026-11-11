// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
)

func applicationParticipants(cast *Participants) []conditions.ParticipantConditions {
	out := make([]conditions.ParticipantConditions, 0, len(cast.order))
	for _, id := range cast.order {
		out = append(out, conditions.ParticipantConditions{MemberID: id, Conditions: heldConditions(cast, id)})
	}
	return out
}

// Mutable interactions load the whole supplied universe before binding any
// behavior. Local projections may inspect one sheet without its remote owners;
// they cannot deliver, finalize or end an unbound application.
func attachAll(ctx context.Context, surf *surface, in *attachAllInput) (*Participants, error) {
	if in.LocalProjection {
		return attachAllLocal(ctx, surf, in)
	}
	if in.DropUnreadable {
		return nil, fmt.Errorf("resolution: lenient loading is only available to local projections")
	}
	ordered := append([]Participant(nil), in.Participants...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID() < ordered[j].ID() })
	cast := &Participants{characters: map[string]*character.Character{}, monsters: map[string]*monster.Monster{}}
	for _, participant := range ordered {
		id := participant.ID()
		switch {
		case participant.Character != nil:
			ch, err := character.Load(ctx, participant.Character)
			if err != nil {
				return nil, fmt.Errorf("resolution: load character %q: %w", id, err)
			}
			cast.characters[id] = ch
		case participant.Monster != nil:
			m, err := monstertraits.LoadMonster(ctx, participant.Monster)
			if err != nil {
				return nil, fmt.Errorf("resolution: load monster %q: %w", id, err)
			}
			cast.monsters[id] = m
		default:
			return nil, fmt.Errorf("resolution: participant %q has no sheet", id)
		}
		cast.order = append(cast.order, id)
	}
	for _, id := range cast.order {
		held, err := concentrationHeldBy(cast, id)
		if err != nil {
			return nil, err
		}
		if held.Held {
			if _, err := nativeMaintenanceSpell(held.Info.SourceRef, held.Info.SourceName); err != nil {
				return nil, err
			}
		}
	}
	binding, err := conditions.BindApplications(&conditions.BindApplicationsInput{Participants: applicationParticipants(cast), Bus: surf.inner})
	if err != nil {
		return nil, fmt.Errorf("%w: bind applications: %w", ErrBadParticipant, err)
	}
	surf.ledger.applications = binding
	cast.applications = binding
	for _, id := range cast.order {
		view := surf.forParticipant(id)
		if ch, ok := cast.characters[id]; ok {
			if err := character.AttachWithRoller(ctx, ch, view, in.Roller); err != nil {
				return nil, fmt.Errorf("resolution: attach character %q: %w", id, err)
			}
		} else if err := monstertraits.AttachMonster(ctx, cast.monsters[id], view, in.Roller); err != nil {
			return nil, fmt.Errorf("resolution: attach monster %q: %w", id, err)
		}
	}
	return cast, nil
}
