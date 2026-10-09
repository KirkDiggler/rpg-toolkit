// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// TestExecutionPathsNeverAttachInformation: the offers every execution verb
// selects from — compileOffersFor, called exactly as attack, cast, activate,
// move and death save call it — carry nil information, while the same
// member's Afford carries it. Information can therefore describe an action
// but nothing an execution path decides can read it. The structural half pins
// that attachInformation has one caller, Afford.
func TestExecutionPathsNeverAttachInformation(t *testing.T) {
	ctx := context.Background()
	sessions := &strikeSessions{byID: map[string]*SessionData{}}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	characters := &strikeCharacters{byID: map[string]*character.Data{
		"alice": strikeFixtureFighter("alice"),
		"bob":   strikeFixtureFighter("bob"),
	}}
	mgr, err := NewManager(&Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: &scriptedDice{}, TurnDriver: Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: DiscardEvents{},
	})
	require.NoError(t, err)
	launchScene(t, mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 4, 4)}},
		Party: []sceneSeat{seatAt("alice", 1, 1), seatAt("bob", 2, 1)},
	})
	authorTurnClock(t, encounters.byID["sess"], []string{"alice", "bob"}, 0)

	afford, err := mgr.Afford(ctx, &AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	described := 0
	for _, declaration := range afford.Declarations {
		if declaration.Information != nil {
			described++
		}
	}
	require.NotZero(t, described, "precondition: Afford describes this member's panel")

	data, err := mgr.loadSessionData(ctx, "sess")
	require.NoError(t, err)
	enc, err := mgr.loadWorld(ctx, data)
	require.NoError(t, err)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: "alice"})
	require.NoError(t, err)
	offers, err := mgr.compileOffersFor(ctx, enc, data, "sess", "alice", clock, mgr.loadActorSheet(ctx, "alice"),
		VerbAttack, VerbMove, VerbActivate, VerbCast, VerbIntimidate, VerbPersuade, VerbDeathSave, VerbEndTurn,
	)
	require.NoError(t, err)
	require.NotEmpty(t, offers)
	for _, offer := range offers {
		require.Nil(t, offer.declaration.Information, "%s compiled for execution carries information", offer.declaration.Verb)
	}

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	callers := map[string]int{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		require.NoError(t, err)
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "attachInformation" {
						callers[fn.Name.Name]++
					}
				}
				return true
			})
		}
	}
	require.Equal(t, map[string]int{"Afford": 1}, callers)
}

// TestSaveWordsMatchTheSavesPackage: the renderer matches save outcomes and
// recurrences by value because session never imports saves outside tests.
// This is where a renamed constant is caught.
func TestSaveWordsMatchTheSavesPackage(t *testing.T) {
	require.Equal(t, string(saves.Negated), saveOutcomeNegated)
	require.Equal(t, string(saves.Half), saveOutcomeHalf)
	require.Equal(t, string(saves.RecurrenceNone), saveRecurrenceNone)
	require.Equal(t, string(saves.RecurrenceEndOfTurn), saveRecurrenceTurnEnd)
}

// TestRenderFactsBranchesTheAffordScenesDoNotReach covers the rows no Afford
// scene in this package produces today: a ranged weapon, a two-handed grip,
// flat bonuses, a repeating save with an unknown DC, a self range, a
// fixed-count target, the other area words and a caster-side effect.
func TestRenderFactsBranchesTheAffordScenesDoNotReach(t *testing.T) {
	t.Run("ranged two-handed weapon", func(t *testing.T) {
		rows, err := renderFacts(combatActions.BaseFacts{
			Damage: []combatActions.DamageFact{{Dice: "1d8", FlatBonus: 2, Type: damage.Piercing,
				Ability: &combatActions.AbilityFact{Ability: abilities.DEX, Modifier: 3, Participates: true}}},
			Grip:   combatActions.GripTwoHanded,
			Ranged: &combatActions.RangedDelivery{NormalFeet: 150, LongFeet: 600},
		})
		require.NoError(t, err)
		require.Equal(t, []ActionInformationDetail{
			{Label: "Base damage", Value: "1d8 +2 + DEX modifier (+3) · Piercing"},
			{Label: "Grip", Value: "Two-handed"},
			{Label: "Range", Value: "150 ft (long 600 ft)"},
		}, rows)
	})
	t.Run("cast", func(t *testing.T) {
		rows, err := renderFacts(combatActions.BaseFacts{Cast: &combatActions.CastFacts{
			Targets:         combatActions.TargetsFact{Rule: combatActions.CastTargetSelf},
			DamageIfInjured: []combatActions.DamageFact{{Dice: "1d12", Type: damage.Necrotic}},
			Save: &combatActions.SaveFact{Abilities: []abilities.Ability{abilities.STR, abilities.DEX},
				OnSuccess: saves.Negated, Recurrence: saves.RecurrenceEndOfTurn},
			Effects: []combatActions.EffectFact{{Ref: *refs.Conditions.Baned(), Recipient: combatActions.CastRecipientCaster}},
			Area: &combatActions.CastArea{Catches: combatActions.AreaCatchesEveryone, Footprint: combatActions.Footprint{
				Shape: combatActions.AreaRadius, SizeFeet: 20, Origin: combatActions.AreaOriginPoint}},
		}})
		require.NoError(t, err)
		baned, _ := conditionsDisplayForTest(t)
		require.Equal(t, []ActionInformationDetail{
			{Label: "Damage if injured", Value: "1d12 · Necrotic"},
			{Label: "Save", Value: "STR or DEX save · success: negated · repeats at end of turn"},
			{Label: "On you", Value: baned.name},
			{Label: baned.name, Value: baned.detail},
			{Label: "Range", Value: "Self"},
			{Label: "Area", Value: "20 ft radius at a point · affects everyone"},
		}, rows)
	})
	t.Run("targets and cones", func(t *testing.T) {
		require.Equal(t, "2 creatures", renderTargets(combatActions.TargetsFact{Min: 2, Max: 2}))
		require.Equal(t, "1 creature", renderTargets(combatActions.TargetsFact{Min: 1, Max: 1}))
		cone, err := renderArea(&combatActions.CastArea{Catches: combatActions.AreaCatchesOthers,
			Footprint: combatActions.Footprint{Shape: combatActions.AreaTriangle, SizeFeet: 15, Origin: combatActions.AreaOriginCaster}})
		require.NoError(t, err)
		require.Equal(t, "15 ft cone from you · affects others", cone)
	})
	t.Run("fails closed", func(t *testing.T) {
		_, err := renderFacts(combatActions.BaseFacts{Cast: &combatActions.CastFacts{
			Save: &combatActions.SaveFact{OnSuccess: "quartered"}}})
		require.ErrorIs(t, err, ErrBadAttack, "an outcome with no word is refused, not guessed")
		_, err = renderFacts(combatActions.BaseFacts{Cast: &combatActions.CastFacts{
			Effects: []combatActions.EffectFact{{Ref: core.Ref{Module: "dnd5e", Type: "conditions", ID: "nowhere"}}}}})
		require.ErrorIs(t, err, ErrBadAttack, "a condition with no catalogue entry is refused")
	})
}

type displayed struct{ name, detail string }

func conditionsDisplayForTest(t *testing.T) (displayed, bool) {
	t.Helper()
	d, ok := conditions.DisplayFor(*refs.Conditions.Baned())
	require.True(t, ok)
	require.NotEmpty(t, d.Detail)
	return displayed{name: d.Name, detail: d.Detail}, ok
}
