// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Action information through the real Afford (provider-design R10–R14).
//
// Every fact row asserted here is this seam's render contract, so the literal
// strings are the point. Every PROSE string is its owner's, so these tests
// read it from the owner — the spell catalogue, the condition display
// catalogue, the posed offer — rather than repeating it, and a wording change
// in content never fails a test here.

// detail is one expected row.
func detail(label, value string) session.ActionInformationDetail {
	return session.ActionInformationDetail{Label: label, Value: value}
}

// castScene is a CastSuite driven from a plain test, so each Done-when case
// stands under its own name.
func castScene(t *testing.T, bard *character.Data, cells int, rolls ...int) *CastSuite {
	t.Helper()
	s := &CastSuite{}
	s.SetT(t)
	s.scene(bard, cells, rolls...)
	return s
}

// affordRows is one member's whole panel.
func affordRows(t *testing.T, mgr *session.Manager, member string) []session.Declaration {
	t.Helper()
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	require.NoError(t, err)
	return out.Declarations
}

func rowOfVerb(t *testing.T, rows []session.Declaration, verb session.Verb) session.Declaration {
	t.Helper()
	for _, row := range rows {
		if row.Verb == verb {
			return row
		}
	}
	t.Fatalf("no %s row in %d declarations", verb, len(rows))
	return session.Declaration{}
}

func spellDescription(t *testing.T, spell spells.Spell) string {
	t.Helper()
	data := spells.GetData(spell)
	require.NotNil(t, data, "the catalogue carries %s", spell)
	require.NotEmpty(t, data.Description, "precondition: the catalogue describes %s", spell)
	return data.Description
}

// warhammerBarbarian is EffectRowsSuite's raging barbarian with a warhammer
// in hand, so one Attack row carries both base facts and an effect row.
func warhammerBarbarian(t *testing.T) *character.Data {
	t.Helper()
	rage, err := (&conditions.RagingCondition{
		CharacterID: "alice", Source: refs.Features.Rage().String(),
	}).ToJSON()
	require.NoError(t, err)
	return &character.Data{
		ID: "alice", PlayerID: "player-alice", Name: "alice", Level: 1,
		Levels: syntheticLevels(classes.Barbarian, 1), ClassID: classes.Barbarian, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 8, abilities.WIS: 10, abilities.CHA: 8,
		},
		HitPoints: 14, MaxHitPoints: 14, ProficiencyBonus: 2,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Warhammer), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: string(weapons.Warhammer)},
		Conditions:     []json.RawMessage{rage},
	}
}

// assembledAttackDescription is the prose the rulebook assembles onto a
// sheet's main-hand swing — the owner's words, read from the owner.
func assembledAttackDescription(t *testing.T, data *character.Data) string {
	t.Helper()
	sheet, err := character.Load(context.Background(), data)
	require.NoError(t, err)
	definition, err := character.AssembleAttack(sheet, &character.AssembleAttackInput{Slot: character.SlotMainHand})
	require.NoError(t, err)
	return definition.Description
}

// warhammerEffectRowsBeforeInformation is the Effects slice of this row as
// the session produced it BEFORE information existed (captured at abcb38f3,
// the commit before attachInformation). Information sits beside effect rows
// and must not move one byte of them.
const warhammerEffectRowsBeforeInformation = `[{"id":"dnd5e:conditions:raging","ref":"dnd5e:conditions:raging","name":"Raging","description":"Adds your rage damage bonus to melee weapon attacks that use Strength. Also grants advantage on Strength-based skill checks and Strength saving throws, and resistance to bludgeoning, piercing, and slashing damage.","state":"applies","reason":"The melee weapon attack uses Strength","participation":"contributes_now","benefit":"+2 damage"}]`

