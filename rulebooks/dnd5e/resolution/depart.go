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
	// with reason [DepartedReason].
	Ended []encounter.ActivationResult

	// DirtyCharacters and DirtyMonsters are the [DepartInput.Others] the
	// departure changed, and only those: a caster whose hold no longer names
	// the leaver, or whose hold ended because the leaver was its last target.
	DirtyCharacters []*character.Data
	DirtyMonsters   []*monster.Data
}

// Depart takes off a leaving character every effect another member's
// concentration holds on it, and tells each hold so.
//
// A hold that still holds an effect on somebody else continues, naming only
// them. A hold whose every effect was on the leaver ends, with reason
// [DepartedReason], and its caster comes back dirty. The leaver's own holds
// are not touched: today's Exit writes no sheet, so they leave with the
// leaver exactly as they are. An effect on the leaver that no hold names is
// not touched either.
//
// Returns [ErrNilInput], or [ErrBadParticipant] for a record that is not one
// sheet, a leaver also among the others, or an effect of a concentration
// spell on the leaver whose caster was not passed in — refused before
// anything changes. No runtime character or event bus crosses this data
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

	removals, err := collectRemovals(ctx, surf.inner, one.ID())
	if err != nil {
		return nil, err
	}
	releaseErr := release(ctx, surf.inner, one.ID(), held)
	if stopErr := removals.stop(ctx); stopErr != nil {
		return nil, errors.Join(releaseErr, stopErr)
	}
	if releaseErr != nil {
		return nil, releaseErr
	}

	left, err := leaver.ToData()
	if err != nil {
		return nil, fmt.Errorf("resolution: depart %q: %w", one.ID(), err)
	}
	ended, err := removals.endedOutside(nil)
	if err != nil {
		return nil, err
	}

	out = &DepartOutput{Character: left, Ended: ended}
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

// heldEffects is one hold and the effects of it that sit on the leaver.
type heldEffects struct {
	hold    *conditions.ConcentratingCondition
	onLeave []dnd5eEvents.ConditionAddress
}

// heldOn finds every hold in the cast that names an effect on the leaver, and
// refuses an effect of a concentration spell whose caster was not passed in.
func heldOn(leaver *character.Data, cast *Participants) ([]heldEffects, error) {
	var held []heldEffects
	named := map[dnd5eEvents.ConditionAddress]bool{}
	for _, id := range cast.order {
		if id == leaver.ID {
			continue
		}
		for _, condition := range conditionsOf(cast, id) {
			hold, ok := condition.(*conditions.ConcentratingCondition)
			if !ok {
				continue
			}
			found := heldEffects{hold: hold}
			for _, child := range hold.Children {
				if child.MemberID == leaver.ID {
					found.onLeave = append(found.onLeave, child)
					named[child] = true
				}
			}
			if len(found.onLeave) > 0 {
				held = append(held, found)
			}
		}
	}

	// An effect whose caster is not here cannot be told apart from an effect
	// nobody holds by looking for the hold. The spell it came from can: an
	// effect of a concentration spell is held by its caster, and leaving
	// without telling the caster would leave a hold naming a member gone.
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
		if concentrationSpell(effect.sourceRef) {
			return nil, fmt.Errorf("%w: %s on %q is held by %q, who was not passed in",
				ErrBadParticipant, effect.address.ConditionRef, leaver.ID, effect.address.SourceID)
		}
	}

	return held, nil
}

// release takes each held effect off the leaver, through the hold: a hold
// with nothing left but the leaver ends; any other drops the leaver's effects
// and continues.
func release(ctx context.Context, bus events.EventBus, leaverID string, held []heldEffects) error {
	removals := dnd5eEvents.ConditionRemovedTopic.On(bus)
	for _, found := range held {
		targets := found.hold.Children
		if len(found.onLeave) == len(targets) {
			owner := found.hold.ConditionAddress()
			if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
				MemberID: owner.MemberID, ConditionRef: owner.ConditionRef, SourceID: owner.SourceID,
				Reason: DepartedReason,
			}); err != nil {
				return fmt.Errorf("resolution: depart %q: end %s: %w", leaverID, found.hold.SpellName, err)
			}
			continue
		}
		for _, child := range slices.Clone(found.onLeave) {
			if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
				MemberID: child.MemberID, ConditionRef: child.ConditionRef, SourceID: child.SourceID,
				Reason: DepartedReason,
			}); err != nil {
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
// source ref. A blob that does not carry them is skipped: it names no caster.
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
func concentrationSpell(ref *core.Ref) bool {
	if ref == nil || ref.Module != refs.Module || ref.Type != refs.TypeSpells {
		return false
	}
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Spell(ref.ID)})
	return definition != nil && definition.Cast != nil && definition.Cast.Concentration != nil
}
