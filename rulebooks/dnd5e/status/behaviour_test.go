// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package status_test

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/status"
)

// The holder is B; A is the other member. Every probe reads one roll or one
// fold on the world's bus, the way resolution asks it.

// frame is an attack frame by actor at target, built from what both sheets
// hold, as resolution builds one.
func (s *statusSuite) frame(w *world, actor, target string, melee bool) contributions.Frame {
	f := contributions.Frame{Actor: actor, Target: contributions.Known(target),
		ActorClassLevels: contributions.KnownClassLevels(contributions.ClassLevel{Class: classes.Fighter, Levels: 1}),
		Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack), Ability: contributions.Known(abilities.STR),
			Melee: contributions.Known(melee), WeaponPool: contributions.Known(true), Opportunity: contributions.Known(false)}}
	for _, holder := range w.sheets {
		held := contributions.MemberHeld{Member: holder.GetID(), Conditions: []contributions.HeldCondition{}}
		for _, condition := range holder.GetConditions() {
			address := conditions.ConditionAddressOf(holder.GetID(), condition)
			held.Conditions = append(held.Conditions, contributions.HeldCondition{Ref: address.ConditionRef, SourceID: address.SourceID})
		}
		f.Held = append(f.Held, held)
		projected, err := conditions.ProjectHeldEffects(&conditions.ProjectHeldEffectsInput{HolderID: holder.GetID(), Conditions: holder.GetConditions()})
		s.Require().NoError(err)
		f.HeldEffects = append(f.HeldEffects, projected.Details...)
	}
	return f
}

// attackAdvantage reports whether an attack by actor at target carries the
// status's advantage.
func (s *statusSuite) attackAdvantage(w *world, actor, target string, melee bool, name string) bool {
	ctx := context.Background()
	f := s.frame(w, actor, target, melee)
	e := dndevents.AttackChainEvent{AttackerID: actor, TargetID: target, Frame: f}
	c, err := dndevents.AttackChain.On(w.bus).PublishWithChain(ctx, e, events.NewStagedChain[dndevents.AttackChainEvent](combat.ModifierStages))
	s.Require().NoError(err)
	out, err := c.Execute(ctx, e)
	s.Require().NoError(err)
	for _, source := range out.AdvantageSources {
		if strings.Contains(source.Reason, name) {
			return true
		}
	}
	return false
}

// fixtureDamage is one weapon component of amount damage of kind.
func fixtureDamage(amount int, kind damage.Type) []dndevents.DamageComponent {
	ref := core.Ref{Module: "example", Type: "actions", ID: "hit"}
	return []dndevents.DamageComponent{{Source: dndevents.DamageSourceWeapon, DamageType: kind,
		Roll: dndevents.RollComponent{Source: dndevents.RollSource{Ref: &ref, Name: "Fixture hit"}, Modifier: &amount}}}
}

// dealt is the total of a 10-point slashing hit by actor at target, after
// every damage rule.
func (s *statusSuite) dealt(w *world, actor, target string) int {
	ctx := context.Background()
	in := dndevents.DamageChainInput{AttackerID: actor, TargetID: target, Frame: s.frame(w, actor, target, true),
		WeaponDamageType: damage.Slashing, Components: fixtureDamage(10, damage.Slashing)}
	c, err := dndevents.DamageChain.On(w.bus).PublishWithChain(ctx, dndevents.NewDamageChainEvent(in), events.NewStagedChain[*dndevents.DamageChainEvent](combat.ModifierStages))
	s.Require().NoError(err)
	out, err := c.Execute(ctx, dndevents.NewDamageChainEvent(in))
	s.Require().NoError(err)
	total := 0
	for _, component := range out.Components {
		if component.Roll.Modifier != nil {
			total += *component.Roll.Modifier
		}
	}
	return total
}

// taken is what the holder takes of a 10-point hit of kind, after every
// reduction and multiplier.
func (s *statusSuite) taken(w *world, kind damage.Type) int {
	ctx := context.Background()
	event, err := dndevents.NewIncomingDamageEvent(dndevents.IncomingDamageInput{TargetID: "B", SourceID: "A",
		Frame: s.frame(w, "A", "B", true), Dealt: fixtureDamage(10, kind)})
	s.Require().NoError(err)
	c, err := dndevents.IncomingDamageChain.On(w.bus).PublishWithChain(ctx, event.Clone(), events.NewStagedChain[*dndevents.IncomingDamageEvent](combat.ModifierStages))
	s.Require().NoError(err)
	folded, err := c.Execute(ctx, event.Clone())
	s.Require().NoError(err)
	settled, err := combat.SettleDamage(&combat.SettleDamageInput{Dealt: event.Dealt(), Reductions: folded.Reductions, Multipliers: folded.Multipliers})
	s.Require().NoError(err)
	n := 0
	for _, part := range settled.Types {
		n += part.Taken
	}
	return n
}

