// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// heldBonusFixture is the Keep's kitchen in author's terms: a feast that
// raises the maximum by 2, a hearty stew that raises it by 3, and helping the
// cook that adds 1 to initiative. Each lasts until its holder's run ends.
const heldBonusFixture = `{
 "parts":{
  "feast-raise":{"kind":"hit_point_maximum","amount":{"fixed":2}},
  "stew-raise":{"kind":"hit_point_maximum","amount":{"fixed":3}},
  "quick":{"kind":"initiative_bonus","amount":{"fixed":1}},
  "run-end":{"kind":"end_on","params":{"beat":"run_end","reason":"run_ended"}}
 },
 "benefit_sets":{"feast":["feast-raise"],"stew":["stew-raise"],"quick":["quick"]},
 "definitions":{
  "feasted":{"name":"Feasted","coordinator":"recipient","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"feast","set":"feast"}]},
  "stewed":{"name":"Stewed","coordinator":"recipient","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"stew","set":"stew"}]},
  "well-fed":{"name":"Well fed","coordinator":"recipient","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"quick","set":"quick"}]},
  "fed-by-a":{"name":"Fed by A","coordinator":"user","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"feast","set":"feast"}]}
 }
}`

type heldBonusSuite struct{ suite.Suite }

func TestHeldBonusSuite(t *testing.T) { suite.Run(t, new(heldBonusSuite)) }

func (s *heldBonusSuite) setup() (*definitionWorld, *conditions.CompiledDefinitions) {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	defs, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(heldBonusFixture)})
	s.Require().NoError(err)
	return w, defs.Definitions
}

func (s *heldBonusSuite) give(w *definitionWorld, defs *conditions.CompiledDefinitions, id, user, target string) {
	prepared, _, err := w.prepare(defs, id, user, target, id+"-"+target)
	s.Require().NoError(err)
	s.Require().NoError(w.apply(prepared))
}

func (s *heldBonusSuite) heal(w *definitionWorld, who string, amount int) {
	s.Require().NoError(dndevents.HealingReceivedTopic.On(w.bus).Publish(context.Background(),
		dndevents.HealingReceivedEvent{TargetID: who, Amount: amount, Source: "fixture"}))
}

func (s *heldBonusSuite) damage(w *definitionWorld, who string, amount int) {
	w.holder(who).ApplyDamage(context.Background(), &combat.ApplyDamageInput{Instances: []combat.DamageInstance{{Amount: amount, Type: "slashing"}}})
}

func (s *heldBonusSuite) viewMaximum(w *definitionWorld, who string) int {
	view, err := w.holder(who).StatusView(&character.StatusViewInput{})
	s.Require().NoError(err)
	return view.View.HitPoints.Maximum
}

// heldSum sums one reader over a sheet's held conditions, as a sheet does.
func (s *heldBonusSuite) heldSum(w *definitionWorld, who string, read func(*conditions.HeldBonusInput) (*conditions.HeldBonusOutput, error)) int {
	total := 0
	for _, held := range w.holder(who).GetConditions() {
		out, err := read(&conditions.HeldBonusInput{Condition: held})
		s.Require().NoError(err)
		total += out.Amount
	}
	return total
}

// Catches a reader left on the stored field: the getter, the status view, the
// heal cap and the long rest must each reach the raised number.
func (s *heldBonusSuite) TestARaiseShowsInEveryReader() {
	w, defs := s.setup()
	s.give(w, defs, "feasted", "B", "B")
	b := w.holder("B")

	s.Equal(102, b.GetMaxHitPoints(), "getter")
	s.Equal(102, s.viewMaximum(w, "B"), "status view")
	s.Equal(100, b.GetHitPoints(), "gaining a raise does not heal")

	s.heal(w, "B", 50)
	s.Equal(102, b.GetHitPoints(), "healing to full reaches the raised maximum, and no further")

	s.damage(w, "B", 30)
	_, err := b.LongRest(context.Background())
	s.Require().NoError(err)
	s.Equal(102, b.GetHitPoints(), "a long rest restores to the raised maximum")

	s.Equal(100, w.holder("A").GetMaxHitPoints(), "a member without the status keeps their maximum")
}

// Catches a sum that keeps only the last raise, or one that counts a
// recipient-coordinated status's coordinator half as a second raise.
func (s *heldBonusSuite) TestTwoRaisesAddAndOneIsCountedOnce() {
	w, defs := s.setup()
	s.give(w, defs, "feasted", "B", "B")
	s.Equal(102, w.holder("B").GetMaxHitPoints(), "both halves of one status on one sheet raise once")
	s.give(w, defs, "stewed", "B", "B")
	s.Equal(105, w.holder("B").GetMaxHitPoints(), "two different raises add")
	s.Equal(105, s.viewMaximum(w, "B"))

	// A status one member gives another: the giver holds the coordinator half,
	// which raises nothing; the receiver holds the raise.
	s.give(w, defs, "fed-by-a", "A", "C")
	s.Equal(100, w.holder("A").GetMaxHitPoints(), "the giver's half raises nothing")
	s.Equal(102, w.holder("C").GetMaxHitPoints())
}

