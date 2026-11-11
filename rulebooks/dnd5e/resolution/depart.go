// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// DepartedReason is the removal reason a departure writes: on each effect of
// another member's concentration that comes off the leaver, and on a hold
// that ends because the leaver was all it still held.
const DepartedReason = "departed"

// DepartInput is a character leaving a run, and the run's other sheets.
type DepartInput struct {
	// Character is the leaver's record, not a live sheet. It is cloned before
	// it is loaded.
	Character *character.Data

	// Others are the other sheets of the run, every one of them (R3): whoever
	// holds a concentration whose effect sits on the leaver must be here, so
	// the hold can be told. They are cloned before they are loaded.
	Others []Participant
}

// DepartOutput is the leaver's record and every other sheet the departure
// changed.
type DepartOutput struct {
	// Character is the leaver's record after every held effect came off it.
	Character *character.Data

	// Ended are the effects that came off the leaver, each a condition removal
	// with reason [DepartedReason], that no break accounts for: an effect of a
	// hold that ended is in that break's Removed, once.
	Ended []encounter.ActivationResult

	// ConcentrationBreaks are the holds the departure ended — the leaver's
	// own, and another's whose last target was the leaver — each with reason
	// [DepartedReason], the caster to tell, and every effect that came off
	// with it: the same record a Resolve or a ShortRest returns.
	// A hold that continues for other targets is not here; its caster is in
	// DirtyCharacters or DirtyMonsters.
	ConcentrationBreaks []encounter.ConcentrationBreak

	// DirtyCharacters and DirtyMonsters are the [DepartInput.Others] the
	// departure changed, and only those: a caster whose hold no longer names
	// the leaver or ended, and a target of the leaver's own holds.
	DirtyCharacters []*character.Data
	DirtyMonsters   []*monster.Data
}

// Depart settles what concentration ties a leaving character to the run.
//
// Every effect another member's concentration holds on the leaver comes off
// it, and the hold is told: a hold that still holds an effect on somebody
// else continues, naming only them; a hold whose every effect was on the
// leaver ends, with reason [DepartedReason]. Every hold the leaver holds ends
// too, with the same reason, and its effects come off every remaining target:
// the run settles what it put on its members, or a target's own departure
// would later refuse for a caster who is gone. An effect on the leaver that
// no hold names is not touched.
//
// Returns [ErrNilInput], or [ErrBadParticipant] for a record that is not one
// sheet, a leaver also among the others, an effect of a concentration spell
// on the leaver whose caster was not passed in, or a hold of the leaver's
// reaching a member not passed in — refused before anything changes. No runtime character or event bus crosses this data
// boundary.
func Depart(ctx context.Context, in *DepartInput) (*DepartOutput, error) {
	return departOn(ctx, in, newSurface(events.NewEventBus()))
}

// departOn is Depart with its transient surface supplied for lifecycle tests.
// It stays unexported so a caller cannot retain the operation's bus.
func departOn(
	ctx context.Context, in *DepartInput, surf *surface,
) (out *DepartOutput, err error) {
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

	one := Participant{Character: cloneCharacterData(in.Character)}
	participants, err := castWithOthers(one, in.Others)
	if err != nil {
		return nil, err
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{Participants: participants, Roller: refusingRoller{}})
	if err != nil {
		return nil, err
	}

	ctx = installTruth(ctx, nil, cast, nil)

	leaver, ok := cast.Character(one.ID())
	if !ok {
		return nil, fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, one.ID())
	}

	held, err := heldOn(one.Character, cast)
	if err != nil {
		return nil, err
	}

	ends, err := collectConcentrationEnds(ctx, surf.inner)
	if err != nil {
		return nil, err
	}
	removals, err := collectRemovals(ctx, surf.inner, one.ID())
	if err != nil {
		return nil, errors.Join(err, ends.stop(ctx))
	}
	releaseErr := release(ctx, surf.inner, one.ID(), held, cast.applications)
	if stopErr := errors.Join(ends.stop(ctx), removals.stop(ctx)); stopErr != nil {
		return nil, errors.Join(releaseErr, stopErr)
	}
	if releaseErr != nil {
		return nil, releaseErr
	}

	left, err := leaver.ToData()
	if err != nil {
		return nil, fmt.Errorf("resolution: depart %q: %w", one.ID(), err)
	}
	var ownerAddresses []dnd5eEvents.ConditionAddress
	for _, found := range held {
		ownerAddresses = append(ownerAddresses, found.owner)
	}
	ended, err := removals.endedOutside(ends.facts, ownerAddresses)
	if err != nil {
		return nil, err
	}

	breaks, err := breaksFrom(ends.facts, nil)
	if err != nil {
		return nil, err
	}

	out = &DepartOutput{Character: left, Ended: ended, ConcentrationBreaks: breaks}
	dirty, err := dirtyCharacters(cast)
	if err != nil {
		return nil, err
	}
	for _, record := range dirty {
		if record.ID != one.ID() {
			out.DirtyCharacters = append(out.DirtyCharacters, record)
		}
	}
	out.DirtyMonsters = dirtyMonsters(cast)

	return out, nil
}

