package resolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// reconcileFogMembership makes persisted In Fog conditions agree with active
// sight areas and current placement. Geometry is authoritative; the condition
// is the source-qualified snapshot of that fact for a participant.
func reconcileFogMembership(ctx context.Context, bus events.EventBus, cast *Participants, room spatial.Room, areas []encounter.SightAreaData) error {
	active := make(map[string]encounter.SightAreaData, len(areas))
	for _, area := range areas {
		if area.Ref == refs.Spells.FogCloud().String() {
			active[area.ID] = area
		}
	}

	for _, id := range cast.IDs() {
		entity, err := cast.entity(id)
		if err != nil {
			return err
		}
		position, placed := room.GetEntityPosition(id)
		inside := make(map[string]bool, len(active))
		for areaID, area := range active {
			if placed && encounter.SightAreaContains(area, position, room.GetGrid()) {
				inside[areaID] = true
			}
		}
		var currentConditions []dnd5eEvents.ConditionBehavior
		if character, ok := cast.Character(id); ok {
			currentConditions = character.GetConditions()
		} else if monster, ok := cast.Monster(id); ok {
			currentConditions = monster.GetConditions()
		}
		for _, condition := range currentConditions {
			address, ok := condition.(dnd5eEvents.ConditionAddressProvider)
			if !ok || condition.Ref().String() != refs.Conditions.InFog().String() {
				continue
			}
			current := address.ConditionAddress()
			if inside[areaIDForCondition(current.SourceID, active)] {
				delete(inside, areaIDForCondition(current.SourceID, active))
				continue
			}
			if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
				MemberID: id, ConditionRef: current.ConditionRef, SourceID: current.SourceID, Reason: "left fog cloud",
			}); err != nil {
				return err
			}
		}
		areaIDs := make([]string, 0, len(inside))
		for areaID := range inside {
			areaIDs = append(areaIDs, areaID)
		}
		sort.Strings(areaIDs)
		for _, areaID := range areaIDs {
			area := active[areaID]
			membership, err := conditions.NewInFogCondition(conditions.NewInFogConditionInput{
				MemberID: id, SourceID: opaqueFogSourceID(area.ID), SourceRef: refs.Spells.FogCloud(),
			})
			if err != nil {
				return err
			}
			if err := dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
				Target: entity, Type: dnd5eEvents.ConditionType(membership.Ref().ID),
				Source: dnd5eEvents.ConditionSourceSpell, Condition: membership,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func opaqueFogSourceID(areaID string) string {
	sum := sha256.Sum256([]byte(areaID))
	return hex.EncodeToString(sum[:16])
}

func areaIDForCondition(sourceID string, active map[string]encounter.SightAreaData) string {
	for areaID := range active {
		if opaqueFogSourceID(areaID) == sourceID {
			return areaID
		}
	}
	return sourceID
}

// FogMembershipInput supplies the participants and current encounter geometry
// for a host movement or reload reconciliation.
type FogMembershipInput struct {
	Participants []Participant
	Room         spatial.Room
	Areas        []encounter.SightAreaData
	Roller       dice.Roller
}

// FogMembershipOutput contains sheets changed by membership reconciliation.
type FogMembershipOutput struct {
	DirtyCharacters []*character.Data
	DirtyMonsters   []*monster.Data
}

// ReconcileFogMembership loads the supplied sheets on one resolver-owned bus,
// reconciles source-qualified In Fog memberships from geometry, and returns
// only changed sheets for the host to persist.
func ReconcileFogMembership(ctx context.Context, in *FogMembershipInput) (*FogMembershipOutput, error) {
	if in == nil || in.Room == nil || in.Roller == nil {
		return nil, fmt.Errorf("fog membership: invalid input")
	}
	surf := newSurface(events.NewEventBus())
	cast, err := attachAll(ctx, surf, &attachAllInput{Participants: in.Participants, Roller: in.Roller})
	if err != nil {
		return nil, errors.Join(err, surf.teardown(ctx))
	}
	if err := reconcileFogMembership(ctx, surf.inner, cast, in.Room, in.Areas); err != nil {
		return nil, errors.Join(err, surf.teardown(ctx))
	}
	if err := surf.teardown(ctx); err != nil {
		return nil, err
	}
	return &FogMembershipOutput{DirtyCharacters: dirtyCharacters(cast), DirtyMonsters: dirtyMonsters(cast)}, nil
}