// beats publish each ends_on value's beat for a member.
var beats = map[string]func(bus events.EventBus, who string) error{
	"run": func(bus events.EventBus, who string) error {
		return dndevents.RunEndTopic.On(bus).Publish(context.Background(), dndevents.RunEndEvent{SubjectID: who})
	},
	"rest": func(bus events.EventBus, who string) error {
		return dndevents.RestTopic.On(bus).Publish(context.Background(), dndevents.RestEvent{CharacterID: who, RestType: coreResources.ResetLongRest})
	},
	"fight starts": func(bus events.EventBus, who string) error {
		return dndevents.CombatStartTopic.On(bus).Publish(context.Background(), dndevents.CombatStartEvent{SubjectID: who})
	},
	"fight ends": func(bus events.EventBus, who string) error {
		return dndevents.CombatEndTopic.On(bus).Publish(context.Background(), dndevents.CombatEndEvent{SubjectID: who})
	},
	"next turn": func(bus events.EventBus, who string) error {
		return dndevents.TurnStartTopic.On(bus).Publish(context.Background(), dndevents.TurnStartEvent{SubjectID: who})
	},
	"dropped to zero": func(bus events.EventBus, who string) error {
		return dndevents.DamageTakenTopic.On(bus).Publish(context.Background(), &dndevents.DamageTakenEvent{MemberID: who, Amount: 5, DroppedToZero: true})
	},
}

// EACH WORD DOES WHAT IT SAYS, AND NOT ITS NEIGHBOUR'S: one row per word, the
// status given with that word alone, the behaviour it names shown, and the
// neighbouring behaviour a swapped mapping would produce shown absent.
func (s *statusSuite) TestEachWordDoesWhatItSaysAndNotItsNeighbours() {
	const holder, other = "B", "A"
	for _, tc := range []struct {
		name   string
		change string
		check  func(w *world)
	}{
		// Catches `attacks` mapped to against_holder (or to a check/save).
		{"advantage on attacks", `{"advantage":"attacks"}`, func(w *world) {
			s.True(s.attackAdvantage(w, holder, other, true, "Probe"), "the holder's own attack")
			s.False(s.attackAdvantage(w, other, holder, true, "Probe"), "an attack against the holder")
		}},
		// Catches `attacks against you` mapped to by_holder.
		{"attacks against you have advantage", `{"advantage":"attacks against you"}`, func(w *world) {
			s.True(s.attackAdvantage(w, other, holder, true, "Probe"), "an attack against the holder")
			s.False(s.attackAdvantage(w, holder, other, true, "Probe"), "the holder's own attack")
		}},
		// Catches `attack: melee` mapped to another fact, or to false.
		{"only for melee attacks", `{"advantage":"attacks","when":{"attack":"melee"}}`, func(w *world) {
			s.True(s.attackAdvantage(w, holder, other, true, "Probe"), "a melee attack")
			s.False(s.attackAdvantage(w, holder, other, false, "Probe"), "a ranged attack")
		}},
		// Catches a resistance list mapped to the wrong types.
		{"resistance to fire", `{"resistance":["fire"]}`, func(w *world) {
			s.Equal(5, s.taken(w, damage.Fire), "fire is halved")
			s.Equal(10, s.taken(w, damage.Cold), "cold is not")
		}},
		// Catches `damage` with its sign lost or applied to the wrong side.
		{"two more damage", `{"damage":2}`, func(w *world) {
			s.Equal(12, s.dealt(w, holder, other), "the holder's hit")
			s.Equal(10, s.dealt(w, other, holder), "a hit on the holder")
		}},
		{"one less damage", `{"damage":-1}`, func(w *world) {
			s.Equal(9, s.dealt(w, holder, other), "the holder's hit")
		}},
		// Catches max_hit_points mapped to initiative (or to the wrong member).
		{"two more maximum hit points", `{"max_hit_points":2}`, func(w *world) {
			s.Equal(22, w.sheets[1].GetMaxHitPoints(), "the holder's maximum")
			s.Equal(20, w.sheets[0].GetMaxHitPoints(), "another member's maximum")
			s.Zero(s.initiative(w, holder), "it is not initiative")
		}},
		// Catches cannot mapped to nothing, or read for the wrong member.
		{"cannot move", `{"cannot":["move"]}`, func(w *world) {
			s.Equal([]string{"Probe"}, s.cannotMove(w, 1).Sources, "the holder cannot move")
			s.False(s.cannotMove(w, 0).CannotMove, "another member may move")
			s.Zero(s.initiative(w, holder), "it is not initiative")
			s.Require().NoError(beats["rest"](w.bus, holder))
			s.False(s.cannotMove(w, 1).CannotMove, "ending it gives moving back")
		}},
		// Catches initiative mapped to the hit point maximum (or to the wrong member).
		{"one more to initiative", `{"initiative":1}`, func(w *world) {
			s.Equal(1, s.initiative(w, holder), "the holder's initiative")
			s.Zero(s.initiative(w, other), "another member's initiative")
			s.Equal(20, w.sheets[1].GetMaxHitPoints(), "it is not the maximum")
		}},
	} {
		s.Run(tc.name, func() {
			w := s.world()
			s.give(w, s.compile(tc.change, `{"ends_on":"rest"}`))
			tc.check(w)
		})
	}

	// Catches any two ends_on values swapped: each ends on its own beat, for
	// its holder, and on none of the others, nor on its own beat for
	// another member.
	for word := range beats {
		s.Run("ends on "+word, func() {
			w := s.world()
			s.give(w, s.compile(`{"advantage":"saves"}`, `{"ends_on":"`+word+`"}`))
			s.Require().NoError(beats[word](w.bus, other))
			s.NotEmpty(w.sheets[1].GetConditions(), "%s for another member", word)
			for neighbour, publish := range beats {
				if neighbour == word {
					continue
				}
				s.Require().NoError(publish(w.bus, holder))
				s.NotEmpty(w.sheets[1].GetConditions(), "%q does not end on %q", word, neighbour)
			}
			s.Require().NoError(beats[word](w.bus, holder))
			s.Empty(w.sheets[1].GetConditions(), "%q ends on its own beat", word)
		})
	}
}

