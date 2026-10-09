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
// member's Afford carries it. It runs over a readable actor and over the two
// branches where compileOffersFor emits blockers for an execution verb to
// select against: an unreadable sheet and a downed actor. The structural half
// pins that attachInformation has one caller, Afford.
func TestExecutionPathsNeverAttachInformation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(characters *strikeCharacters)
		blocked bool
	}{
		{name: "readable", prepare: func(*strikeCharacters) {}},
		{name: "unreadable", blocked: true, prepare: func(c *strikeCharacters) { delete(c.byID, "alice") }},
		{name: "downed", blocked: true, prepare: func(c *strikeCharacters) { c.byID["alice"].HitPoints = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			tc.prepare(characters)

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
			sawBlockedSessionVerb := false
			for _, offer := range offers {
				require.Nil(t, offer.declaration.Information,
					"%s compiled for execution carries information", offer.declaration.Verb)
				if !offer.declaration.Available && offer.declaration.ID == "" &&
					sessionVerbProse[offer.declaration.Verb] != "" {
					sawBlockedSessionVerb = true
				}
			}
			if tc.blocked {
				require.True(t, sawBlockedSessionVerb, "precondition: this scene emits a blocked session verb")
			}
		})
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
			{Label: "Base damage", Value: "1d8 + 2 + DEX modifier (+3) · Piercing"},
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
		require.ErrorIs(t, err, ErrBadInformation, "an outcome with no word is refused, not guessed")
		_, err = renderFacts(combatActions.BaseFacts{Cast: &combatActions.CastFacts{
			Effects: []combatActions.EffectFact{{Ref: core.Ref{Module: "dnd5e", Type: "conditions", ID: "nowhere"}}}}})
		require.ErrorIs(t, err, ErrUnknownContent, "a condition with no catalogue entry is the catalogue's gap")
	})
}

// TestDescribedInformationNeverSynthesisesProse: a definition whose owner
// wrote no description gets none — not its name, not its ref — while its
// facts still render. Every compiled definition in today's catalogue carries
// prose, so this is pinned on the renderer's own seam.
func TestDescribedInformationNeverSynthesisesProse(t *testing.T) {
	definition := clubDefinition()
	info, err := describedInformation(&definition)
	require.NoError(t, err)
	require.Empty(t, info.Description)
	require.Equal(t, []ActionInformationDetail{
		{Label: "Base damage", Value: "1d4 · Bludgeoning"},
		{Label: "Reach", Value: "5 ft"},
	}, info.Details)
}

func clubDefinition() combatActions.Definition {
	return combatActions.Definition{
		Ref: *refs.Weapons.Club(), Name: "Club",
		Attack: &combatActions.AttackProfile{
			Category: combatActions.AttackCategoryWeapon,
			Delivery: combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			Damage:   []damage.Damage{{Dice: "1d4", Type: damage.Bludgeoning}},
		},
	}
}

// TestAttachInformationFailsClosed: Afford refuses rather than ship a card
// with a missing or guessed row. A definition the rulebook will not describe
// is ErrBadInformation; an applied condition the display catalogue does not
// hold is ErrUnknownContent. A blocker that compiled nothing stays nil.
func TestAttachInformationFailsClosed(t *testing.T) {
	invalid := clubDefinition()
	invalid.Name = ""
	err := attachInformation(&attachInformationInput{Offers: []compiledOffer{
		{declaration: Declaration{Verb: VerbAttack}, attack: &invalid},
	}})
	require.ErrorIs(t, err, ErrBadInformation)

	uncatalogued := core.Ref{Module: "dnd5e", Type: "conditions", ID: "nowhere"}
	spell := combatActions.Definition{
		Ref: *refs.Spells.Bane(), Name: "Bane",
		Cast: &combatActions.CastProfile{
			RangeFeet: 30, Target: combatActions.CastTargetOneCreature, MinTargets: 1, MaxTargets: 1,
			Effects: []combatActions.CastEffect{{Recipient: combatActions.CastRecipientTarget, Ref: uncatalogued}},
		},
	}
	err = attachInformation(&attachInformationInput{Offers: []compiledOffer{
		{declaration: Declaration{Verb: VerbCast}, spell: &spell},
	}})
	require.ErrorIs(t, err, ErrUnknownContent)

	blocked := []compiledOffer{{declaration: Declaration{Verb: VerbAttack}}, {declaration: Declaration{Verb: VerbCast}}}
	require.NoError(t, attachInformation(&attachInformationInput{Offers: blocked}))
	require.Nil(t, blocked[0].declaration.Information)
	require.Nil(t, blocked[1].declaration.Information)
}

// TestHealingModifiersRenderSigned: every healing modifier renders as its
// name and signed amount, the way a damage row renders its ability clause; a
// negative and a zero are stated, never hidden or written "+ -1".
func TestHealingModifiersRenderSigned(t *testing.T) {
	require.Equal(t, "1d8 + Wisdom (+3) + Disciple of Life (+2)", renderHealing(&combatActions.HealingFact{
		Dice: "1d8", Modifiers: []combatActions.ModifierFact{{Name: "Wisdom", Amount: 3}, {Name: "Disciple of Life", Amount: 2}},
	}))
	require.Equal(t, "1d8 + Charisma (-1)", renderHealing(&combatActions.HealingFact{
		Dice: "1d8", Modifiers: []combatActions.ModifierFact{{Name: "Charisma", Amount: -1}},
	}))
	require.Equal(t, "1d4 + Wisdom (+0)", renderHealing(&combatActions.HealingFact{
		Dice: "1d4", Modifiers: []combatActions.ModifierFact{{Name: "Wisdom", Amount: 0}},
	}))
	require.Equal(t, "1d6 - 1 · Slashing", renderDamage(combatActions.DamageFact{Dice: "1d6", FlatBonus: -1, Type: damage.Slashing}))
}

type displayed struct{ name, detail string }

func conditionsDisplayForTest(t *testing.T) (displayed, bool) {
	t.Helper()
	d, ok := conditions.DisplayFor(*refs.Conditions.Baned())
	require.True(t, ok)
	require.NotEmpty(t, d.Detail)
	return displayed{name: d.Name, detail: d.Detail}, ok
}

// TestRenderSaveComparesSavesConstants holds the saves ruling in the one
// place session compares save words: every case in renderSave names a saves
// constant, so a string literal written back in place of saves.Half or
// saves.RecurrenceEndOfTurn fails here. The empty string is the one literal
// allowed: it is the unset recurrence a gate's own encoding reads as none.
func TestRenderSaveComparesSavesConstants(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "information.go", nil, 0)
	require.NoError(t, err)
	cases := 0
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "renderSave" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				cases++
				if lit, ok := expr.(*ast.BasicLit); ok {
					require.Equal(t, `""`, lit.Value, "%s: a literal stands in for a saves constant", fset.Position(lit.Pos()))
					continue
				}
				selector, ok := expr.(*ast.SelectorExpr)
				require.True(t, ok, "%s: a case must name a saves constant", fset.Position(expr.Pos()))
				pkg, ok := selector.X.(*ast.Ident)
				require.True(t, ok && pkg.Name == "saves", "%s: a case must name a saves constant", fset.Position(expr.Pos()))
			}
			return true
		})
	}
	require.GreaterOrEqual(t, cases, 4, "precondition: renderSave's switches were found")
}
