// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// definitionWorld supplies custody and transport only. All configured behavior
// comes from public rulebook APIs, not a simulated item or effect engine.
type definitionWorld struct {
	// Optional test observer registered before keepers, not a runtime/data callback.
	beforeAttach func(events.EventBus) error
	sheets       []*character.Character
	bus          events.EventBus
	binding      *conditions.ApplicationBinding
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
	if w.beforeAttach != nil {
		if err := w.beforeAttach(w.bus); err != nil {
			return err
		}
	}
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
func (w *definitionWorld) reload() error { return w.reloadWithout("") }

func (w *definitionWorld) reloadWithout(omit string) error {
	if err := w.binding.CheckComplete(); err != nil {
		return err
	}
	data := make([]*character.Data, 0, len(w.sheets))
	for _, c := range w.sheets {
		if c.GetID() == omit {
			continue
		}
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

// Authoritative fixture facts are captured as values, not retained live target
// objects. An observer test keeps a Clone of this frame and does not refresh it.
func (w *definitionWorld) frame(actor, target string) (contributions.Frame, error) {
	f := contributions.Frame{Actor: actor, Target: contributions.Known(target), ActorClassLevels: contributions.KnownClassLevels(contributions.ClassLevel{Class: classes.Fighter, Levels: 1}), Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack), Ability: contributions.Known(abilities.STR), Melee: contributions.Known(true), WeaponPool: contributions.Known(true), Opportunity: contributions.Known(false)}}
	for _, holder := range w.sheets {
		held := contributions.MemberHeld{Member: holder.GetID(), Conditions: []contributions.HeldCondition{}}
		for _, condition := range holder.GetConditions() {
			address := conditions.ConditionAddressOf(holder.GetID(), condition)
			held.Conditions = append(held.Conditions, contributions.HeldCondition{Ref: address.ConditionRef, SourceID: address.SourceID})
		}
		f.Held = append(f.Held, held)
		projected, err := conditions.ProjectHeldEffects(&conditions.ProjectHeldEffectsInput{HolderID: holder.GetID(), Conditions: holder.GetConditions()})
		if err != nil {
			return contributions.Frame{}, err
		}
		f.HeldEffects = append(f.HeldEffects, projected.Details...)
	}
	return f, nil
}
func fixtureDamage(amount int, kind damage.Type) []dndevents.DamageComponent {
	ref := core.Ref{Module: "example", Type: "actions", ID: "hit"}
	return []dndevents.DamageComponent{{Source: dndevents.DamageSourceWeapon, DamageType: kind, Roll: dndevents.RollComponent{Source: dndevents.RollSource{Ref: &ref, Name: "Fixture hit"}, Modifier: &amount}}}
}
func (w *definitionWorld) damageBonus(actor, target string, ability abilities.Ability, melee, weapon bool) (*dndevents.DamageChainEvent, error) {
	frame, err := w.frame(actor, target)
	if err != nil {
		return nil, err
	}
	frame.Action.Ability = contributions.Known(ability)
	frame.Action.Melee = contributions.Known(melee)
	frame.Action.WeaponPool = contributions.Known(weapon)
	in := dndevents.DamageChainInput{AttackerID: actor, TargetID: target, Frame: frame, WeaponDamageType: damage.Slashing, Components: fixtureDamage(10, damage.Slashing)}
	ctx := context.Background()
	c, err := dndevents.DamageChain.On(w.bus).PublishWithChain(ctx, dndevents.NewDamageChainEvent(in), events.NewStagedChain[*dndevents.DamageChainEvent](combat.ModifierStages))
	if err != nil {
		return nil, err
	}
	return c.Execute(ctx, dndevents.NewDamageChainEvent(in))
}

func (w *definitionWorld) incoming(target string, kind damage.Type, amount int) (*combat.SettleDamageOutput, error) {
	frame, err := w.frame("C", target)
	if err != nil {
		return nil, err
	}
	event, err := dndevents.NewIncomingDamageEvent(dndevents.IncomingDamageInput{TargetID: target, SourceID: "C", Frame: frame, Dealt: fixtureDamage(amount, kind)})
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	c, err := dndevents.IncomingDamageChain.On(w.bus).PublishWithChain(ctx, event.Clone(), events.NewStagedChain[*dndevents.IncomingDamageEvent](combat.ModifierStages))
	if err != nil {
		return nil, err
	}
	folded, err := c.Execute(ctx, event.Clone())
	if err != nil {
		return nil, err
	}
	if err := folded.CheckUnaltered(event); err != nil {
		return nil, err
	}
	return combat.SettleDamage(&combat.SettleDamageInput{Dealt: event.Dealt(), Reductions: folded.Reductions, Multipliers: folded.Multipliers})
}
func (w *definitionWorld) hit(target string, kind damage.Type, amount int) (*dndevents.DamageTakenEvent, error) {
	settled, err := w.incoming(target, kind, amount)
	if err != nil {
		return nil, err
	}
	instances := []combat.DamageInstance{}
	for _, part := range settled.Types {
		instances = append(instances, combat.DamageInstance{Amount: part.Taken, Type: string(part.Type)})
	}
	holder := w.holder(target)
	if holder == nil {
		return nil, fmt.Errorf("missing damage holder")
	}
	applied := holder.ApplyDamage(context.Background(), &combat.ApplyDamageInput{Instances: instances})
	fact := &dndevents.DamageTakenEvent{MemberID: target, Amount: applied.TotalDamage, DamageType: kind, DroppedToZero: applied.DroppedToZero}
	return fact, dndevents.DamageTakenTopic.On(w.bus).Publish(context.Background(), fact)
}
func (w *definitionWorld) turnEnd(who string) error {
	return dndevents.TurnEndTopic.On(w.bus).Publish(context.Background(), dndevents.TurnEndEvent{SubjectID: who})
}
func (w *definitionWorld) turnStart(who string) error {
	return dndevents.TurnStartTopic.On(w.bus).Publish(context.Background(), dndevents.TurnStartEvent{SubjectID: who})
}
func (w *definitionWorld) rest(who string) error {
	return dndevents.RestTopic.On(w.bus).Publish(context.Background(), dndevents.RestEvent{CharacterID: who, RestType: coreResources.ResetLongRest})
}
func (w *definitionWorld) miss(who string) error {
	_, err := dndevents.PostAttackRollChain.On(w.bus).PublishWithChain(context.Background(), &dndevents.PostAttackRollEvent{AttackerID: who}, events.NewStagedChain[*dndevents.PostAttackRollEvent](combat.ModifierStages))
	return err
}
func (w *definitionWorld) attack(frame contributions.Frame) (dndevents.AttackChainEvent, error) {
	target, _ := frame.Target.Get()
	e := dndevents.AttackChainEvent{AttackerID: frame.Actor, TargetID: target, Frame: frame}
	ctx := context.Background()
	c, err := dndevents.AttackChain.On(w.bus).PublishWithChain(ctx, e, events.NewStagedChain[dndevents.AttackChainEvent](combat.ModifierStages))
	if err != nil {
		return e, err
	}
	return c.Execute(ctx, e)
}
func (w *definitionWorld) coordinator(application string) (string, error) {
	for _, holder := range w.sheets {
		for _, condition := range holder.GetConditions() {
			if linked, ok := condition.(*conditions.LinkedCondition); ok {
				info := linked.LinkedApplicationInfo()
				if info.ID == application && info.Role == conditions.LinkedCoordinator {
					return holder.GetID(), nil
				}
			}
		}
	}
	return "", fmt.Errorf("application has no current coordinator")
}
func (w *definitionWorld) maintenance(holder, application string) (*conditions.MaintenanceInfo, error) {
	for _, condition := range w.holder(holder).GetConditions() {
		if linked, ok := condition.(*conditions.LinkedCondition); ok {
			info := linked.LinkedApplicationInfo()
			if info.ID == application && info.Role == conditions.LinkedCoordinator {
				return linked.MaintenanceInfo()
			}
		}
	}
	return nil, fmt.Errorf("application has no current maintenance")
}
func (w *definitionWorld) saveAdvantage(holder string, ability abilities.Ability) (int, error) {
	e := &dndevents.SavingThrowChainEvent{SaverID: holder, Ability: ability, DC: 10}
	ctx := context.Background()
	c, err := dndevents.SavingThrowChain.On(w.bus).PublishWithChain(ctx, e, events.NewStagedChain[*dndevents.SavingThrowChainEvent](combat.ModifierStages))
	if err != nil {
		return 0, err
	}
	out, err := c.Execute(ctx, e)
	if err != nil {
		return 0, err
	}
	return len(out.AdvantageSources), nil
}
func (w *definitionWorld) checkAdvantage(holder string, skill skills.Skill) (int, error) {
	e := &dndevents.AbilityCheckChainEvent{CheckerID: holder, Skill: skill, DC: 10}
	ctx := context.Background()
	c, err := dndevents.AbilityCheckChain.On(w.bus).PublishWithChain(ctx, e, events.NewStagedChain[*dndevents.AbilityCheckChainEvent](combat.ModifierStages))
	if err != nil {
		return 0, err
	}
	out, err := c.Execute(ctx, e)
	if err != nil {
		return 0, err
	}
	return len(out.AdvantageSources), nil
}