// TestAffordWarhammerFactsAboveUnchangedEffects: a compiled swing renders its
// base facts — the participating modifier joins the damage — while its effect
// rows stay exactly what they were. The off-hand half pins the other side of
// damage.IncludesAbilityModifier: a stated, positive, non-participating
// modifier renders nothing.
func TestAffordWarhammerFactsAboveUnchangedEffects(t *testing.T) {
	t.Run("main hand", func(t *testing.T) {
		s := &EffectRowsSuite{}
		s.SetT(t)
		s.SetupTest()
		s.cave(warhammerBarbarian(t))
		attack := s.mainAttack(s.afford("alice"))

		require.NotNil(t, attack.Information)
		require.Equal(t, assembledAttackDescription(t, warhammerBarbarian(t)), attack.Information.Description,
			"the swing's prose is the assembled definition's own")
		require.Equal(t, []session.ActionInformationDetail{
			detail("Base damage", "1d8 + STR modifier (+3) · Bludgeoning"),
			detail("Grip", "One-handed"),
			detail("Reach", "5 ft"),
		}, attack.Information.Details)

		effects, err := json.Marshal(attack.Effects)
		require.NoError(t, err)
		require.JSONEq(t, warhammerEffectRowsBeforeInformation, string(effects))
	})

	t.Run("off hand", func(t *testing.T) {
		mgr, _, _, _ := aFight(t, offHandFighter("alice"), []int{1})
		main := attackDeclarationForSlot(t, affordOffHandFight(t, mgr).Declarations, session.SlotAction)
		require.NotNil(t, main.Information)
		require.Equal(t, []session.ActionInformationDetail{
			detail("Base damage", "1d6 + STR modifier (+3) · Piercing"),
			detail("Grip", "One-handed"),
			detail("Reach", "5 ft"),
		}, main.Information.Details)

		_, err := mgr.Attack(context.Background(), &session.AttackInput{
			Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: main.ID,
		})
		require.NoError(t, err)
		bonus := attackDeclarationForSlot(t, affordOffHandFight(t, mgr).Declarations, session.SlotBonus)
		require.NotNil(t, bonus.Information)
		require.Equal(t, []session.ActionInformationDetail{
			detail("Base damage", "1d6 · Slashing"),
			detail("Grip", "Off-hand"),
			detail("Reach", "5 ft"),
		}, bonus.Information.Details,
			"an off-hand swing without the style adds no positive modifier, so none is rendered")
	})
}

// TestAffordBaneDescribedWithZeroEffects: a cast that makes no spell attack
// carries no effect rows, and is still fully described.
func TestAffordBaneDescribedWithZeroEffects(t *testing.T) {
	s := castScene(t, castingBardWithSpells("bard", spells.Bane), 2)
	row := s.castRow(spells.Bane)

	require.Empty(t, row.Effects, "precondition: Bane makes no spell attack, so it carries no effect rows")
	require.NotNil(t, row.Information)
	require.Equal(t, spellDescription(t, spells.Bane), row.Information.Description)

	baned, known := conditions.DisplayFor(*refs.Conditions.Baned())
	require.True(t, known)
	require.NotEmpty(t, baned.Detail, "precondition: the catalogue explains Baned")
	require.Equal(t, []session.ActionInformationDetail{
		detail("Save", "CHA save · DC 13 · success: negated"),
		detail("On a failed save", baned.Name),
		detail(baned.Name, baned.Detail),
		detail("Range", "30 ft"),
		detail("Targets", "1 to 3 creatures"),
		detail("Concentration", "Required"),
	}, row.Information.Details)
	require.Equal(t, "Baned", baned.Name)
}

// TestAffordThunderwaveFacts: an area cast whose save halves the damage.
func TestAffordThunderwaveFacts(t *testing.T) {
	s := castScene(t, castingBardWithSpells("bard", spells.Thunderwave), 2)
	row := s.castRow(spells.Thunderwave)

	require.NotNil(t, row.Information)
	require.Equal(t, spellDescription(t, spells.Thunderwave), row.Information.Description)
	require.Contains(t, row.Information.Details, detail("Base damage", "2d8 · Thunder"))
	require.Contains(t, row.Information.Details, detail("Save", "CON save · DC 13 · success: half damage"))
	require.Contains(t, row.Information.Details, detail("Area", "15 ft cube from your edge · affects others"))
}

// TestAffordCureWoundsFacts: healing with its sourced modifier, a touch
// range, and no save.
func TestAffordCureWoundsFacts(t *testing.T) {
	s := castScene(t, castingBardWithSpells("bard", spells.CureWounds), 1)
	row := s.castRow(spells.CureWounds)

	require.NotNil(t, row.Information)
	require.Equal(t, spellDescription(t, spells.CureWounds), row.Information.Description)
	require.Contains(t, row.Information.Details, detail("Healing", "1d8 + 3 (Charisma)"))
	require.Contains(t, row.Information.Details, detail("Range", "Touch"))
	for _, row := range row.Information.Details {
		require.NotEqual(t, "Save", row.Label, "Cure Wounds asks no save")
	}
}