// Catches a removal that leaves current hit points above the new maximum, one
// that lowers the stored base, or one that forgets to mark the sheet changed.
func (s *heldBonusSuite) TestEndingARaiseCapsCurrentAndKeepsTheBase() {
	w, defs := s.setup()
	s.give(w, defs, "feasted", "B", "B")
	s.heal(w, "B", 2)
	s.Require().Equal(102, w.holder("B").GetHitPoints())
	s.Require().NoError(w.runEnd("B"))

	b := w.holder("B")
	s.Equal(100, b.GetMaxHitPoints())
	s.Equal(100, b.GetHitPoints(), "current is capped at the new maximum")
	s.True(b.IsDirty())
	data, err := b.ToData()
	s.Require().NoError(err)
	s.Equal(100, data.MaxHitPoints, "the base is unchanged")
	s.Equal(100, data.HitPoints)

	// Below the new maximum, ending a raise leaves current alone.
	s.give(w, defs, "feasted", "C", "C")
	s.damage(w, "C", 10)
	s.Require().NoError(w.runEnd("C"))
	s.Equal(90, w.holder("C").GetHitPoints())
}

// Catches a save that writes the effective maximum as the base (a reload would
// then count the raise twice) or a load that drops the raise.
func (s *heldBonusSuite) TestSaveAndReloadKeepTheBaseAndTheRaise() {
	w, defs := s.setup()
	s.give(w, defs, "feasted", "B", "B")
	s.heal(w, "B", 2)
	data, err := w.holder("B").ToData()
	s.Require().NoError(err)
	s.Equal(100, data.MaxHitPoints, "the stored number is the base")
	s.Equal(102, data.HitPoints)

	s.Require().NoError(w.reload())
	b := w.holder("B")
	s.Equal(102, b.GetMaxHitPoints(), "the raise is read again from the held status")
	s.Equal(102, b.GetHitPoints(), "current above the base survives the load")
	again, err := b.ToData()
	s.Require().NoError(err)
	s.Equal(100, again.MaxHitPoints, "still the base after a round trip")
}

// Catches an initiative reader that answers for a non-holder, or one that
// reads the hit point raise as initiative (or the reverse).
func (s *heldBonusSuite) TestInitiativeBonusIsReadForItsHolderOnly() {
	w, defs := s.setup()
	s.give(w, defs, "well-fed", "B", "B")
	s.give(w, defs, "feasted", "C", "C")

	s.Equal(1, s.heldSum(w, "B", conditions.InitiativeBonus))
	s.Zero(s.heldSum(w, "A", conditions.InitiativeBonus), "nothing held")
	s.Zero(s.heldSum(w, "C", conditions.InitiativeBonus), "a raise is not initiative")
	s.Zero(s.heldSum(w, "B", conditions.HitPointMaximumBonus), "initiative is not a raise")
	s.Equal(100, w.holder("B").GetMaxHitPoints())
}

// Catches a reader that accepts nothing to read.
func (s *heldBonusSuite) TestReadersRefuseMissingInput() {
	for _, read := range []func(*conditions.HeldBonusInput) (*conditions.HeldBonusOutput, error){conditions.HitPointMaximumBonus, conditions.InitiativeBonus} {
		_, err := read(nil)
		s.Error(err)
		_, err = read(&conditions.HeldBonusInput{})
		s.Error(err)
	}
}

// Catches a kind that takes the advantage special case or claims to apply to
// an action: each row describes itself in plain words and does not apply to
// an attack.
func (s *heldBonusSuite) TestEachKindDescribesItselfInItsRow() {
	w, defs := s.setup()
	s.give(w, defs, "feasted", "B", "B")
	s.give(w, defs, "well-fed", "B", "B")
	frame, err := w.frame("B", "C")
	s.Require().NoError(err)
	info, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{Conditions: w.holder("B").GetConditions(), Frame: frame})
	s.Require().NoError(err)
	rows := map[string]contributions.Effect{}
	for _, row := range info.Effects {
		rows[row.Description] = row
	}
	for _, want := range []string{"Raises the holder's hit point maximum by 2.", "Adds 1 to the holder's initiative."} {
		row, ok := rows[want]
		s.Require().True(ok, "a row reads %q", want)
		s.Equal(contributions.StateDoesNotApply, row.State, "it is not an attack's benefit")
		s.Empty(row.Benefit)
	}
	for _, detail := range frame.HeldEffects {
		if detail.MemberID == "B" {
			s.Empty(detail.Outputs, "neither bears against its holder")
		}
	}
}

// Catches a kind that accepts a zero, negative, fractional or missing amount,
// or a guard, on either kind.
func (s *heldBonusSuite) TestRefusals() {
	for _, kind := range []string{"hit_point_maximum", "initiative_bonus"} {
		for name, tc := range map[string]struct{ part, want string }{
			"zero":     {`{"kind":"` + kind + `","amount":{"fixed":0}}`, "must be a positive whole number, got 0"},
			"negative": {`{"kind":"` + kind + `","amount":{"fixed":-2}}`, "must be a positive whole number, got -2"},
			"fraction": {`{"kind":"` + kind + `","amount":{"fixed":1.5}}`, ""},
			"missing":  {`{"kind":"` + kind + `"}`, "requires an amount"},
			"dice":     {`{"kind":"` + kind + `","amount":{"dice":"1d4"}}`, "requires a fixed whole number"},
			"guarded":  {`{"kind":"` + kind + `","amount":{"fixed":1},"when":{"fact":{"name":"action.melee","op":"eq","value":true}}}`, "does not take a guard"},
			"params":   {`{"kind":"` + kind + `","params":{"on":"attack"},"amount":{"fixed":1}}`, "takes no params"},
		} {
			s.Run(kind+"/"+name, func() {
				raw := strings.Replace(heldBonusFixture, `{"kind":"hit_point_maximum","amount":{"fixed":2}}`, tc.part, 1)
				s.Require().NotEqual(heldBonusFixture, raw)
				_, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(raw)})
				s.Require().Error(err)
				if tc.want != "" {
					s.Contains(err.Error(), tc.want)
				}
			})
		}
	}
}
