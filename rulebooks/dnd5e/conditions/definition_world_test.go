// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// definitionWorld supplies custody and transport only. All configured behavior
// comes from public rulebook APIs, not a simulated item or effect engine.
type definitionWorld struct {
	sheets  []*character.Character
	bus     events.EventBus
	binding *conditions.ApplicationBinding
}

func newDefinitionWorld() (*definitionWorld, error) {
	w := &definitionWorld{}
	for _, id := range []string{"A", "B", "C"} {
		c, err := character.Load(context.Background(), &character.Data{ID: id, PlayerID: "player-" + id, Name: id, Level: 1, ProficiencyBonus: 2, ClassID: classes.Fighter, RaceID: races.Human, HitPoints: 100, MaxHitPoints: 100, AbilityScores: shared.AbilityScores{abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14, abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8}})
		if err != nil {
			return nil, err
		}
		w.sheets = append(w.sheets, c)
	}
	if err := w.bind(); err != nil {
		return nil, err
	}
	return w, nil
}
func (w *definitionWorld) bind() error {
	w.bus = events.NewEventBus()
	b, err := conditions.BindApplications(&conditions.BindApplicationsInput{Participants: bindingParticipants(w.sheets), Bus: w.bus})
	if err != nil {
		return err
	}
	w.binding = b
	for _, c := range w.sheets {
		if err := character.Attach(context.Background(), c, w.bus); err != nil {
			return err
		}
	}
	return nil
}
func (w *definitionWorld) holder(id string) *character.Character {
	for _, c := range w.sheets {
		if c.GetID() == id {
			return c
		}
	}
	return nil
}
func (w *definitionWorld) close() error {
	if w.binding == nil {
		return nil
	}
	if err := w.binding.Close(context.Background()); err != nil {
		return err
	}
	for _, c := range w.sheets {
		for _, effect := range c.GetConditions() {
			if err := effect.Remove(context.Background(), w.bus); err != nil {
				return err
			}
		}
	}
	w.binding = nil
	return nil
}
func (w *definitionWorld) reload() error {
	if err := w.binding.CheckComplete(); err != nil {
		return err
	}
	data := make([]*character.Data, 0, len(w.sheets))
	for _, c := range w.sheets {
		d, err := c.ToData()
		if err != nil {
			return err
		}
		data = append(data, d)
	}
	if err := w.close(); err != nil {
		return err
	}
	w.sheets = nil
	for _, d := range data {
		c, err := character.Load(context.Background(), d)
		if err != nil {
			return err
		}
		w.sheets = append(w.sheets, c)
	}
	return w.bind()
}
func (w *definitionWorld) prepare(defs *conditions.CompiledDefinitions, id, user, target, application string) (*conditions.PreparedApplication, conditions.ApplicationSpec, error) {
	bound, err := conditions.BindDefinition(&conditions.BindDefinitionInput{Definitions: defs, DefinitionID: id, UserID: user, RecipientID: target, ApplicationID: application, Origin: definitionOrigin()})
	if err != nil {
		return nil, conditions.ApplicationSpec{}, err
	}
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: application, CoordinatorID: bound.CoordinatorID, Spec: bound.Spec, Origin: definitionOrigin(), Existing: bindingParticipants(w.sheets)})
	return prepared, bound.Spec, err
}
func (w *definitionWorld) remove(address dndevents.ConditionAddress, application, reason string) error {
	return dndevents.ConditionRemovedTopic.On(w.bus).Publish(context.Background(), dndevents.ConditionRemovedEvent{MemberID: address.MemberID, ConditionRef: address.ConditionRef, SourceID: address.SourceID, ApplicationID: application, Reason: reason})
}
func (w *definitionWorld) owner(prepared *conditions.PreparedApplication) error {
	if replaced, ok := prepared.Replacement(); ok {
		if err := w.remove(replaced, replaced.SourceID, "recast"); err != nil {
			return err
		}
	}
	made, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	if err != nil {
		return err
	}
	address := made.Condition.(dndevents.ConditionAddressProvider).ConditionAddress()
	holder := w.holder(address.MemberID)
	if holder == nil {
		return fmt.Errorf("missing owner holder")
	}
	return dndevents.ConditionAppliedTopic.On(w.bus).Publish(context.Background(), dndevents.ConditionAppliedEvent{Target: holder, Condition: made.Condition, ApplicationID: prepared.ID()})
}
func (w *definitionWorld) deliver(prepared *conditions.PreparedApplication, finalize bool) (conditions.RecipientCandidate, error) {
	made, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
	if err != nil {
		return conditions.RecipientCandidate{}, err
	}
	holder := w.holder(made.Candidate.Address.MemberID)
	if holder == nil {
		return conditions.RecipientCandidate{}, fmt.Errorf("missing recipient holder")
	}
	if err := dndevents.ConditionAppliedTopic.On(w.bus).Publish(context.Background(), dndevents.ConditionAppliedEvent{Target: holder, Condition: made.Condition, ApplicationID: prepared.ID()}); err != nil {
		return conditions.RecipientCandidate{}, err
	}
	if finalize {
		_, err = w.binding.FinalizeRecipient(context.Background(), &conditions.FinalizeRecipientInput{ApplicationID: prepared.ID(), Key: made.Candidate.Key, Delivered: true})
	}
	return made.Candidate, err
}
func (w *definitionWorld) apply(prepared *conditions.PreparedApplication) error {
	if err := w.owner(prepared); err != nil {
		return err
	}
	if _, err := w.deliver(prepared, true); err != nil {
		return err
	}
	return w.binding.CheckComplete()
}