// TestAffordSpentDodgeStillDescribed: availability and information are
// independent — a spent Dodge says it cannot be taken and still says what it
// is.
func TestAffordSpentDodgeStillDescribed(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, _ := aFight(t, alice, []int{1, 1})
	const dodge = "dnd5e:combat_abilities:dodge"

	fresh := activationFor(t, affordRows(t, mgr, "alice"), dodge)
	require.True(t, fresh.Available)
	require.NotNil(t, fresh.Information)
	require.NotEmpty(t, fresh.Information.Description, "Dodge's combat ability authors its prose")
	require.Empty(t, fresh.Information.Details, "an activation states prose, not facts")

	_, err := mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: fresh.ID,
	})
	require.NoError(t, err)

	spent := activationFor(t, affordRows(t, mgr, "alice"), dodge)
	require.False(t, spent.Available, "precondition: the action is spent")
	require.NotNil(t, spent.Information)
	require.Equal(t, fresh.Information.Description, spent.Information.Description)
}

// TestAffordCommandOptionsDescribed: each word on Command's menu carries the
// spell's own prose for it, in the content's order.
func TestAffordCommandOptionsDescribed(t *testing.T) {
	s := castScene(t, castingBardWithSpells("bard", spells.Command), 2)
	row := s.castRow(spells.Command)

	definition := spells.CastDefinition(spells.CastDefinitionInput{
		Spell: spells.Command, SpellSaveDC: 13, SpellcastingAbility: abilities.CHA,
	})
	require.NotNil(t, definition)
	require.Len(t, row.Options, len(definition.Cast.Options))
	for i, authored := range definition.Cast.Options {
		require.NotEmpty(t, authored.Description, "precondition: the spell describes %s", authored.ID)
		require.Equal(t, authored.ID, row.Options[i].ID)
		require.Equal(t, authored.Description, row.Options[i].Description)
	}
	require.NotNil(t, row.Information)
	require.Equal(t, spellDescription(t, spells.Command), row.Information.Description)
}

// TestAffordSessionVerbsDescribedIncludingBlocked: the verbs session owns say
// what they are on the turn, off it, and on the world clock; a blocked
// Attack, which compiled no definition, carries nothing.
func TestAffordSessionVerbsDescribedIncludingBlocked(t *testing.T) {
	s := &CastSuite{}
	s.SetT(t)
	s.sceneWithAllies(castingBardWithSpells("bard"), []*character.Data{armedFighter("patient")}, 2)
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "bard"})
	require.NoError(t, err)
	active, waiting := "bard", "patient"
	if turn.Active == "patient" {
		active, waiting = "patient", "bard"
	}
	require.Equal(t, active, turn.Active, "precondition: one of the two is acting")

	t.Run("on the turn", func(t *testing.T) {
		rows := affordRows(t, s.mgr, active)
		for _, verb := range []session.Verb{session.VerbMove, session.VerbEndTurn} {
			row := rowOfVerb(t, rows, verb)
			require.NotNil(t, row.Information, "%s", verb)
			require.NotEmpty(t, row.Information.Description, "%s", verb)
			require.Empty(t, row.Information.Details, "%s states no facts", verb)
		}
	})

	t.Run("off the turn", func(t *testing.T) {
		rows := affordRows(t, s.mgr, waiting)
		for _, verb := range []session.Verb{
			session.VerbMove, session.VerbEndTurn, session.VerbIntimidate, session.VerbPersuade,
		} {
			row := rowOfVerb(t, rows, verb)
			require.False(t, row.Available, "precondition: it is not %s's turn", waiting)
			require.NotNil(t, row.Information, "%s", verb)
			require.NotEmpty(t, row.Information.Description, "%s", verb)
		}
		for _, verb := range []session.Verb{session.VerbAttack, session.VerbActivate, session.VerbCast} {
			require.Nil(t, rowOfVerb(t, rows, verb).Information,
				"a blocked %s compiled no definition, so it has no owner's prose to carry", verb)
		}
	})

	t.Run("world clock", func(t *testing.T) {
		p := &PersuadeSuite{}
		p.SetT(t)
		p.SetupTest()
		mgr := p.front([]int{10})
		turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
		require.NoError(t, err)
		require.Equal(t, session.ClockWorld, turn.Clock, "precondition: nothing here is fighting")
		for _, verb := range []session.Verb{session.VerbIntimidate, session.VerbPersuade} {
			row := rowOfVerb(t, p.rows(mgr), verb)
			require.NotNil(t, row.Information, "%s", verb)
			require.NotEmpty(t, row.Information.Description, "%s", verb)
		}
	})
}