// castWithOthers validates and clones the cast an entry with others loads:
// the one acting first, then each other.
func castWithOthers(one Participant, others []Participant) ([]Participant, error) {
	if err := one.validate(); err != nil {
		return nil, err
	}
	participants := []Participant{one}
	for _, other := range others {
		if err := other.validate(); err != nil {
			return nil, err
		}
		if other.ID() == one.ID() {
			return nil, fmt.Errorf("%w: %q is also among the others", ErrBadParticipant, one.ID())
		}
		if other.Character != nil {
			other = Participant{Character: cloneCharacterData(other.Character)}
		} else {
			clone, err := cloneMonsterData(other.Monster)
			if err != nil {
				return nil, err
			}
			other = Participant{Monster: clone}
		}
		participants = append(participants, other)
	}
	return participants, nil
}

// cloneMonsterData is an owned copy of a monster record, through its own
// persisted form.
func cloneMonsterData(in *monster.Data) (*monster.Data, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("%w: monster %q: %w", ErrBadParticipant, in.ID, err)
	}
	out := &monster.Data{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("%w: monster %q: %w", ErrBadParticipant, in.ID, err)
	}
	return out, nil
}

// heldEffects is a detached protocol view of one application's affected entries.
type heldEffects struct {
	owner       dnd5eEvents.ConditionAddress
	application string
	name        string
	live        []dnd5eEvents.ConditionAddress
	onLeave     []dnd5eEvents.ConditionAddress
	pending     []string
	own         bool
}

// heldOn reads exact current membership, never a condition class or ref-as-role.
// The interaction binding already validated every live/pending holder.
func heldOn(leaver *character.Data, cast *Participants) ([]heldEffects, error) {
	var held []heldEffects
	named := map[dnd5eEvents.ConditionAddress]bool{}
	for _, id := range cast.order {
		for _, condition := range conditionsOf(cast, id) {
			provider, ok := condition.(interface {
				LinkedApplicationInfo() conditions.LinkedApplicationInfo
			})
			if !ok {
				continue
			}
			info := provider.LinkedApplicationInfo()
			if info.Role != conditions.LinkedCoordinator {
				continue
			}
			display, err := conditions.DescribeCondition(&conditions.DescribeConditionInput{Condition: condition})
			if err != nil {
				return nil, err
			}
			found := heldEffects{owner: info.Owner, application: info.ID, name: display.Display.Name, own: id == leaver.ID}
			for _, member := range info.Members {
				if slices.Contains(info.LiveMembers, member.Key) {
					found.live = append(found.live, member.Address)
					if member.Address.MemberID == leaver.ID {
						found.onLeave = append(found.onLeave, member.Address)
						named[member.Address] = true
					}
				}
				if member.Address.MemberID == leaver.ID && slices.Contains(info.PendingMembers, member.Key) {
					found.pending = append(found.pending, member.Key)
				}
			}
			if found.own || len(found.onLeave) > 0 || len(found.pending) > 0 {
				held = append(held, found)
			}
		}
	}
	// Current adopted payloads have no backlink. Preserve their explicit source
	// check; linked payload integrity was checked by the root binding already.
	for _, effect := range peekEffects(leaver) {
		if named[effect.address] || effect.address.SourceID == "" || effect.address.SourceID == leaver.ID {
			continue
		}
		if _, ok := cast.Character(effect.address.SourceID); ok {
			continue
		}
		if _, ok := cast.Monster(effect.address.SourceID); ok {
			continue
		}
		concentration, err := concentrationSpell(effect.sourceRef)
		if err != nil {
			return nil, err
		}
		if concentration {
			return nil, fmt.Errorf("%w: %s on %q is held by %q, who was not passed in", ErrBadParticipant, effect.address.ConditionRef, leaver.ID, effect.address.SourceID)
		}
	}
	return held, nil
}