// initiative is what a member's held conditions add to their initiative.
func (s *statusSuite) initiative(w *world, who string) int {
	total := 0
	for _, c := range w.sheets {
		if c.GetID() != who {
			continue
		}
		for _, held := range c.GetConditions() {
			out, err := conditions.InitiativeBonus(&conditions.HeldBonusInput{Condition: held})
			s.Require().NoError(err)
			total += out.Amount
		}
	}
	return total
}

// cannotMove asks the sheet at index i whether it may move by its own will.
func (s *statusSuite) cannotMove(w *world, i int) *conditions.CannotMoveOutput {
	out, err := w.sheets[i].CannotMove()
	s.Require().NoError(err)
	return out
}

// THE KITCHEN EXAMPLE: both changes in force on the holder, and exactly one
// heal of 2 returned for the caller to run once. Catches when_given dropped,
// doubled, put in the library as a part, or carrying the wrong amount.
func (s *statusSuite) TestTheFeastGivesItsChangesAndOneHeal() {
	const feast = `{"name":"Feasted","changes":[{"max_hit_points":2},{"initiative":1}],` +
		`"when_given":[{"heal":2}],"maintenance":[{"ends_on":"run"}],"again":"ignored"}`
	compiled, err := status.Compile(&status.CompileInput{ID: "feasted", Status: json.RawMessage(feast)})
	s.Require().NoError(err)
	s.Equal([]status.WhenGivenAct{{Kind: status.WhenGivenHeal, Amount: 2}}, compiled.WhenGiven)
	s.NotContains(string(compiled.Library), "heal", "a when_given act is not an effect part")

	w := s.world()
	s.give(w, compiled)
	s.Equal(22, w.sheets[1].GetMaxHitPoints())
	s.Equal(20, w.sheets[1].GetHitPoints(), "giving alone does not heal; the caller runs the act")
	s.Equal(1, s.initiative(w, "B"))

	for name, doc := range map[string]string{
		"missing": strings.Replace(feast, `"when_given":[{"heal":2}],`, ``, 1),
		"empty":   strings.Replace(feast, `[{"heal":2}]`, `[]`, 1),
	} {
		out, err := status.Compile(&status.CompileInput{ID: "feasted", Status: json.RawMessage(doc)})
		s.Require().NoError(err, name)
		s.Empty(out.WhenGiven, "%s when_given returns no acts", name)
	}
}

// compile is a status named Probe with one change and one rule.
func (s *statusSuite) compile(change, rule string) *status.CompileOutput {
	doc := `{"name":"Probe","changes":[` + change + `],"maintenance":[` + rule + `],"again":"ignored"}`
	out, err := status.Compile(&status.CompileInput{ID: "probe", Status: json.RawMessage(doc)})
	s.Require().NoError(err, doc)
	return out
}