// TestAffordAbsentProseStaysAbsent: Patient Defense's feature authors no
// prose (provider-design O2: it spends ki without delivering its benefit,
// toolkit#1986), so its row carries no information at all — its name is right
// there on the row and is never turned into a description.
func TestAffordAbsentProseStaysAbsent(t *testing.T) {
	monk := quarterstaffMonk(t, "alice")
	setLevel(monk, 2)
	raw, err := json.Marshal(features.PatientDefenseData{
		Ref: refs.Features.PatientDefense(), ID: "patient-defense", Name: "Patient Defense", CharacterID: "alice",
	})
	require.NoError(t, err)
	monk.Features = append(monk.Features, raw)
	if monk.Resources == nil {
		monk.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{}
	}
	monk.Resources[resources.Ki] = character.RecoverableResourceData{
		Current: 2, Maximum: 2, ResetType: coreResources.ResetShortRest,
	}
	mgr, _, _, _ := aFight(t, monk, []int{1})

	row := activationFor(t, affordRows(t, mgr, "alice"), refs.Features.PatientDefense().String())
	require.Equal(t, "Patient Defense", row.Ability.Name, "precondition: the name is on the row")
	require.Nil(t, row.Information, "no prose and no facts is no information, never one synthesised from the name")
	encoded, err := json.Marshal(row)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `"information"`, "absent on the wire, not an empty object")
}

// TestReactWindowKeepsOfferAndOptionProseAcrossReload: Wrath of the Storm's
// post-hit window carries the feature's prose and each choice's, and a full
// reload — the next HTTP request — reads the same text under the same ID.
func TestReactWindowKeepsOfferAndOptionProseAcrossReload(t *testing.T) {
	s := &CastSuite{}
	s.SetT(t)
	s.scene(s.tempestSheet(), 1, 15, 2, 1, 4, 5)
	ctx := context.Background()
	monsterTurn := func() {
		var err error
		s.mgr, err = session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: reachlessAttacker{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream})
		require.NoError(t, err)
	}
	monsterTurn()
	_, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "cleric", DeclarationID: currentEndTurnID(t, s.mgr, "sess", "cleric")})
	require.NoError(t, err)

	reactRow := func() session.Declaration {
		return rowOfVerb(t, affordRows(t, s.mgr, "cleric"), session.VerbReact)
	}
	before := reactRow()
	require.NotNil(t, before.Information)
	require.NotEmpty(t, before.Information.Description, "the feature that offers the reaction authors its prose")
	require.Empty(t, before.Information.Details)
	require.Len(t, before.Options, 2)
	for _, option := range before.Options {
		require.NotEmpty(t, option.Description, "the offering feature describes %s", option.ID)
	}

	s.reloadHealingScene()
	after := reactRow()
	require.Equal(t, before.ID, after.ID)
	require.Equal(t, before.Information, after.Information)
	require.Equal(t, before.Options, after.Options)
}

// TestReactOpportunityAttackDescribed: the one reaction session names is the
// one it explains.
func TestReactOpportunityAttackDescribed(t *testing.T) {
	s := &ReactWindowSuite{}
	s.SetT(t)
	s.SetupTest()
	mgr := s.twoSkeletons()
	s.endTurn(mgr, "fighter")
	row := s.reactRow(mgr, "fighter")
	require.NotNil(t, row.Reaction)
	require.Equal(t, refs.Conditions.OpportunityAttack().String(), row.Reaction.Ref)
	require.NotNil(t, row.Information)
	require.NotEmpty(t, row.Information.Description)
}

// TestInformationDoesNotAliasRoot: what one Afford hands out is the caller's
// to keep. Mutating every description, detail and option on two successive
// panels changes nothing a third Afford reads, so no row shares a value with
// the catalogue, the root, or another call.
func TestInformationDoesNotAliasRoot(t *testing.T) {
	s := castScene(t, castingBardWithSpells("bard", spells.Bane, spells.Command), 2)
	want, err := json.Marshal(affordRows(t, s.mgr, "bard"))
	require.NoError(t, err)
	require.Contains(t, string(want), `"information"`, "precondition: the panel is described")

	mutate := func(rows []session.Declaration) {
		for i := range rows {
			if info := rows[i].Information; info != nil {
				info.Description = "mutated"
				for j := range info.Details {
					info.Details[j].Value = "mutated"
				}
			}
			for j := range rows[i].Options {
				rows[i].Options[j].Description = "mutated"
			}
		}
	}
	mutate(affordRows(t, s.mgr, "bard"))
	mutate(affordRows(t, s.mgr, "bard"))

	got, err := json.Marshal(affordRows(t, s.mgr, "bard"))
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got))
}