// release ends a departing maintainer or releases only the departing recipient.
// Pending entries resolve without an effect rather than remaining dangling links.
func release(ctx context.Context, bus events.EventBus, leaverID string, held []heldEffects, binding *conditions.ApplicationBinding) error {
	removals := dnd5eEvents.ConditionRemovedTopic.On(bus)
	for _, found := range held {
		if found.own || (len(found.onLeave) > 0 && len(found.onLeave) == len(found.live)) {
			owner := found.owner
			if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{MemberID: owner.MemberID, ConditionRef: owner.ConditionRef, SourceID: owner.SourceID, ApplicationID: found.application, Reason: DepartedReason}); err != nil {
				return fmt.Errorf("resolution: depart %q: end %s: %w", leaverID, found.name, err)
			}
			continue
		}
		for _, key := range found.pending {
			if binding == nil {
				return fmt.Errorf("%w: departure requires application binding", ErrBadParticipant)
			}
			if _, err := binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: found.application, Key: key}); err != nil {
				return err
			}
		}
		for _, child := range found.onLeave {
			if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{MemberID: child.MemberID, ConditionRef: child.ConditionRef, SourceID: child.SourceID, ApplicationID: found.application, Reason: DepartedReason}); err != nil {
				return fmt.Errorf("resolution: depart %q: release %s: %w", leaverID, child.ConditionRef, err)
			}
		}
	}
	return nil
}

// conditionsOf is a cast member's live conditions, character or monster.
func conditionsOf(cast *Participants, id string) []dnd5eEvents.ConditionBehavior {
	if ch, ok := cast.Character(id); ok {
		return ch.GetConditions()
	}
	if m, ok := cast.Monster(id); ok {
		return m.GetConditions()
	}
	return nil
}

// peekedEffect is a persisted effect's address and the ref of what made it.
type peekedEffect struct {
	address   dnd5eEvents.ConditionAddress
	sourceRef *core.Ref
}

// peekEffects reads each persisted effect on a record for its address and its
// source ref (core.Ref reads either persisted form, string or object). A blob
// that does not carry them is skipped: it names no caster.
func peekEffects(data *character.Data) []peekedEffect {
	var out []peekedEffect
	for _, raw := range data.Conditions {
		var head struct {
			Ref       *core.Ref `json:"ref"`
			SourceID  string    `json:"source_id"`
			SourceRef *core.Ref `json:"source_ref"`
		}
		if json.Unmarshal(raw, &head) != nil || head.Ref == nil {
			continue
		}
		out = append(out, peekedEffect{
			address: dnd5eEvents.ConditionAddress{
				MemberID: data.ID, ConditionRef: head.Ref.String(), SourceID: head.SourceID,
			},
			sourceRef: head.SourceRef,
		})
	}
	return out
}

// concentrationSpell reports whether ref names a spell this build casts as a
// concentration spell. Content declares it; nothing here names a spell.
func concentrationSpell(ref *core.Ref) (bool, error) {
	if ref == nil || ref.Module != refs.Module || ref.Type != refs.TypeSpells {
		return false, nil
	}
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: ref.ID})
	if definition == nil || definition.Cast == nil {
		return false, nil
	}
	described, err := effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: definition.Cast.Maintenance})
	if err != nil {
		return false, err
	}
	return described.Concentration, nil
}
